// Package server exposes the Maildex JSON API and the embedded web UI.
package server

import (
	"encoding/json"
	"io"
	"io/fs"
	"mime"
	"net"
	"net/http"
	"path"
	"strings"
	"sync"
	"time"

	"github.com/Bagareal/maildex/internal/store"
)

type Server struct {
	st      *store.Store
	version string
	static  fs.FS

	mu         sync.Mutex
	live       map[string]*liveSession
	challenges map[string]challenge
	fails      map[string]*failRec
	upd        updateCache
}

func New(st *store.Store, version string, static fs.FS) *Server {
	_ = mime.AddExtensionType(".webmanifest", "application/manifest+json")
	return &Server{
		st:         st,
		version:    version,
		static:     static,
		live:       map[string]*liveSession{},
		challenges: map[string]challenge{},
		fails:      map[string]*failRec{},
	}
}

func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()

	// Public: bootstrap, login and passkey login.
	mux.HandleFunc("GET /api/session", s.api(s.handleSession))
	mux.HandleFunc("POST /api/setup", s.api(s.handleSetup))
	mux.HandleFunc("POST /api/login", s.api(s.handleLogin))
	mux.HandleFunc("POST /api/passkey/login/begin", s.api(s.handlePasskeyLoginBegin))
	mux.HandleFunc("POST /api/passkey/login/finish", s.api(s.handlePasskeyLoginFinish))

	// Session required, but usable while locked.
	mux.HandleFunc("POST /api/logout", s.api(s.auth(s.handleLogout, true)))
	mux.HandleFunc("POST /api/unlock", s.api(s.auth(s.handleUnlock, true)))

	// Session required and unlocked.
	mux.HandleFunc("POST /api/lock", s.api(s.auth(s.handleLock, false)))
	mux.HandleFunc("POST /api/ping", s.api(s.auth(func(w http.ResponseWriter, r *http.Request) { ok(w) }, false)))
	mux.HandleFunc("POST /api/password", s.api(s.auth(s.handlePassword, false)))
	mux.HandleFunc("GET /api/passkeys", s.api(s.auth(s.handlePasskeyList, false)))
	mux.HandleFunc("POST /api/passkeys/begin", s.api(s.auth(s.handlePasskeyRegisterBegin, false)))
	mux.HandleFunc("POST /api/passkeys/finish", s.api(s.auth(s.handlePasskeyRegisterFinish, false)))
	mux.HandleFunc("DELETE /api/passkeys/{id}", s.api(s.auth(s.handlePasskeyDelete, false)))

	mux.HandleFunc("GET /api/data", s.api(s.auth(s.handleData, false)))
	mux.HandleFunc("POST /api/emails", s.api(s.auth(s.handleEmailCreate, false)))
	mux.HandleFunc("PUT /api/emails/{id}", s.api(s.auth(s.handleEmailUpdate, false)))
	mux.HandleFunc("DELETE /api/emails/{id}", s.api(s.auth(s.handleEmailDelete, false)))
	mux.HandleFunc("POST /api/folders", s.api(s.auth(s.handleFolderCreate, false)))
	mux.HandleFunc("PUT /api/folders/{id}", s.api(s.auth(s.handleFolderUpdate, false)))
	mux.HandleFunc("DELETE /api/folders/{id}", s.api(s.auth(s.handleFolderDelete, false)))
	mux.HandleFunc("POST /api/services/bulk", s.api(s.auth(s.handleServiceBulk, false)))
	mux.HandleFunc("POST /api/services", s.api(s.auth(s.handleServiceCreate, false)))
	mux.HandleFunc("PUT /api/services/{id}", s.api(s.auth(s.handleServiceUpdate, false)))
	mux.HandleFunc("DELETE /api/services/{id}", s.api(s.auth(s.handleServiceDelete, false)))
	mux.HandleFunc("PUT /api/settings", s.api(s.auth(s.handleSettings, false)))
	mux.HandleFunc("GET /api/export", s.api(s.auth(s.handleExport, false)))
	mux.HandleFunc("POST /api/import", s.api(s.auth(s.handleImport, false)))
	mux.HandleFunc("GET /api/version", s.api(s.auth(s.handleVersion, false)))

	mux.Handle("/", s.staticHandler())
	return securityHeaders(mux)
}

func securityHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		h := w.Header()
		h.Set("Content-Security-Policy", "default-src 'self'; img-src 'self' data:; style-src 'self' 'unsafe-inline'; object-src 'none'; base-uri 'none'; frame-ancestors 'none'")
		h.Set("X-Content-Type-Options", "nosniff")
		h.Set("X-Frame-Options", "DENY")
		h.Set("Referrer-Policy", "no-referrer")
		next.ServeHTTP(w, r)
	})
}

func (s *Server) staticHandler() http.Handler {
	files := http.FileServerFS(s.static)
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		p := strings.TrimPrefix(path.Clean(r.URL.Path), "/")
		if p != "" {
			if _, err := fs.Stat(s.static, p); err != nil {
				r.URL.Path = "/" // single-page app fallback
			}
		}
		w.Header().Set("Cache-Control", "no-cache")
		files.ServeHTTP(w, r)
	})
}

// api wraps API handlers: no caching, and a custom header on anything that
// changes state so that cross-site form posts can never trigger it.
func (s *Server) api(h http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "no-store")
		if r.Method != http.MethodGet && r.Method != http.MethodHead && r.Header.Get("X-Maildex") == "" {
			fail(w, http.StatusForbidden, "missing X-Maildex header")
			return
		}
		h(w, r)
	}
}

func writeJSON(w http.ResponseWriter, code int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(v)
}

func fail(w http.ResponseWriter, code int, msg string) {
	writeJSON(w, code, map[string]string{"error": msg})
}

func ok(w http.ResponseWriter) { writeJSON(w, http.StatusOK, map[string]bool{"ok": true}) }

const (
	maxBody       = 1 << 20
	maxImportBody = 8 << 20
)

func readBody(w http.ResponseWriter, r *http.Request, limit int64, v any) bool {
	dec := json.NewDecoder(io.LimitReader(r.Body, limit))
	if err := dec.Decode(v); err != nil {
		fail(w, http.StatusBadRequest, "invalid request body")
		return false
	}
	return true
}

func clientIP(r *http.Request) string {
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}

func isHTTPS(r *http.Request) bool {
	return r.TLS != nil || strings.EqualFold(r.Header.Get("X-Forwarded-Proto"), "https")
}

// failRec throttles repeated wrong passwords from one address.
type failRec struct {
	count int
	last  time.Time
}

const (
	maxFails   = 5
	failWindow = time.Minute
)

func (s *Server) throttled(ip string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	f := s.fails[ip]
	return f != nil && f.count >= maxFails && time.Since(f.last) < failWindow
}

func (s *Server) noteFailure(ip string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	f := s.fails[ip]
	if f == nil || time.Since(f.last) > failWindow {
		f = &failRec{}
		s.fails[ip] = f
	}
	f.count++
	f.last = time.Now()
}

func (s *Server) clearFailures(ip string) {
	s.mu.Lock()
	delete(s.fails, ip)
	s.mu.Unlock()
}
