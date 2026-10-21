package server

import (
	"crypto/rand"
	"encoding/json"
	"errors"
	"net"
	"net/http"
	"strings"
	"time"

	"github.com/Bagareal/maildex/internal/store"
	"github.com/Bagareal/maildex/internal/webauthn"
)

var errAlreadySetUp = errors.New("already set up")

const challengeTTL = 5 * time.Minute

type challenge struct {
	kind    string // "register" or "login"
	expires time.Time
}

func (s *Server) newChallenge(kind string) string {
	raw := make([]byte, 32)
	_, _ = rand.Read(raw)
	c := webauthn.B64(raw)
	now := time.Now()
	s.mu.Lock()
	defer s.mu.Unlock()
	for k, v := range s.challenges {
		if v.expires.Before(now) {
			delete(s.challenges, k)
		}
	}
	s.challenges[c] = challenge{kind: kind, expires: now.Add(challengeTTL)}
	return c
}

// takeChallenge extracts the challenge from the client data and consumes it,
// so every challenge works exactly once.
func (s *Server) takeChallenge(clientDataJSON []byte, kind string) (string, bool) {
	var cd struct {
		Challenge string `json:"challenge"`
	}
	if json.Unmarshal(clientDataJSON, &cd) != nil {
		return "", false
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	c, found := s.challenges[cd.Challenge]
	delete(s.challenges, cd.Challenge)
	if !found || c.kind != kind || c.expires.Before(time.Now()) {
		return "", false
	}
	return cd.Challenge, true
}

func expectFor(r *http.Request, challenge string) webauthn.Expect {
	scheme := "http"
	if isHTTPS(r) {
		scheme = "https"
	}
	host := r.Host
	if fh := r.Header.Get("X-Forwarded-Host"); fh != "" {
		host = strings.TrimSpace(strings.Split(fh, ",")[0])
	}
	rpID := host
	if h, _, err := net.SplitHostPort(host); err == nil {
		rpID = h
	}
	return webauthn.Expect{Challenge: challenge, Origin: scheme + "://" + host, RPID: strings.Trim(rpID, "[]")}
}

type passkeyInfo struct {
	ID      string    `json:"id"`
	Name    string    `json:"name"`
	Created time.Time `json:"created"`
}

func (s *Server) handlePasskeyList(w http.ResponseWriter, r *http.Request) {
	out := []passkeyInfo{}
	s.st.View(func(_ *store.Data, a *store.Auth) {
		for _, c := range a.Credentials {
			out = append(out, passkeyInfo{c.ID, c.Name, c.Created})
		}
	})
	writeJSON(w, http.StatusOK, out)
}

func (s *Server) handlePasskeyRegisterBegin(w http.ResponseWriter, r *http.Request) {
	exclude := []map[string]any{}
	s.st.View(func(_ *store.Data, a *store.Auth) {
		for _, c := range a.Credentials {
			exclude = append(exclude, map[string]any{"type": "public-key", "id": c.ID})
		}
	})
	exp := expectFor(r, "")
	writeJSON(w, http.StatusOK, map[string]any{"publicKey": map[string]any{
		"challenge": s.newChallenge("register"),
		"rp":        map[string]string{"name": "Maildex", "id": exp.RPID},
		"user":      map[string]string{"id": webauthn.B64([]byte("maildex-owner")), "name": "maildex", "displayName": "Maildex"},
		"pubKeyCredParams": []map[string]any{
			{"type": "public-key", "alg": webauthn.AlgES256},
			{"type": "public-key", "alg": webauthn.AlgRS256},
		},
		"authenticatorSelection": map[string]string{"authenticatorAttachment": "platform", "residentKey": "preferred", "userVerification": "required"},
		"attestation":            "none",
		"excludeCredentials":     exclude,
		"timeout":                60000,
	}})
}

func (s *Server) handlePasskeyRegisterFinish(w http.ResponseWriter, r *http.Request) {
	var b struct {
		Name     string `json:"name"`
		Response struct {
			ClientDataJSON    string `json:"clientDataJSON"`
			AttestationObject string `json:"attestationObject"`
		} `json:"response"`
	}
	if !readBody(w, r, maxBody, &b) {
		return
	}
	cd, err1 := webauthn.FromB64(b.Response.ClientDataJSON)
	att, err2 := webauthn.FromB64(b.Response.AttestationObject)
	if err1 != nil || err2 != nil {
		fail(w, http.StatusBadRequest, "invalid response")
		return
	}
	ch, found := s.takeChallenge(cd, "register")
	if !found {
		fail(w, http.StatusBadRequest, "challenge expired, try again")
		return
	}
	reg, err := webauthn.VerifyRegistration(att, cd, expectFor(r, ch))
	if err != nil {
		fail(w, http.StatusBadRequest, "biometric registration failed: "+err.Error())
		return
	}
	name := strings.TrimSpace(b.Name)
	if name == "" {
		name = "This device"
	}
	if len(name) > 60 {
		name = name[:60]
	}
	id := webauthn.B64(reg.CredentialID)
	err = s.st.Update(func(_ *store.Data, a *store.Auth) error {
		for _, c := range a.Credentials {
			if c.ID == id {
				return errors.New("already registered")
			}
		}
		a.Credentials = append(a.Credentials, store.Credential{ID: id, Name: name, PublicKey: reg.PublicKey, SignCount: reg.SignCount, Created: time.Now()})
		return nil
	})
	if err != nil {
		fail(w, http.StatusConflict, err.Error())
		return
	}
	ok(w)
}

func (s *Server) handlePasskeyDelete(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	err := s.st.Update(func(_ *store.Data, a *store.Auth) error {
		kept := a.Credentials[:0]
		for _, c := range a.Credentials {
			if c.ID != id {
				kept = append(kept, c)
			}
		}
		a.Credentials = kept
		return nil
	})
	if err != nil {
		fail(w, http.StatusInternalServerError, "could not save")
		return
	}
	ok(w)
}

func (s *Server) handlePasskeyLoginBegin(w http.ResponseWriter, r *http.Request) {
	allow := []map[string]any{}
	s.st.View(func(_ *store.Data, a *store.Auth) {
		for _, c := range a.Credentials {
			allow = append(allow, map[string]any{"type": "public-key", "id": c.ID})
		}
	})
	if len(allow) == 0 {
		fail(w, http.StatusNotFound, "no biometrics registered")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"publicKey": map[string]any{
		"challenge":        s.newChallenge("login"),
		"rpId":             expectFor(r, "").RPID,
		"allowCredentials": allow,
		"userVerification": "required",
		"timeout":          60000,
	}})
}

// handlePasskeyLoginFinish signs in (or unlocks the current session) with a
// verified passkey assertion.
func (s *Server) handlePasskeyLoginFinish(w http.ResponseWriter, r *http.Request) {
	ip := clientIP(r)
	if s.throttled(ip) {
		fail(w, http.StatusTooManyRequests, "too many attempts, try again in a minute")
		return
	}
	var b struct {
		ID       string `json:"id"`
		Response struct {
			ClientDataJSON    string `json:"clientDataJSON"`
			AuthenticatorData string `json:"authenticatorData"`
			Signature         string `json:"signature"`
		} `json:"response"`
	}
	if !readBody(w, r, maxBody, &b) {
		return
	}
	cd, err1 := webauthn.FromB64(b.Response.ClientDataJSON)
	ad, err2 := webauthn.FromB64(b.Response.AuthenticatorData)
	sig, err3 := webauthn.FromB64(b.Response.Signature)
	if err1 != nil || err2 != nil || err3 != nil {
		fail(w, http.StatusBadRequest, "invalid response")
		return
	}
	ch, found := s.takeChallenge(cd, "login")
	if !found {
		fail(w, http.StatusBadRequest, "challenge expired, try again")
		return
	}
	var cred *store.Credential
	s.st.View(func(_ *store.Data, a *store.Auth) {
		for _, c := range a.Credentials {
			if c.ID == b.ID {
				cp := c
				cred = &cp
			}
		}
	})
	if cred == nil {
		s.noteFailure(ip)
		fail(w, http.StatusUnauthorized, "unknown biometric credential")
		return
	}
	count, err := webauthn.VerifyAssertion(cred.PublicKey, ad, cd, sig, expectFor(r, ch))
	if err != nil || ((count != 0 || cred.SignCount != 0) && count <= cred.SignCount) {
		s.noteFailure(ip)
		fail(w, http.StatusUnauthorized, "biometric check failed")
		return
	}
	s.clearFailures(ip)
	_ = s.st.Update(func(_ *store.Data, a *store.Auth) error {
		for i := range a.Credentials {
			if a.Credentials[i].ID == cred.ID {
				a.Credentials[i].SignCount = count
			}
		}
		return nil
	})
	if id, valid := s.sessionID(r); valid {
		s.setLock(id, false)
	} else if err := s.startSession(w, r); err != nil {
		fail(w, http.StatusInternalServerError, "could not start session")
		return
	}
	ok(w)
}
