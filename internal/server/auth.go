package server

import (
	"crypto/pbkdf2"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/Bagareal/maildex/internal/store"
)

const (
	cookieName    = "maildex_session"
	sessionTTL    = 90 * 24 * time.Hour
	pbkdf2Iter    = 600_000
	minPasswordLn = 8
)

type liveSession struct {
	lastActive time.Time
	locked     bool
}

func hashPassword(pw string) (string, error) {
	salt := make([]byte, 16)
	if _, err := rand.Read(salt); err != nil {
		return "", err
	}
	key, err := pbkdf2.Key(sha256.New, pw, salt, pbkdf2Iter, 32)
	if err != nil {
		return "", err
	}
	enc := base64.RawStdEncoding
	return fmt.Sprintf("pbkdf2-sha256$%d$%s$%s", pbkdf2Iter, enc.EncodeToString(salt), enc.EncodeToString(key)), nil
}

func checkPassword(encoded, pw string) bool {
	parts := strings.Split(encoded, "$")
	if len(parts) != 4 || parts[0] != "pbkdf2-sha256" {
		return false
	}
	iter, err := strconv.Atoi(parts[1])
	if err != nil || iter < 1 {
		return false
	}
	enc := base64.RawStdEncoding
	salt, err1 := enc.DecodeString(parts[2])
	want, err2 := enc.DecodeString(parts[3])
	if err1 != nil || err2 != nil {
		return false
	}
	got, err := pbkdf2.Key(sha256.New, pw, salt, iter, len(want))
	return err == nil && subtle.ConstantTimeCompare(got, want) == 1
}

func tokenID(token string) string {
	h := sha256.Sum256([]byte(token))
	return hex.EncodeToString(h[:])
}

// startSession creates a persistent session and sets the cookie.
func (s *Server) startSession(w http.ResponseWriter, r *http.Request) error {
	raw := make([]byte, 32)
	if _, err := rand.Read(raw); err != nil {
		return err
	}
	token := base64.RawURLEncoding.EncodeToString(raw)
	id := tokenID(token)
	now := time.Now()
	err := s.st.Update(func(_ *store.Data, a *store.Auth) error {
		kept := a.Sessions[:0]
		for _, rec := range a.Sessions {
			if rec.Expires.After(now) {
				kept = append(kept, rec)
			}
		}
		a.Sessions = append(kept, store.SessionRec{ID: id, Created: now, Expires: now.Add(sessionTTL)})
		return nil
	})
	if err != nil {
		return err
	}
	s.mu.Lock()
	s.live[id] = &liveSession{lastActive: now}
	s.mu.Unlock()
	http.SetCookie(w, &http.Cookie{
		Name: cookieName, Value: token, Path: "/", MaxAge: int(sessionTTL.Seconds()),
		HttpOnly: true, Secure: isHTTPS(r), SameSite: http.SameSiteLaxMode,
	})
	return nil
}

// sessionID returns the ID of the valid session attached to the request.
func (s *Server) sessionID(r *http.Request) (string, bool) {
	c, err := r.Cookie(cookieName)
	if err != nil || c.Value == "" {
		return "", false
	}
	id := tokenID(c.Value)
	found := false
	s.st.View(func(_ *store.Data, a *store.Auth) {
		now := time.Now()
		for _, rec := range a.Sessions {
			if rec.ID == id && rec.Expires.After(now) {
				found = true
				return
			}
		}
	})
	return id, found
}

// isLocked applies the idle timeout and reports whether the session needs
// unlocking. The idle lock only applies once a passkey is enrolled.
func (s *Server) isLocked(id string) bool {
	var enabled bool
	var minutes int
	s.st.View(func(d *store.Data, a *store.Auth) {
		enabled = len(a.Credentials) > 0
		minutes = d.Settings.LockMinutes
	})
	s.mu.Lock()
	defer s.mu.Unlock()
	ls := s.live[id]
	if ls == nil {
		ls = &liveSession{lastActive: time.Now()}
		s.live[id] = ls
	}
	if !ls.locked && enabled && minutes > 0 && time.Since(ls.lastActive) > time.Duration(minutes)*time.Minute {
		ls.locked = true
	}
	return ls.locked
}

func (s *Server) setLock(id string, locked bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	ls := s.live[id]
	if ls == nil {
		ls = &liveSession{}
		s.live[id] = ls
	}
	ls.locked = locked
	if !locked {
		ls.lastActive = time.Now()
	}
}

// auth requires a valid session. Locked sessions are rejected with 423 unless
// allowLocked is set.
func (s *Server) auth(h http.HandlerFunc, allowLocked bool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, valid := s.sessionID(r)
		if !valid {
			fail(w, http.StatusUnauthorized, "not signed in")
			return
		}
		if s.isLocked(id) {
			if !allowLocked {
				fail(w, http.StatusLocked, "locked")
				return
			}
		} else {
			s.mu.Lock()
			if ls := s.live[id]; ls != nil {
				ls.lastActive = time.Now()
			}
			s.mu.Unlock()
		}
		h(w, r)
	}
}

func (s *Server) hasPassword() (set bool) {
	s.st.View(func(_ *store.Data, a *store.Auth) { set = a.PasswordHash != "" })
	return
}

func (s *Server) handleSession(w http.ResponseWriter, r *http.Request) {
	out := map[string]any{"version": s.version, "setup": !s.hasPassword(), "authenticated": false, "locked": false, "biometric": false, "secure": isHTTPS(r)}
	s.st.View(func(_ *store.Data, a *store.Auth) { out["biometric"] = len(a.Credentials) > 0 })
	if id, valid := s.sessionID(r); valid {
		out["authenticated"] = true
		out["locked"] = s.isLocked(id)
	}
	writeJSON(w, http.StatusOK, out)
}

type passwordBody struct {
	Password string `json:"password"`
}

func (s *Server) handleSetup(w http.ResponseWriter, r *http.Request) {
	var b passwordBody
	if !readBody(w, r, maxBody, &b) {
		return
	}
	if len(b.Password) < minPasswordLn {
		fail(w, http.StatusBadRequest, "password must be at least 8 characters")
		return
	}
	hash, err := hashPassword(b.Password)
	if err != nil {
		fail(w, http.StatusInternalServerError, "could not hash password")
		return
	}
	conflict := false
	if err := s.st.Update(func(_ *store.Data, a *store.Auth) error {
		if a.PasswordHash != "" {
			conflict = true
			return errAlreadySetUp
		}
		a.PasswordHash = hash
		return nil
	}); err != nil {
		if conflict {
			fail(w, http.StatusConflict, "already set up")
		} else {
			fail(w, http.StatusInternalServerError, "could not save")
		}
		return
	}
	if err := s.startSession(w, r); err != nil {
		fail(w, http.StatusInternalServerError, "could not start session")
		return
	}
	ok(w)
}

func (s *Server) passwordMatches(pw string) bool {
	var hash string
	s.st.View(func(_ *store.Data, a *store.Auth) { hash = a.PasswordHash })
	return hash != "" && checkPassword(hash, pw)
}

func (s *Server) handleLogin(w http.ResponseWriter, r *http.Request) {
	ip := clientIP(r)
	if s.throttled(ip) {
		fail(w, http.StatusTooManyRequests, "too many attempts, try again in a minute")
		return
	}
	var b passwordBody
	if !readBody(w, r, maxBody, &b) {
		return
	}
	if !s.passwordMatches(b.Password) {
		s.noteFailure(ip)
		fail(w, http.StatusUnauthorized, "wrong password")
		return
	}
	s.clearFailures(ip)
	if err := s.startSession(w, r); err != nil {
		fail(w, http.StatusInternalServerError, "could not start session")
		return
	}
	ok(w)
}

func (s *Server) handleUnlock(w http.ResponseWriter, r *http.Request) {
	ip := clientIP(r)
	if s.throttled(ip) {
		fail(w, http.StatusTooManyRequests, "too many attempts, try again in a minute")
		return
	}
	var b passwordBody
	if !readBody(w, r, maxBody, &b) {
		return
	}
	if !s.passwordMatches(b.Password) {
		s.noteFailure(ip)
		fail(w, http.StatusUnauthorized, "wrong password")
		return
	}
	s.clearFailures(ip)
	id, _ := s.sessionID(r)
	s.setLock(id, false)
	ok(w)
}

func (s *Server) handleLock(w http.ResponseWriter, r *http.Request) {
	id, _ := s.sessionID(r)
	s.setLock(id, true)
	ok(w)
}

func (s *Server) handleLogout(w http.ResponseWriter, r *http.Request) {
	id, _ := s.sessionID(r)
	_ = s.st.Update(func(_ *store.Data, a *store.Auth) error {
		kept := a.Sessions[:0]
		for _, rec := range a.Sessions {
			if rec.ID != id {
				kept = append(kept, rec)
			}
		}
		a.Sessions = kept
		return nil
	})
	s.mu.Lock()
	delete(s.live, id)
	s.mu.Unlock()
	http.SetCookie(w, &http.Cookie{Name: cookieName, Value: "", Path: "/", MaxAge: -1, HttpOnly: true, Secure: isHTTPS(r), SameSite: http.SameSiteLaxMode})
	ok(w)
}

func (s *Server) handlePassword(w http.ResponseWriter, r *http.Request) {
	var b struct {
		Current string `json:"current"`
		New     string `json:"new"`
	}
	if !readBody(w, r, maxBody, &b) {
		return
	}
	if !s.passwordMatches(b.Current) {
		fail(w, http.StatusUnauthorized, "current password is wrong")
		return
	}
	if len(b.New) < minPasswordLn {
		fail(w, http.StatusBadRequest, "password must be at least 8 characters")
		return
	}
	hash, err := hashPassword(b.New)
	if err != nil {
		fail(w, http.StatusInternalServerError, "could not hash password")
		return
	}
	keep, _ := s.sessionID(r)
	err = s.st.Update(func(_ *store.Data, a *store.Auth) error {
		a.PasswordHash = hash
		kept := a.Sessions[:0] // sign out every other device
		for _, rec := range a.Sessions {
			if rec.ID == keep {
				kept = append(kept, rec)
			}
		}
		a.Sessions = kept
		return nil
	})
	if err != nil {
		fail(w, http.StatusInternalServerError, "could not save")
		return
	}
	ok(w)
}
