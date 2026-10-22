package server

import (
	"net/http"
	"slices"
	"strings"
	"time"

	"github.com/Bagareal/maildex/internal/store"
)

// The export/import format refers to emails by address and folders by name
// rather than internal ID so a file can be edited by hand, merged, or produced
// from a spreadsheet.
type transferService struct {
	serviceCore
	Email   string   `json:"email"`
	Folders []string `json:"folders"`
	// Category is what files from before folders existed (and CSVs) call a
	// single folder; it is merged into Folders on import.
	Category string `json:"category,omitempty"`
}

type transferFile struct {
	App      string            `json:"app,omitempty"`
	Version  int               `json:"version,omitempty"`
	Exported time.Time         `json:"exported,omitempty"`
	Emails   []emailInput      `json:"emails"`
	Folders  []folderInput     `json:"folders"`
	Services []transferService `json:"services"`
}

func (s *Server) handleExport(w http.ResponseWriter, r *http.Request) {
	out := transferFile{App: "maildex", Version: 2, Exported: time.Now().UTC(), Emails: []emailInput{}, Folders: []folderInput{}, Services: []transferService{}}
	s.st.View(func(d *store.Data, _ *store.Auth) {
		addr := map[string]string{}
		for _, e := range d.Emails {
			addr[e.ID] = e.Address
			out.Emails = append(out.Emails, emailInput{e.Address, e.Label, e.Provider, e.Recovery, e.Notes, e.AtRisk})
		}
		names := map[string]string{}
		for _, f := range d.Folders {
			names[f.ID] = f.Name
			out.Folders = append(out.Folders, folderInput{f.Name, f.Color})
		}
		for _, sv := range d.Services {
			folders := []string{}
			for _, id := range sv.Folders {
				if n, ok := names[id]; ok {
					folders = append(folders, n)
				}
			}
			out.Services = append(out.Services, transferService{
				serviceCore: serviceCore{Name: sv.Name, Username: sv.Username, LoginMethod: sv.LoginMethod, Status: sv.Status,
					TwoFactor: sv.TwoFactor, URL: sv.URL, Renewal: sv.Renewal, Notes: sv.Notes},
				Email:   addr[sv.EmailID],
				Folders: folders,
			})
		}
	})
	w.Header().Set("Content-Disposition", `attachment; filename="maildex-export-`+time.Now().Format("2006-01-02")+`.json"`)
	writeJSON(w, http.StatusOK, out)
}

type importResult struct {
	EmailsAdded   int `json:"emailsAdded"`
	FoldersAdded  int `json:"foldersAdded"`
	ServicesAdded int `json:"servicesAdded"`
	Skipped       int `json:"skipped"`
}

// handleImport merges a transfer file into the existing data. Existing emails
// and folders are matched by address / name and services that already exist
// (same name, email and username) are skipped, so importing twice is harmless.
func (s *Server) handleImport(w http.ResponseWriter, r *http.Request) {
	var in transferFile
	if !readBody(w, r, maxImportBody, &in) {
		return
	}
	if len(in.Emails)+len(in.Services)+len(in.Folders) > 20000 {
		fail(w, http.StatusBadRequest, "file too large")
		return
	}
	var res importResult
	err := s.st.Update(func(d *store.Data, _ *store.Auth) error {
		res = importResult{}
		ensureEmail := func(in emailInput) (string, bool) {
			e, err := in.normalize()
			if err != nil {
				return "", false
			}
			if i := findEmailByAddress(d, e.Address); i >= 0 {
				return d.Emails[i].ID, true
			}
			e.ID, e.Created = store.NewID(), time.Now()
			d.Emails = append(d.Emails, e)
			res.EmailsAdded++
			return e.ID, true
		}
		ensureFolder := func(in folderInput) (string, bool) {
			name, color, err := in.normalize()
			if err != nil || (d.FolderIndex(name) < 0 && len(d.Folders) >= maxFolders) {
				return "", false
			}
			id, created := d.EnsureFolder(name)
			if created {
				res.FoldersAdded++
				if color != "" {
					d.Folders[d.FolderIndex(name)].Color = color
				}
			}
			return id, true
		}
		for _, e := range in.Emails {
			ensureEmail(e)
		}
		for _, f := range in.Folders {
			ensureFolder(f)
		}
		for _, ts := range in.Services {
			sv, err := ts.serviceCore.normalize()
			if err != nil {
				res.Skipped++
				continue
			}
			eid, valid := ensureEmail(emailInput{Address: ts.Email})
			if !valid {
				res.Skipped++
				continue
			}
			sv.EmailID = eid
			dup := false
			for _, ex := range d.Services {
				if ex.EmailID == eid && strings.EqualFold(ex.Name, sv.Name) && strings.EqualFold(ex.Username, sv.Username) {
					dup = true
					break
				}
			}
			if dup {
				res.Skipped++
				continue
			}
			sv.Folders = []string{}
			for _, name := range append(slices.Clone(ts.Folders), ts.Category) {
				if strings.TrimSpace(name) == "" {
					continue
				}
				if id, ok := ensureFolder(folderInput{Name: name}); ok && !slices.Contains(sv.Folders, id) {
					sv.Folders = append(sv.Folders, id)
				}
			}
			sv.ID, sv.Created = store.NewID(), time.Now()
			d.Services = append(d.Services, sv)
			res.ServicesAdded++
		}
		return nil
	})
	if err != nil {
		fail(w, http.StatusInternalServerError, "could not save")
		return
	}
	writeJSON(w, http.StatusOK, res)
}
