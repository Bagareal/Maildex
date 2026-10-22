package server

import (
	"errors"
	"net/http"
	"regexp"
	"slices"
	"strings"

	"github.com/Bagareal/maildex/internal/store"
)

var (
	errFolderDup = errors.New("a folder with that name already exists")
	colorRE      = regexp.MustCompile(`^#[0-9a-fA-F]{6}$`)
)

const maxFolders = 500

type folderInput struct {
	Name  string `json:"name"`
	Color string `json:"color"`
}

func (in folderInput) normalize() (name, color string, err error) {
	name = clip(in.Name, 40)
	if name == "" {
		return "", "", errors.New("give the folder a name")
	}
	color = strings.TrimSpace(in.Color)
	if color != "" && !colorRE.MatchString(color) {
		return "", "", errors.New("color must look like #6366f1")
	}
	return name, strings.ToLower(color), nil
}

// keepFolders drops unknown and duplicate IDs, keeping the caller's order.
func keepFolders(d *store.Data, ids []string) []string {
	out := []string{}
	for _, id := range ids {
		known := slices.ContainsFunc(d.Folders, func(f store.Folder) bool { return f.ID == id })
		if known && !slices.Contains(out, id) {
			out = append(out, id)
		}
	}
	return out
}

func (s *Server) handleFolderCreate(w http.ResponseWriter, r *http.Request) {
	var in folderInput
	if !readBody(w, r, maxBody, &in) {
		return
	}
	name, color, err := in.normalize()
	if err != nil {
		fail(w, http.StatusBadRequest, err.Error())
		return
	}
	var f store.Folder
	err = s.st.Update(func(d *store.Data, _ *store.Auth) error {
		if d.FolderIndex(name) >= 0 {
			return errFolderDup
		}
		if len(d.Folders) >= maxFolders {
			return errors.New("too many folders")
		}
		d.EnsureFolder(name)
		i := d.FolderIndex(name)
		if color != "" {
			d.Folders[i].Color = color
		}
		f = d.Folders[i]
		return nil
	})
	respond(w, err, f)
}

func (s *Server) handleFolderUpdate(w http.ResponseWriter, r *http.Request) {
	var in folderInput
	if !readBody(w, r, maxBody, &in) {
		return
	}
	name, color, err := in.normalize()
	if err != nil {
		fail(w, http.StatusBadRequest, err.Error())
		return
	}
	id := r.PathValue("id")
	var f store.Folder
	err = s.st.Update(func(d *store.Data, _ *store.Auth) error {
		i := slices.IndexFunc(d.Folders, func(x store.Folder) bool { return x.ID == id })
		if i < 0 {
			return errNotFound
		}
		if j := d.FolderIndex(name); j >= 0 && j != i {
			return errFolderDup
		}
		d.Folders[i].Name = name
		if color != "" {
			d.Folders[i].Color = color
		}
		f = d.Folders[i]
		return nil
	})
	respond(w, err, f)
}

// handleFolderDelete removes the folder only; its services stay, just no longer in it.
func (s *Server) handleFolderDelete(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	err := s.st.Update(func(d *store.Data, _ *store.Auth) error {
		i := slices.IndexFunc(d.Folders, func(x store.Folder) bool { return x.ID == id })
		if i < 0 {
			return errNotFound
		}
		d.Folders = slices.Delete(d.Folders, i, i+1)
		for k := range d.Services {
			d.Services[k].Folders = slices.DeleteFunc(d.Services[k].Folders, func(x string) bool { return x == id })
		}
		return nil
	})
	respond(w, err, map[string]bool{"ok": true})
}

type bulkInput struct {
	IDs       []string `json:"ids"`
	Action    string   `json:"action"`
	FolderID  string   `json:"folderId"`
	TwoFactor []string `json:"twoFactor"`
	Status    string   `json:"status"`
}

var errBadBulk = errors.New("invalid bulk request")

// handleServiceBulk applies one action to many services at once. Folder and
// 2FA actions add or remove only what is named and leave the rest untouched.
func (s *Server) handleServiceBulk(w http.ResponseWriter, r *http.Request) {
	var in bulkInput
	if !readBody(w, r, maxBody, &in) {
		return
	}
	if len(in.IDs) == 0 || len(in.IDs) > 20000 {
		fail(w, http.StatusBadRequest, "select at least one service")
		return
	}
	tf := cleanTwoFactor(in.TwoFactor)
	affected := 0
	err := s.st.Update(func(d *store.Data, _ *store.Auth) error {
		// Validate before touching anything.
		switch in.Action {
		case "addFolder", "removeFolder":
			if !slices.ContainsFunc(d.Folders, func(f store.Folder) bool { return f.ID == in.FolderID }) {
				return errNotFound
			}
		case "addTwoFactor", "removeTwoFactor":
			if len(tf) == 0 {
				return errBadBulk
			}
		case "setStatus":
			if !statuses[in.Status] {
				return errBadBulk
			}
		case "delete":
		default:
			return errBadBulk
		}
		chosen := make(map[string]bool, len(in.IDs))
		for _, id := range in.IDs {
			chosen[id] = true
		}
		if in.Action == "delete" {
			kept := d.Services[:0:0]
			for _, sv := range d.Services {
				if chosen[sv.ID] {
					affected++
				} else {
					kept = append(kept, sv)
				}
			}
			d.Services = kept
			return nil
		}
		for i := range d.Services {
			sv := &d.Services[i]
			if !chosen[sv.ID] {
				continue
			}
			affected++
			switch in.Action {
			case "addFolder":
				if !slices.Contains(sv.Folders, in.FolderID) {
					sv.Folders = append(sv.Folders, in.FolderID)
				}
			case "removeFolder":
				sv.Folders = slices.DeleteFunc(sv.Folders, func(x string) bool { return x == in.FolderID })
			case "addTwoFactor":
				sv.TwoFactor = cleanTwoFactor(append(slices.Clone(sv.TwoFactor), tf...))
			case "removeTwoFactor":
				sv.TwoFactor = slices.DeleteFunc(slices.Clone(sv.TwoFactor), func(x string) bool { return slices.Contains(tf, x) })
			case "setStatus":
				sv.Status = in.Status
			}
		}
		return nil
	})
	switch {
	case errors.Is(err, errBadBulk):
		fail(w, http.StatusBadRequest, err.Error())
	default:
		respond(w, err, map[string]int{"affected": affected})
	}
}
