package store

import (
	"os"
	"path/filepath"
	"slices"
	"testing"
)

// Data written before folders existed has a single "category" per service. It
// must become folders on load, without losing anything.
func TestMigratesCategoriesToFolders(t *testing.T) {
	dir := t.TempDir()
	old := `{
  "emails": [{"id":"e1","address":"me@example.com"}],
  "services": [
    {"id":"s1","name":"Chase","emailId":"e1","category":"Banks"},
    {"id":"s2","name":"Revolut","emailId":"e1","category":"banks"},
    {"id":"s3","name":"Netflix","emailId":"e1","category":"Streaming"},
    {"id":"s4","name":"Misc","emailId":"e1"}
  ],
  "settings": {"lockMinutes": 5}
}`
	if err := os.WriteFile(filepath.Join(dir, "data.json"), []byte(old), 0o600); err != nil {
		t.Fatal(err)
	}

	s, err := Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	s.View(func(d *Data, _ *Auth) {
		if len(d.Folders) != 2 {
			t.Fatalf("want 2 folders (case-insensitive merge), got %+v", d.Folders)
		}
		banks := d.Folders[d.FolderIndex("Banks")].ID
		if !slices.Equal(d.Services[0].Folders, []string{banks}) || !slices.Equal(d.Services[1].Folders, []string{banks}) {
			t.Fatalf("banks services: %v %v", d.Services[0].Folders, d.Services[1].Folders)
		}
		if d.Services[3].Folders == nil || len(d.Services[3].Folders) != 0 {
			t.Fatalf("uncategorised service should have an empty, non-nil list: %#v", d.Services[3].Folders)
		}
		if d.Services[0].Category != "" {
			t.Fatal("category should be cleared after migration")
		}
	})

	if b, err := os.ReadFile(filepath.Join(dir, "data.pre-folders.json")); err != nil || string(b) != old {
		t.Fatalf("the original data must be kept untouched: %v", err)
	}

	// Reopening is stable: no new folders, and the pre-migration copy is not overwritten.
	s2, err := Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	s2.View(func(d *Data, _ *Auth) {
		if len(d.Folders) != 2 {
			t.Fatalf("reopen changed folders: %+v", d.Folders)
		}
	})
	if b, _ := os.ReadFile(filepath.Join(dir, "data.pre-folders.json")); string(b) != old {
		t.Fatal("pre-migration copy was overwritten")
	}
}
