package server

import (
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/Bagareal/maildex/internal/store"
)

var (
	statuses = map[string]bool{"active": true, "to-close": true, "closed": true}
	methods  = map[string]bool{"password": true, "google": true, "apple": true, "microsoft": true, "github": true, "facebook": true, "passkey": true, "sso": true, "other": true}
	// Second factors, in display order.
	twoFactorKinds = []string{"totp", "passkey", "security-key", "sms", "email", "push", "backup"}
)

// cleanTwoFactor keeps only known methods, once each, in display order.
func cleanTwoFactor(in []string) []string {
	have := map[string]bool{}
	for _, m := range in {
		have[strings.ToLower(strings.TrimSpace(m))] = true
	}
	out := []string{}
	for _, k := range twoFactorKinds {
		if have[k] {
			out = append(out, k)
		}
	}
	return out
}

func clip(s string, n int) string {
	s = strings.TrimSpace(s)
	if r := []rune(s); len(r) > n {
		return string(r[:n])
	}
	return s
}

type emailInput struct {
	Address  string `json:"address"`
	Label    string `json:"label"`
	Provider string `json:"provider"`
	Recovery string `json:"recovery"`
	Notes    string `json:"notes"`
	AtRisk   bool   `json:"atRisk"`
}

func (in emailInput) normalize() (store.Email, error) {
	addr := strings.ToLower(clip(in.Address, 254))
	at := strings.Index(addr, "@")
	if at < 1 || at == len(addr)-1 || strings.Count(addr, "@") != 1 || strings.ContainsAny(addr, " \t") {
		return store.Email{}, errors.New("enter a valid email address")
	}
	return store.Email{
		Address: addr, Label: clip(in.Label, 80), Provider: clip(in.Provider, 80),
		Recovery: clip(in.Recovery, 300), Notes: clip(in.Notes, 2000), AtRisk: in.AtRisk,
	}, nil
}

// serviceCore holds the fields shared by the API and the import/export file.
type serviceCore struct {
	Name        string   `json:"name"`
	Username    string   `json:"username"`
	LoginMethod string   `json:"loginMethod"`
	Status      string   `json:"status"`
	TwoFactor   []string `json:"twoFactor"`
	URL         string   `json:"url"`
	Renewal     string   `json:"renewal"`
	Notes       string   `json:"notes"`
}

// serviceInput is what the web UI sends: the email and folders by ID.
type serviceInput struct {
	serviceCore
	EmailID string   `json:"emailId"`
	Folders []string `json:"folders"`
}

func (in serviceCore) normalize() (store.Service, error) {
	name := clip(in.Name, 120)
	if name == "" {
		return store.Service{}, errors.New("service name is required")
	}
	status := strings.ToLower(clip(in.Status, 20))
	if status == "" {
		status = "active"
	}
	if !statuses[status] {
		return store.Service{}, errors.New("unknown status")
	}
	method := strings.ToLower(clip(in.LoginMethod, 20))
	if method == "" {
		method = "password"
	} else if !methods[method] {
		method = "other"
	}
	renewal := clip(in.Renewal, 10)
	if renewal != "" {
		if _, err := time.Parse("2006-01-02", renewal); err != nil {
			return store.Service{}, errors.New("renewal must be a date (YYYY-MM-DD)")
		}
	}
	url := clip(in.URL, 500)
	if url != "" && !strings.HasPrefix(url, "http://") && !strings.HasPrefix(url, "https://") {
		url = "https://" + url
	}
	return store.Service{
		Name: name, Username: clip(in.Username, 120), LoginMethod: method,
		Status: status, TwoFactor: cleanTwoFactor(in.TwoFactor), URL: url, Renewal: renewal, Notes: clip(in.Notes, 2000),
	}, nil
}

func findEmail(d *store.Data, id string) int {
	for i := range d.Emails {
		if d.Emails[i].ID == id {
			return i
		}
	}
	return -1
}

func findEmailByAddress(d *store.Data, addr string) int {
	for i := range d.Emails {
		if d.Emails[i].Address == addr {
			return i
		}
	}
	return -1
}

func findService(d *store.Data, id string) int {
	for i := range d.Services {
		if d.Services[i].ID == id {
			return i
		}
	}
	return -1
}

var (
	errNotFound  = errors.New("not found")
	errDuplicate = errors.New("that email address already exists")
	errNoEmail   = errors.New("choose an existing email for this service")
)

// respond maps store-callback errors to HTTP statuses.
func respond(w http.ResponseWriter, err error, okBody any) {
	switch {
	case err == nil:
		writeJSON(w, http.StatusOK, okBody)
	case errors.Is(err, errNotFound):
		fail(w, http.StatusNotFound, err.Error())
	case errors.Is(err, errDuplicate), errors.Is(err, errFolderDup):
		fail(w, http.StatusConflict, err.Error())
	case errors.Is(err, errNoEmail):
		fail(w, http.StatusBadRequest, err.Error())
	default:
		fail(w, http.StatusInternalServerError, "could not save")
	}
}

func (s *Server) handleData(w http.ResponseWriter, r *http.Request) {
	var out store.Data
	s.st.View(func(d *store.Data, _ *store.Auth) {
		out = store.Data{
			Emails:   append([]store.Email{}, d.Emails...),
			Services: append([]store.Service{}, d.Services...),
			Folders:  append([]store.Folder{}, d.Folders...),
			Settings: d.Settings,
		}
	})
	writeJSON(w, http.StatusOK, out)
}

func (s *Server) handleEmailCreate(w http.ResponseWriter, r *http.Request) {
	var in emailInput
	if !readBody(w, r, maxBody, &in) {
		return
	}
	e, err := in.normalize()
	if err != nil {
		fail(w, http.StatusBadRequest, err.Error())
		return
	}
	e.ID, e.Created = store.NewID(), time.Now()
	err = s.st.Update(func(d *store.Data, _ *store.Auth) error {
		if findEmailByAddress(d, e.Address) >= 0 {
			return errDuplicate
		}
		d.Emails = append(d.Emails, e)
		return nil
	})
	respond(w, err, e)
}

func (s *Server) handleEmailUpdate(w http.ResponseWriter, r *http.Request) {
	var in emailInput
	if !readBody(w, r, maxBody, &in) {
		return
	}
	e, err := in.normalize()
	if err != nil {
		fail(w, http.StatusBadRequest, err.Error())
		return
	}
	id := r.PathValue("id")
	err = s.st.Update(func(d *store.Data, _ *store.Auth) error {
		i := findEmail(d, id)
		if i < 0 {
			return errNotFound
		}
		if j := findEmailByAddress(d, e.Address); j >= 0 && j != i {
			return errDuplicate
		}
		e.ID, e.Created = id, d.Emails[i].Created
		d.Emails[i] = e
		return nil
	})
	respond(w, err, e)
}

func (s *Server) handleEmailDelete(w http.ResponseWriter, r *http.Request) {
	id, force := r.PathValue("id"), r.URL.Query().Get("force") == "1"
	linked := 0
	err := s.st.Update(func(d *store.Data, _ *store.Auth) error {
		i := findEmail(d, id)
		if i < 0 {
			return errNotFound
		}
		kept := d.Services[:0:0]
		for _, sv := range d.Services {
			if sv.EmailID == id {
				linked++
			} else {
				kept = append(kept, sv)
			}
		}
		if linked > 0 && !force {
			return errHasServices
		}
		d.Services = kept
		d.Emails = append(d.Emails[:i], d.Emails[i+1:]...)
		return nil
	})
	if errors.Is(err, errHasServices) {
		writeJSON(w, http.StatusConflict, map[string]any{"error": "this email still has services", "services": linked})
		return
	}
	respond(w, err, map[string]bool{"ok": true})
}

var errHasServices = errors.New("has services")

func (s *Server) handleServiceCreate(w http.ResponseWriter, r *http.Request) {
	var in serviceInput
	if !readBody(w, r, maxBody, &in) {
		return
	}
	sv, err := in.serviceCore.normalize()
	if err != nil {
		fail(w, http.StatusBadRequest, err.Error())
		return
	}
	sv.ID, sv.Created, sv.EmailID = store.NewID(), time.Now(), in.EmailID
	err = s.st.Update(func(d *store.Data, _ *store.Auth) error {
		if findEmail(d, sv.EmailID) < 0 {
			return errNoEmail
		}
		sv.Folders = keepFolders(d, in.Folders)
		d.Services = append(d.Services, sv)
		return nil
	})
	respond(w, err, sv)
}

func (s *Server) handleServiceUpdate(w http.ResponseWriter, r *http.Request) {
	var in serviceInput
	if !readBody(w, r, maxBody, &in) {
		return
	}
	sv, err := in.serviceCore.normalize()
	if err != nil {
		fail(w, http.StatusBadRequest, err.Error())
		return
	}
	sv.EmailID = in.EmailID
	id := r.PathValue("id")
	err = s.st.Update(func(d *store.Data, _ *store.Auth) error {
		i := findService(d, id)
		if i < 0 {
			return errNotFound
		}
		if findEmail(d, sv.EmailID) < 0 {
			return errNoEmail
		}
		sv.Folders = keepFolders(d, in.Folders)
		sv.ID, sv.Created = id, d.Services[i].Created
		d.Services[i] = sv
		return nil
	})
	respond(w, err, sv)
}

func (s *Server) handleServiceDelete(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	err := s.st.Update(func(d *store.Data, _ *store.Auth) error {
		i := findService(d, id)
		if i < 0 {
			return errNotFound
		}
		d.Services = append(d.Services[:i], d.Services[i+1:]...)
		return nil
	})
	respond(w, err, map[string]bool{"ok": true})
}

func (s *Server) handleSettings(w http.ResponseWriter, r *http.Request) {
	var in store.Settings
	if !readBody(w, r, maxBody, &in) {
		return
	}
	if in.LockMinutes < 0 || in.LockMinutes > 1440 {
		fail(w, http.StatusBadRequest, "lock timeout must be between 0 and 1440 minutes")
		return
	}
	err := s.st.Update(func(d *store.Data, _ *store.Auth) error {
		d.Settings = in
		return nil
	})
	respond(w, err, in)
}
