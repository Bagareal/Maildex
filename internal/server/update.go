package server

import (
	"encoding/json"
	"net/http"
	"strconv"
	"strings"
	"time"
)

const releasesAPI = "https://api.github.com/repos/Bagareal/Maildex/releases/latest"

type updateCache struct {
	latest  string
	checked time.Time
}

func (s *Server) latestRelease() string {
	s.mu.Lock()
	if time.Since(s.upd.checked) < 6*time.Hour {
		v := s.upd.latest
		s.mu.Unlock()
		return v
	}
	s.mu.Unlock()

	latest := ""
	client := &http.Client{Timeout: 5 * time.Second}
	if resp, err := client.Get(releasesAPI); err == nil {
		defer resp.Body.Close()
		var rel struct {
			Tag string `json:"tag_name"`
		}
		if resp.StatusCode == http.StatusOK && json.NewDecoder(resp.Body).Decode(&rel) == nil {
			latest = rel.Tag
		}
	}
	s.mu.Lock()
	s.upd = updateCache{latest: latest, checked: time.Now()}
	s.mu.Unlock()
	return latest
}

// newer reports whether version a is newer than b (both like v1.2.3).
func newer(a, b string) bool {
	pa, pb := parseVersion(a), parseVersion(b)
	for i := range pa {
		if pa[i] != pb[i] {
			return pa[i] > pb[i]
		}
	}
	return false
}

func parseVersion(v string) [3]int {
	var out [3]int
	for i, p := range strings.SplitN(strings.TrimPrefix(v, "v"), ".", 3) {
		if i > 2 {
			break
		}
		n, _ := strconv.Atoi(strings.SplitN(p, "-", 2)[0])
		out[i] = n
	}
	return out
}

func (s *Server) handleVersion(w http.ResponseWriter, r *http.Request) {
	latest := ""
	if s.version != "dev" {
		latest = s.latestRelease()
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"version": s.version, "latest": latest, "updateAvailable": latest != "" && newer(latest, s.version),
	})
}
