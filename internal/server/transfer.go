package server

import (
	"net/http"
	"strings"
	"time"

	"github.com/Bagareal/maildex/internal/store"
)

// The export/import format refers to emails by address rather than internal
// ID so a file can be edited by hand, merged, or produced from a spreadsheet.
type transferService struct {
	serviceInput
	Email string `json:"email"`
}

type transferFile struct {
	App      string            `json:"app,omitempty"`
	Version  int               `json:"version,omitempty"`
	Exported time.Time         `json:"exported,omitempty"`
	Emails   []emailInput      `json:"emails"`
	Services []transferService `json:"services"`
}

func (s *Server) handleExport(w http.ResponseWriter, r *http.Request) {
	out := transferFile{App: "maildex", Version: 1, Exported: time.Now().UTC(), Emails: []emailInput{}, Services: []transferService{}}
	s.st.View(func(d *store.Data, _ *store.Auth) {
		addr := map[string]string{}
		for _, e := range d.Emails {
			addr[e.ID] = e.Address
			out.Emails = append(out.Emails, emailInput{e.Address, e.Label, e.Provider, e.Recovery, e.Notes, e.AtRisk})
		}
		for _, sv := range d.Services {
			out.Services = append(out.Services, transferService{
				serviceInput: serviceInput{Name: sv.Name, Username: sv.Username, LoginMethod: sv.LoginMethod, Category: sv.Category,
					Status: sv.Status, URL: sv.URL, Renewal: sv.Renewal, Notes: sv.Notes},
				Email: addr[sv.EmailID],
			})
		}
	})
	w.Header().Set("Content-Disposition", `attachment; filename="maildex-export-`+time.Now().Format("2006-01-02")+`.json"`)
	writeJSON(w, http.StatusOK, out)
}

type importResult struct {
	EmailsAdded   int `json:"emailsAdded"`
	ServicesAdded int `json:"servicesAdded"`
	Skipped       int `json:"skipped"`
}

// handleImport merges a transfer file into the existing data. Existing emails
// are matched by address and services that already exist (same name, email and
// username) are skipped, so importing twice is harmless.
func (s *Server) handleImport(w http.ResponseWriter, r *http.Request) {
	var in transferFile
	if !readBody(w, r, maxImportBody, &in) {
		return
	}
	if len(in.Emails)+len(in.Services) > 20000 {
		fail(w, http.StatusBadRequest, "file too large")
		return
	}
	var res importResult
	err := s.st.Update(func(d *store.Data, _ *store.Auth) error {
		res = importResult{}
		ensure := func(in emailInput) (string, bool) {
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
		for _, e := range in.Emails {
			ensure(e)
		}
		for _, ts := range in.Services {
			sv, err := ts.serviceInput.normalize()
			if err != nil {
				res.Skipped++
				continue
			}
			eid, valid := ensure(emailInput{Address: ts.Email})
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
