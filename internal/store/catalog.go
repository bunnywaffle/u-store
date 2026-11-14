package store

import (
	_ "embed"
	"encoding/json"
	"fmt"
	"strings"
)

//go:embed catalog.json
var catalogData []byte

// catalogApp is one curated entry in catalog.json. Flathub IDs double as the
// app identity and the icon URL source; native overrides map the package name
// on each family of the package manager it belongs to.
type catalogApp struct {
	ID         string            `json:"id"`
	Name       string            `json:"name"`
	Summary    string            `json:"summary"`
	Developer  string            `json:"developer"`
	Categories []string          `json:"categories"`
	Verified   bool              `json:"verified"`
	Featured   bool              `json:"featured"`
	Native     map[string]string `json:"native"` // backend id -> package name
}

type catalogFile struct {
	Categories []Category   `json:"categories"`
	Apps       []catalogApp `json:"apps"`
}

// Catalog is the parsed, in-memory curated catalog.
type Catalog struct {
	Categories []Category
	Apps       []catalogApp
	byID       map[string]*catalogApp
}

// LoadCatalog parses the embedded catalog. It only fails on a programmer
// error (catalog.json is embedded and checked in), so it panics with context.
func LoadCatalog() (*Catalog, error) {
	var f catalogFile
	if err := json.Unmarshal(catalogData, &f); err != nil {
		return nil, fmt.Errorf("catalog: %w", err)
	}
	c := &Catalog{Categories: f.Categories, byID: map[string]*catalogApp{}}
	for i := range f.Apps {
		app := &f.Apps[i]
		c.byID[app.ID] = app
		c.Apps = append(c.Apps, *app)
	}
	return c, nil
}

// NativePackage returns the package name for the given backend, or "".
func (c *Catalog) NativePackage(id, backend string) string {
	if app, ok := c.byID[id]; ok {
		return app.Native[backend]
	}
	return ""
}

// Find returns the curated entry for a Flathub ID, or nil.
func (c *Catalog) Find(id string) *catalogApp {
	return c.byID[id]
}

// ForCategory lists curated apps of one category, featured first.
func (c *Catalog) ForCategory(categoryID string) []catalogApp {
	var out []catalogApp
	for _, app := range c.Apps {
		for _, cat := range app.Categories {
			if cat == categoryID {
				out = append(out, app)
				break
			}
		}
	}
	// featured first, then alphabetical — stable and predictable
	for i := 1; i < len(out); i++ {
		for j := i; j > 0 && less(out[j], out[j-1]); j-- {
			out[j], out[j-1] = out[j-1], out[j]
		}
	}
	return out
}

// Search filters the catalog locally (offline fallback and instant results).
func (c *Catalog) Search(query string) []catalogApp {
	q := strings.ToLower(strings.TrimSpace(query))
	if q == "" {
		return nil
	}
	var out []catalogApp
	for _, app := range c.Apps {
		hay := strings.ToLower(app.Name + " " + app.Summary + " " + app.Developer + " " + strings.Join(app.Categories, " "))
		if strings.Contains(hay, q) {
			out = append(out, app)
		}
	}
	return out
}

func less(a, b catalogApp) bool {
	if a.Featured != b.Featured {
		return a.Featured
	}
	return strings.ToLower(a.Name) < strings.ToLower(b.Name)
}

// iconFor builds the canonical 128px icon URL for a Flathub app ID.
func iconFor(id string) string {
	return "https://dl.flathub.org/media/icons/128x128/" + id + ".png"
}
