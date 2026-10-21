package server

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/cookiejar"
	"net/http/httptest"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/Bagareal/maildex/internal/store"
)

type client struct {
	t   *testing.T
	ts  *httptest.Server
	hc  *http.Client
	srv *Server
}

func newClient(t *testing.T) *client {
	t.Helper()
	st, err := store.Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	srv := New(st, "v1.0.0", fstest.MapFS{"index.html": {Data: []byte("<html>ok</html>")}})
	ts := httptest.NewServer(srv.Handler())
	t.Cleanup(ts.Close)
	jar, _ := cookiejar.New(nil)
	return &client{t: t, ts: ts, hc: &http.Client{Jar: jar}, srv: srv}
}

func (c *client) do(method, path string, body any, out any) int {
	c.t.Helper()
	var rd *bytes.Reader
	if body != nil {
		b, _ := json.Marshal(body)
		rd = bytes.NewReader(b)
	} else {
		rd = bytes.NewReader(nil)
	}
	req, _ := http.NewRequest(method, c.ts.URL+path, rd)
	req.Header.Set("X-Maildex", "1")
	resp, err := c.hc.Do(req)
	if err != nil {
		c.t.Fatal(err)
	}
	defer resp.Body.Close()
	if out != nil {
		_ = json.NewDecoder(resp.Body).Decode(out)
	}
	return resp.StatusCode
}

func want(t *testing.T, got, expected int, what string) {
	t.Helper()
	if got != expected {
		t.Fatalf("%s: got HTTP %d, want %d", what, got, expected)
	}
}

func TestFullFlow(t *testing.T) {
	c := newClient(t)

	want(t, c.do("GET", "/api/data", nil, nil), 401, "data before login")
	want(t, c.do("POST", "/api/setup", map[string]string{"password": "short"}, nil), 400, "weak password")
	want(t, c.do("POST", "/api/setup", map[string]string{"password": "correct horse"}, nil), 200, "setup")
	want(t, c.do("POST", "/api/setup", map[string]string{"password": "another one!"}, nil), 409, "second setup")

	var email store.Email
	want(t, c.do("POST", "/api/emails", map[string]any{"address": "Me@Example.com", "label": "main"}, &email), 200, "create email")
	if email.Address != "me@example.com" {
		t.Fatalf("address not normalised: %q", email.Address)
	}
	want(t, c.do("POST", "/api/emails", map[string]any{"address": "me@example.com"}, nil), 409, "duplicate email")
	want(t, c.do("POST", "/api/emails", map[string]any{"address": "nope"}, nil), 400, "invalid email")

	var svc store.Service
	want(t, c.do("POST", "/api/services", map[string]any{"name": "Netflix", "emailId": email.ID, "renewal": "2030-01-31", "twoFactor": []string{"PASSKEY", "totp", "bogus", "totp"}}, &svc), 200, "create service")
	if svc.Status != "active" || svc.LoginMethod != "password" {
		t.Fatalf("defaults not applied: %+v", svc)
	}
	if got := strings.Join(svc.TwoFactor, ","); got != "totp,passkey" {
		t.Fatalf("two-factor not cleaned: %q", got)
	}
	want(t, c.do("POST", "/api/services", map[string]any{"name": "X", "emailId": "missing"}, nil), 400, "service with unknown email")
	want(t, c.do("POST", "/api/services", map[string]any{"name": "X", "emailId": email.ID, "renewal": "31/01/2030"}, nil), 400, "bad date")

	var data store.Data
	want(t, c.do("GET", "/api/data", nil, &data), 200, "data")
	if len(data.Emails) != 1 || len(data.Services) != 1 {
		t.Fatalf("unexpected data: %+v", data)
	}

	want(t, c.do("DELETE", "/api/emails/"+email.ID, nil, nil), 409, "delete email with services")

	// Export, wipe via cascade delete, re-import.
	var export transferFile
	want(t, c.do("GET", "/api/export", nil, &export), 200, "export")
	if len(export.Services) != 1 || export.Services[0].Email != "me@example.com" {
		t.Fatalf("bad export: %+v", export)
	}
	want(t, c.do("DELETE", "/api/emails/"+email.ID+"?force=1", nil, nil), 200, "force delete")
	var res importResult
	want(t, c.do("POST", "/api/import", export, &res), 200, "import")
	if res.EmailsAdded != 1 || res.ServicesAdded != 1 {
		t.Fatalf("bad import result: %+v", res)
	}
	var again store.Data
	want(t, c.do("GET", "/api/data", nil, &again), 200, "data after import")
	if len(again.Services) != 1 || strings.Join(again.Services[0].TwoFactor, ",") != "totp,passkey" {
		t.Fatalf("two-factor lost in export/import: %+v", again.Services)
	}
	want(t, c.do("POST", "/api/import", export, &res), 200, "re-import")
	if res.ServicesAdded != 0 || res.Skipped != 1 {
		t.Fatalf("import is not idempotent: %+v", res)
	}

	// CSRF guard: state changes without the custom header are refused.
	req, _ := http.NewRequest("POST", c.ts.URL+"/api/lock", nil)
	resp, _ := c.hc.Do(req)
	want(t, resp.StatusCode, 403, "missing X-Maildex header")
	resp.Body.Close()

	// Manual lock blocks data until the password is re-entered.
	want(t, c.do("POST", "/api/lock", nil, nil), 200, "lock")
	want(t, c.do("GET", "/api/data", nil, nil), 423, "data while locked")
	want(t, c.do("POST", "/api/unlock", map[string]string{"password": "wrong"}, nil), 401, "bad unlock")
	want(t, c.do("POST", "/api/unlock", map[string]string{"password": "correct horse"}, nil), 200, "unlock")
	want(t, c.do("GET", "/api/data", nil, nil), 200, "data after unlock")

	// A new browser must log in, and wrong passwords are throttled.
	other := &client{t: t, ts: c.ts, hc: &http.Client{}, srv: c.srv}
	want(t, other.do("GET", "/api/data", nil, nil), 401, "anonymous data")
	for i := 0; i < maxFails; i++ {
		want(t, other.do("POST", "/api/login", map[string]string{"password": "nope"}, nil), 401, "bad login")
	}
	want(t, other.do("POST", "/api/login", map[string]string{"password": "correct horse"}, nil), 429, "throttled login")
}

func TestPasswordHash(t *testing.T) {
	enc, err := hashPassword("s3cret-pass")
	if err != nil {
		t.Fatal(err)
	}
	if !checkPassword(enc, "s3cret-pass") || checkPassword(enc, "s3cret-pasS") || checkPassword("garbage", "x") {
		t.Fatal("password verification is wrong")
	}
}

func TestNewer(t *testing.T) {
	for _, tc := range []struct {
		a, b string
		want bool
	}{{"v1.2.0", "v1.1.9", true}, {"v1.0.0", "v1.0.0", false}, {"v0.9.0", "v1.0.0", false}, {"v1.10.0", "v1.9.0", true}} {
		if newer(tc.a, tc.b) != tc.want {
			t.Errorf("newer(%s,%s) != %v", tc.a, tc.b, tc.want)
		}
	}
}
