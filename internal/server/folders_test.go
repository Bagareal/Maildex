package server

import (
	"slices"
	"strings"
	"testing"

	"github.com/Bagareal/maildex/internal/store"
)

func TestFoldersAndBulk(t *testing.T) {
	c := newClient(t)
	want(t, c.do("POST", "/api/setup", map[string]string{"password": "correct horse"}, nil), 200, "setup")

	var email store.Email
	want(t, c.do("POST", "/api/emails", map[string]any{"address": "me@example.com"}, &email), 200, "email")

	var money, work store.Folder
	want(t, c.do("POST", "/api/folders", map[string]any{"name": "Soldi", "color": "#10B981"}, &money), 200, "create folder")
	if money.Color != "#10b981" {
		t.Fatalf("color not normalised: %q", money.Color)
	}
	want(t, c.do("POST", "/api/folders", map[string]any{"name": "Lavoro"}, &work), 200, "create second folder")
	if work.Color == "" {
		t.Fatal("expected an automatic color")
	}
	want(t, c.do("POST", "/api/folders", map[string]any{"name": "soldi"}, nil), 409, "duplicate folder name")
	want(t, c.do("POST", "/api/folders", map[string]any{"name": "x", "color": "red"}, nil), 400, "bad color")
	want(t, c.do("POST", "/api/folders", map[string]any{"name": "  "}, nil), 400, "empty name")
	want(t, c.do("PUT", "/api/folders/"+work.ID, map[string]any{"name": "SOLDI"}, nil), 409, "rename onto existing")
	want(t, c.do("PUT", "/api/folders/"+work.ID, map[string]any{"name": "Work"}, &work), 200, "rename")

	mk := func(name string, folders []string, tf []string) store.Service {
		var sv store.Service
		want(t, c.do("POST", "/api/services", map[string]any{"name": name, "emailId": email.ID, "folders": folders, "twoFactor": tf}, &sv), 200, "create "+name)
		return sv
	}
	paypal := mk("PayPal", []string{money.ID, work.ID, "bogus", money.ID}, []string{"totp"})
	bank := mk("Bank", nil, nil)
	if !slices.Equal(paypal.Folders, []string{money.ID, work.ID}) {
		t.Fatalf("unknown and duplicate folders should be dropped: %v", paypal.Folders)
	}

	bulk := func(body map[string]any) (int, int) {
		var out map[string]int
		code := c.do("POST", "/api/services/bulk", body, &out)
		return code, out["affected"]
	}
	ids := []string{paypal.ID, bank.ID}

	if code, n := bulk(map[string]any{"ids": ids, "action": "addFolder", "folderId": money.ID}); code != 200 || n != 2 {
		t.Fatalf("addFolder: %d %d", code, n)
	}
	if code, _ := bulk(map[string]any{"ids": ids, "action": "addFolder", "folderId": "nope"}); code != 404 {
		t.Fatalf("unknown folder should be 404, got %d", code)
	}
	if code, _ := bulk(map[string]any{"ids": ids, "action": "explode"}); code != 400 {
		t.Fatalf("unknown action should be 400, got %d", code)
	}
	if code, _ := bulk(map[string]any{"ids": []string{}, "action": "delete"}); code != 400 {
		t.Fatalf("empty selection should be 400, got %d", code)
	}
	if code, _ := bulk(map[string]any{"ids": ids, "action": "addTwoFactor", "twoFactor": []string{"nonsense"}}); code != 400 {
		t.Fatalf("2FA with no valid method should be 400, got %d", code)
	}
	// Adding keeps what is already there.
	bulk(map[string]any{"ids": ids, "action": "addTwoFactor", "twoFactor": []string{"sms", "passkey"}})

	get := func() map[string]store.Service {
		var d store.Data
		want(t, c.do("GET", "/api/data", nil, &d), 200, "data")
		m := map[string]store.Service{}
		for _, sv := range d.Services {
			m[sv.Name] = sv
		}
		return m
	}
	sv := get()
	if got := strings.Join(sv["PayPal"].TwoFactor, ","); got != "totp,passkey,sms" {
		t.Fatalf("existing 2FA must be kept and order stable: %q", got)
	}
	if !slices.Contains(sv["Bank"].Folders, money.ID) || len(sv["Bank"].Folders) != 1 {
		t.Fatalf("bank folders: %v", sv["Bank"].Folders)
	}

	bulk(map[string]any{"ids": ids, "action": "removeTwoFactor", "twoFactor": []string{"totp", "sms"}})
	bulk(map[string]any{"ids": []string{bank.ID}, "action": "setStatus", "status": "to-close"})
	bulk(map[string]any{"ids": ids, "action": "removeFolder", "folderId": money.ID})
	sv = get()
	if got := strings.Join(sv["PayPal"].TwoFactor, ","); got != "passkey" {
		t.Fatalf("removeTwoFactor: %q", got)
	}
	if sv["Bank"].Status != "to-close" || sv["PayPal"].Status != "active" {
		t.Fatalf("setStatus hit the wrong services: %+v", sv)
	}
	if slices.Contains(sv["PayPal"].Folders, money.ID) || !slices.Contains(sv["PayPal"].Folders, work.ID) {
		t.Fatalf("removeFolder: %v", sv["PayPal"].Folders)
	}

	// Export carries folder names and colors; deleting a folder keeps its services.
	var export transferFile
	want(t, c.do("GET", "/api/export", nil, &export), 200, "export")
	if len(export.Folders) != 2 || export.Services[0].Folders[0] != "Work" {
		t.Fatalf("export folders: %+v", export)
	}
	want(t, c.do("DELETE", "/api/folders/"+work.ID, nil, nil), 200, "delete folder")
	sv = get()
	if len(sv) != 2 || len(sv["PayPal"].Folders) != 0 {
		t.Fatalf("deleting a folder must only unfile its services: %+v", sv)
	}
	want(t, c.do("DELETE", "/api/folders/"+work.ID, nil, nil), 404, "delete missing folder")

	// Import recreates folders, honours colors, and still understands the old "category".
	var res importResult
	imp := map[string]any{
		"folders": []map[string]any{{"name": "Work", "color": "#ef4444"}},
		"services": []map[string]any{
			{"name": "Slack", "email": "me@example.com", "folders": []string{"Work", "Team"}},
			{"name": "Old", "email": "me@example.com", "category": "Legacy"},
		},
	}
	want(t, c.do("POST", "/api/import", imp, &res), 200, "import")
	if res.ServicesAdded != 2 || res.FoldersAdded != 3 {
		t.Fatalf("import result: %+v", res)
	}
	var d store.Data
	want(t, c.do("GET", "/api/data", nil, &d), 200, "data")
	if i := d.FolderIndex("Work"); i < 0 || d.Folders[i].Color != "#ef4444" {
		t.Fatalf("imported folder color lost: %+v", d.Folders)
	}
	if d.FolderIndex("Legacy") < 0 {
		t.Fatal("legacy category should become a folder")
	}

	if code, n := bulk(map[string]any{"ids": []string{paypal.ID, bank.ID}, "action": "delete"}); code != 200 || n != 2 {
		t.Fatalf("bulk delete: %d %d", code, n)
	}
	want(t, c.do("GET", "/api/data", nil, &d), 200, "data after delete")
	if len(d.Services) != 2 {
		t.Fatalf("expected the two imported services to remain, got %d", len(d.Services))
	}
}
