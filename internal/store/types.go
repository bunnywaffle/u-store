// Package store implements the U Store backend: distro and package
// manager detection, a curated catalog, live Flathub integration, and the
// install/remove/update operations behind them.
//
// The package is split by responsibility so it stays easy to grow:
//
//	types.go     the JSON types shared with the frontend (stable contract)
//	catalog.go   the embedded, curated catalog (catalog.json)
//	detect.go    distro and package manager detection (/etc/os-release)
//	flathub.go   the Flathub API client (search, details)
//	backends.go  one file per install backend lives here, all behind Source
//	service.go   the Store service bound to the frontend
package store

// AppSummary is the card-shaped view of an app, used in grids and lists.
type AppSummary struct {
	ID            string   `json:"id"` // Flathub app ID (or catalog slug)
	Name          string   `json:"name"`
	Summary       string   `json:"summary"`
	Icon          string   `json:"icon"`
	Categories    []string `json:"categories"`
	Developer     string   `json:"developer,omitempty"`
	Verified      bool     `json:"verified"`
	Source        string   `json:"source"` // preferred backend for this app on this machine
	Sources       []string `json:"sources"`
	NativePackage string   `json:"nativePackage,omitempty"` // native package name, when known
	Installed     bool     `json:"installed"`
	Featured      bool     `json:"featured,omitempty"`
	InstallsLastMonth int64 `json:"installsLastMonth,omitempty"`
}

// AppDetails is the full page for one app.
type AppDetails struct {
	AppSummary
	Description    string   `json:"description"` // HTML, sanitized on the page
	License        string   `json:"license"`
	Homepage       string   `json:"homepage"`
	UpdatedAt      int64    `json:"updatedAt"` // unix seconds, 0 = unknown
	InstallsLastMonth int64 `json:"installsLastMonth"`
	Origin         string   `json:"origin"` // "flathub" | "catalog"
	Screenshots    []string `json:"screenshots"` // up to 4 screenshot URLs, empty when none
}

// Category is one sidebar entry in the Browse view.
type Category struct {
	ID    string `json:"id"`
	Name  string `json:"name"`
	Icon  string `json:"icon"`  // key of an inline SVG in the frontend
	Query string `json:"query"` // keyword handed to Flathub search
}

// SearchResult is one page of search hits.
type SearchResult struct {
	Query       string       `json:"query"`
	Total       int          `json:"total"`
	Hits        []AppSummary `json:"hits"`
	FromFlathub bool         `json:"fromFlathub"` // false when served from the offline catalog
}

// DiscoverResult is everything the Discover view shows on load.
type DiscoverResult struct {
	Trending    []AppSummary `json:"trending"`
	Featured    []AppSummary `json:"featured"`
	Categories  []Category   `json:"categories"`
	FromFlathub bool         `json:"fromFlathub"`
}

// BackendInfo describes one install backend on this machine.
type BackendInfo struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Kind        string `json:"kind"` // "universal" (flatpak, snap) | "native"
	Available   bool   `json:"available"`
	Description string `json:"description"`
}

// SystemInfo is shown in the sidebar footer and Settings.
type SystemInfo struct {
	DistroID        string        `json:"distroId"`
	DistroName      string        `json:"distroName"`
	PrettyName      string        `json:"prettyName"`
	Arch            string        `json:"arch"`
	Backends        []BackendInfo `json:"backends"`
	FlathubEnabled  bool          `json:"flathubEnabled"`
	NativeBackendID string        `json:"nativeBackendId"` // "" when none was detected
	Theme           string        `json:"theme"`           // dark | light | system
	Atomic          bool          `json:"atomic"`           // rpm-ostree (Silverblue, Bluefin, …)
}

// InstalledSource says which backend an installed app came from.
type InstalledSource struct {
	Backend string `json:"backend"` // flatpak | apt | dnf | ...
	Ref     string `json:"ref"`     // flathub app id or native package name
	Scope   string `json:"scope"`   // flatpak only: "user" | "system"
}

// InstalledApp is an installed app, ready to render.
type InstalledApp struct {
	AppSummary
	Sources []InstalledSource `json:"sources"`
}

// UpdateInfo is one pending update.
type UpdateInfo struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	Icon      string `json:"icon"`
	Backend   string `json:"backend"`
	Ref       string `json:"ref"`
	Scope     string `json:"scope"`
	From      string `json:"from"`
	To        string `json:"to"`
}

// InstallEvent is streamed to the caller while install/remove/update runs.
type InstallEvent struct {
	Type    string  `json:"type"`              // start | line | progress | done | error
	Line    string  `json:"line,omitempty"`    // raw output line, shown in the details console
	Percent float64 `json:"percent"`           // -1 when unknown
	Message string  `json:"message,omitempty"` // human-readable status
}
