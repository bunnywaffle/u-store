package store

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

const flathubBase = "https://flathub.org/api"

// Flathub is a small client for the public Flathub API. Every call takes a
// context so navigation away from a page cancels in-flight requests.
type Flathub struct {
	client *http.Client
}

// NewFlathub returns a client with a sane timeout and UA.
func NewFlathub() *Flathub {
	return &Flathub{client: &http.Client{Timeout: 12 * time.Second}}
}

type flathubSearchRequest struct {
	Query   string `json:"query"`
	Lang    string `json:"lang"`
	Page    int    `json:"page"`
	PerPage int    `json:"per_page"`
}

type flathubSearchResponse struct {
	Hits      []flathubHit `json:"hits"`
	TotalHits int          `json:"totalHits"`
}

// stringOrList decodes a JSON value that is either a single string
// or an array of strings — Flathub uses both for main_categories.
type stringOrList []string

func (s *stringOrList) UnmarshalJSON(data []byte) error {
	str := bytes.TrimSpace(data)
	if len(str) == 0 || bytes.Equal(str, []byte("null")) {
		*s = nil
		return nil
	}
	if str[0] == '"' {
		var single string
		if err := json.Unmarshal(str, &single); err != nil {
			return err
		}
		*s = stringOrList{single}
		return nil
	}
	var list []string
	if err := json.Unmarshal(str, &list); err != nil {
		return err
	}
	*s = list
	return nil
}

type flathubHit struct {
	AppID              string       `json:"app_id"`
	Name               string       `json:"name"`
	Summary            string       `json:"summary"`
	Icon               string       `json:"icon"`
	MainCategories     stringOrList `json:"main_categories"`
	DeveloperName      string   `json:"developer_name"`
	VerificationVerified bool   `json:"verification_verified"`
	InstallsLastMonth  int64    `json:"installs_last_month"`
	IsFreeLicense      bool     `json:"is_free_license"`
}

// Search queries Flathub. An empty query returns the most installed apps,
// which is exactly what the Discover view wants.
func (f *Flathub) Search(ctx context.Context, query string, page, perPage int) (hits []flathubHit, total int, err error) {
	if perPage <= 0 || perPage > 100 {
		perPage = 48
	}
	if page < 1 {
		page = 1
	}
	body, err := json.Marshal(flathubSearchRequest{Query: query, Lang: "en", Page: page, PerPage: perPage})
	if err != nil {
		return nil, 0, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, flathubBase+"/v2/search", bytes.NewReader(body))
	if err != nil {
		return nil, 0, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", "UniversalStore/0.1 (+https://mygo.run)")
	resp, err := f.client.Do(req)
	if err != nil {
		return nil, 0, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, 0, fmt.Errorf("flathub search: %s", resp.Status)
	}
	var out flathubSearchResponse
	if err := json.NewDecoder(io.LimitReader(resp.Body, 32<<20)).Decode(&out); err != nil {
		return nil, 0, err
	}
	return out.Hits, out.TotalHits, nil
}

type flathubAppstream struct {
	ID                string   `json:"id"`
	Name              string   `json:"name"`
	Summary           string   `json:"summary"`
	Description       string   `json:"description"`
	ProjectLicense    string   `json:"project_license"`
	DeveloperName     string   `json:"developer_name"`
	Icon              string   `json:"icon"`
	Categories        []string `json:"categories"`
	Urls              struct {
		Homepage string `json:"homepage"`
	} `json:"urls"`
	Metadata map[string]json.RawMessage `json:"metadata"`
	Releases []struct {
		Timestamp string `json:"timestamp"`
	} `json:"releases"`
	Screenshots []struct {
		Sizes []struct {
			Src    string `json:"src"`
			Width  string `json:"width"`
		} `json:"sizes"`
		Caption string `json:"caption"`
	} `json:"screenshots"`
}

// verifiedFromMetadata reads flathub::verification::verified, which Flathub
// emits as a bool, a string, or occasionally a number.
func verifiedFromMetadata(m map[string]json.RawMessage) bool {
	raw, ok := m["flathub::verification::verified"]
	if !ok {
		return false
	}
	var b bool
	if json.Unmarshal(raw, &b) == nil {
		return b
	}
	var s string
	if json.Unmarshal(raw, &s) == nil {
		return strings.EqualFold(s, "true") || s == "1"
	}
	var n float64
	if json.Unmarshal(raw, &n) == nil {
		return n != 0
	}
	return false
}

// Appstream fetches full metadata for one app.
func (f *Flathub) Appstream(ctx context.Context, appID string) (*flathubAppstream, error) {
	if !isSafeAppID(appID) {
		return nil, fmt.Errorf("invalid app id %q", appID)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, flathubBase+"/v2/appstream/"+appID, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "UniversalStore/0.1 (+https://mygo.run)")
	resp, err := f.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("flathub appstream: %s", resp.Status)
	}
	var out flathubAppstream
	if err := json.NewDecoder(io.LimitReader(resp.Body, 32<<20)).Decode(&out); err != nil {
		return nil, err
	}
	return &out, nil
}

// isSafeAppID guards the URL path segment against traversal.
func isSafeAppID(id string) bool {
	if id == "" || len(id) > 200 || strings.Contains(id, "..") || strings.ContainsAny(id, "/\\?#%") {
		return false
	}
	for _, r := range id {
		ok := r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' || r >= '0' && r <= '9' || r == '.' || r == '-' || r == '_'
		if !ok {
			return false
		}
	}
	return true
}

// iconOrDefault returns the hit's icon, falling back to the canonical URL.
func iconOrDefault(icon, appID string) string {
	if icon != "" {
		return icon
	}
	return iconFor(appID)
}

// screenshotsFromAppstream picks one good-resolution screenshot source per
// entry, capped at count, skipping entries with nothing usable.
func screenshotsFromAppstream(as *flathubAppstream, count int) []string {
	var out []string
	for _, s := range as.Screenshots {
		best := ""
		bestWidth := 0
		for _, sz := range s.Sizes {
			var w int
			fmt.Sscanf(sz.Width, "%d", &w)
			if w > bestWidth || (best == "" && w > 0) {
				if w >= 640 && w <= 1920 {
					best = sz.Src
					bestWidth = w
				} else if best == "" {
					best = sz.Src
					bestWidth = w
				}
			}
		}
		if best == "" && len(s.Sizes) > 0 {
			best = s.Sizes[0].Src
		}
		if best != "" {
			out = append(out, best)
		}
		if len(out) >= count {
			break
		}
	}
	return out
}
