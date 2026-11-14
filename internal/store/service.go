package store

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/egoist/mygo"
)

const installedCacheTTL = 20 * time.Second

// Store is the service bound to the frontend. Every exported method is
// callable from the page; mygo generate turns them into typed TS functions.
type Store struct {
	catalog  *Catalog
	distro   Distro
	registry *registry
	flathub  *Flathub
	theme    string

	mu             sync.Mutex
	installed      map[string]map[string]string // backend -> ref -> scope
	installedAt    time.Time
}

// New builds a Store, detecting the machine and loading the catalog.
func New() (*Store, error) {
	catalog, err := LoadCatalog()
	if err != nil {
		return nil, err
	}
	d := DetectDistro()
	return &Store{
		catalog:  catalog,
		distro:   d,
		registry: detectSources(d),
		flathub:  NewFlathub(),
		theme:    ThemeSystem, // replaced by New via LoadTheme when the app is ready
		installed: map[string]map[string]string{},
	}, nil
}

// --- system ---------------------------------------------------------------

// Info reports the distro, architecture and detected package backends.
func (s *Store) Info() SystemInfo {
	nativeID := ""
	for _, info := range s.registry.infos() {
		if info.Kind == "native" && info.Available {
			nativeID = info.ID
			break
		}
	}
	enabled := false
	if fp := s.registry.byID("flatpak"); fp != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
		enabled = len(flathubScopes(ctx)) > 0
		cancel()
	}
	return SystemInfo{
		DistroID:        s.distro.ID,
		DistroName:      s.distro.Name,
		PrettyName:      s.distro.Pretty,
		Arch:            s.distro.Arch,
		Backends:        s.registry.infos(),
		FlathubEnabled:  enabled,
		NativeBackendID: nativeID,
		Theme:           s.theme,
		Atomic:          s.distro.Atomic,
	}
}

// EnableFlathub adds the flathub remote (user scope) when it is missing.
func (s *Store) EnableFlathub() error {
	return EnsureFlathub(context.Background())
}

// --- installed-state cache -------------------------------------------------

func (s *Store) installedSets(ctx context.Context) map[string]map[string]string {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.installed != nil && time.Since(s.installedAt) < installedCacheTTL {
		return s.installed
	}
	sets := map[string]map[string]string{}
	for _, src := range s.registry.sources {
		refs, err := src.installedRefs(ctx)
		if err == nil {
			sets[src.info().ID] = refs
		} else {
			sets[src.info().ID] = map[string]string{}
		}
	}
	s.installed = sets
	s.installedAt = time.Now()
	return sets
}

func (s *Store) invalidateInstalled() {
	s.mu.Lock()
	s.installed = map[string]map[string]string{}
	s.installedAt = time.Time{}
	s.mu.Unlock()
}

// lookupInstalled reports whether the app is installed and where.
// id is the Flathub ID; native refs come from the catalog.
func (s *Store) lookupInstalled(sets map[string]map[string]string, id string) (bool, []InstalledSource) {
	var out []InstalledSource
	if refs, ok := sets["flatpak"]; ok {
		if scope, ok := refs[id]; ok {
			out = append(out, InstalledSource{Backend: "flatpak", Ref: id, Scope: scope})
		}
	}
	for _, native := range nativeBackendIDs {
		ref := s.catalog.NativePackage(id, native)
		if ref == "" {
			continue
		}
		if refs, ok := sets[native]; ok {
			if _, ok := refs[ref]; ok {
				out = append(out, InstalledSource{Backend: native, Ref: ref})
			}
		}
	}
	if ref := s.catalog.NativePackage(id, "snap"); ref != "" {
		if refs, ok := sets["snap"]; ok {
			if _, ok := refs[ref]; ok {
				out = append(out, InstalledSource{Backend: "snap", Ref: ref})
			}
		}
	}
	return len(out) > 0, out
}

var nativeBackendIDs = []string{"apt", "dnf", "pacman", "zypper", "apk"}

// sourcesFor lists backends an app can be installed from, flatpak first.
func (s *Store) sourcesFor(id string) []string {
	var out []string
	if s.registry.byID("flatpak") != nil {
		out = append(out, "flatpak")
	}
	for _, native := range nativeBackendIDs {
		if s.registry.byID(native) != nil && s.catalog.NativePackage(id, native) != "" {
			out = append(out, native)
			break // only this machine's native family
		}
	}
	if s.registry.byID("snap") != nil && s.catalog.NativePackage(id, "snap") != "" {
		out = append(out, "snap")
	}
	return out
}

func (s *Store) preferredBackend(id string) string {
	srcs := s.sourcesFor(id)
	if len(srcs) > 0 {
		return srcs[0]
	}
	return "flatpak"
}

func (s *Store) nativeRef(id, backend string) string {
	return s.catalog.NativePackage(id, backend)
}

// summary builds an AppSummary for a curated catalog entry.
func (s *Store) summary(app catalogApp, sets map[string]map[string]string) AppSummary {
	installed, _ := s.lookupInstalled(sets, app.ID)
	nativePkg := ""
	for _, native := range nativeBackendIDs {
		if ref := s.catalog.NativePackage(app.ID, native); ref != "" && s.registry.byID(native) != nil {
			nativePkg = ref
			break
		}
	}
	srcs := s.sourcesFor(app.ID)
	src := "flatpak"
	if len(srcs) > 0 {
		src = srcs[0]
	}
	return AppSummary{
		ID:            app.ID,
		Name:          app.Name,
		Summary:       app.Summary,
		Icon:          iconFor(app.ID),
		Categories:    app.Categories,
		Developer:     app.Developer,
		Verified:      app.Verified,
		Source:        src,
		Sources:       srcs,
		NativePackage: nativePkg,
		Installed:     installed,
		Featured:      app.Featured,
	}
}

// flathubSummary builds an AppSummary from a live Flathub hit.
func (s *Store) flathubSummary(hit flathubHit, sets map[string]map[string]string) AppSummary {
	installed, _ := s.lookupInstalled(sets, hit.AppID)
	id := hit.AppID
	name := hit.Name
	if app := s.catalog.Find(id); app != nil && app.Name != "" {
		name = app.Name // curated spelling wins
	}
	cats := []string{}
	for _, c := range hit.MainCategories {
		if c != "" {
			cats = append(cats, strings.ToLower(c))
		}
	}
	nativePkg := ""
	for _, native := range nativeBackendIDs {
		if ref := s.catalog.NativePackage(id, native); ref != "" && s.registry.byID(native) != nil {
			nativePkg = ref
			break
		}
	}
	return AppSummary{
		ID:            id,
		Name:          name,
		Summary:       hit.Summary,
		Icon:          iconOrDefault(hit.Icon, id),
		Categories:    cats,
		Developer:     hit.DeveloperName,
		Verified:      hit.VerificationVerified,
		Source:        "flatpak",
		Sources:       []string{"flatpak"},
		NativePackage: nativePkg,
		Installed:     installed,
	}
}

// --- discovery & search ------------------------------------------------------

// Discover returns the curated featured picks plus the live trending list.
// When Flathub is unreachable the view still renders from the catalog.
func (s *Store) Discover(ctx context.Context) (DiscoverResult, error) {
	sets := s.installedSets(ctx)
	featured := []AppSummary{}
	for _, app := range s.catalog.Apps {
		if app.Featured {
			featured = append(featured, s.summary(app, sets))
		}
	}
	trending := []AppSummary{}
	fromFlathub := false
	hits, _, err := s.flathub.Search(ctx, "", 1, 18)
	if err == nil {
		fromFlathub = true
		for _, hit := range hits {
			trending = append(trending, s.flathubSummary(hit, sets))
		}
	} else {
		// offline: featured fill the trending rail rather than an empty page
		for _, app := range s.catalog.Apps {
			if len(trending) >= 12 {
				break
			}
			trending = append(trending, s.summary(app, sets))
		}
	}
	return DiscoverResult{Trending: trending, Featured: featured, Categories: s.catalog.Categories, FromFlathub: fromFlathub}, nil
}

// Search queries Flathub live, falling back to the embedded catalog offline.
func (s *Store) Search(ctx context.Context, query string) (SearchResult, error) {
	query = strings.TrimSpace(query)
	result := SearchResult{Query: query}
	if query == "" {
		return result, nil
	}
	sets := s.installedSets(ctx)
	hits, total, err := s.flathub.Search(ctx, query, 1, 48)
	if err != nil {
		for _, app := range s.catalog.Search(query) {
			result.Hits = append(result.Hits, s.summary(app, sets))
		}
		result.Total = len(result.Hits)
		result.FromFlathub = false
		return result, nil
	}
	for _, hit := range hits {
		result.Hits = append(result.Hits, s.flathubSummary(hit, sets))
	}
	result.Total = total
	result.FromFlathub = true
	return result, nil
}

// AppDetails resolves one app, combining curated data with live metadata.
func (s *Store) AppDetails(ctx context.Context, id string) (AppDetails, error) {
	if !safeRef(id) {
		return AppDetails{}, fmt.Errorf("invalid app id %q", id)
	}
	sets := s.installedSets(ctx)
	details := AppDetails{AppSummary: AppSummary{ID: id, Icon: iconFor(id), Source: "flatpak", Sources: s.sourcesFor(id)}}
	installed, _ := s.lookupInstalled(sets, id)
	details.Installed = installed

	fromFlathub := false
	if as, err := s.flathub.Appstream(ctx, id); err == nil {
		fromFlathub = true
		details.Name = as.Name
		details.Summary = as.Summary
		details.Description = as.Description
		details.License = as.ProjectLicense
		details.Developer = as.DeveloperName
		details.Icon = iconOrDefault(as.Icon, id)
		details.Verified = verifiedFromMetadata(as.Metadata)
		details.Homepage = as.Urls.Homepage
		details.Categories = as.Categories
		if len(as.Releases) > 0 {
			var ts int64
			fmt.Sscanf(as.Releases[0].Timestamp, "%d", &ts)
			details.UpdatedAt = ts
		}
		details.Screenshots = screenshotsFromAppstream(as, 4)
	}
	if app := s.catalog.Find(id); app != nil {
		if details.Name == "" {
			details.Name = app.Name
			details.Summary = app.Summary
			details.Developer = app.Developer
			details.Verified = app.Verified
			details.Icon = iconFor(id)
			details.Categories = app.Categories
		}
		for _, native := range nativeBackendIDs {
			if ref := s.catalog.NativePackage(id, native); ref != "" {
				details.NativePackage = ref
				break
			}
		}
	}
	if details.Name == "" {
		// last resort: the ID itself, so the page can still render
		details.Name = id
		details.Summary = "Details unavailable — check your network connection."
	}
	if fromFlathub {
		details.Origin = "flathub"
	} else {
		details.Origin = "catalog"
	}
	details.Source = s.preferredBackend(id)
	return details, nil
}

// --- install / remove / update ---------------------------------------------

func resolveRef(s *Store, id, backend string) (string, error) {
	switch backend {
	case "flatpak", "":
		return id, nil
	case "snap":
		return s.localInstallRef(id, "snap"), nil
	default:
		return s.localInstallRef(id, backend), nil
	}
}

// Install installs an app from the chosen backend (default: preferred),
// streaming progress to the page.
func (s *Store) Install(ctx context.Context, id, backend string, progress *mygo.Channel[InstallEvent]) error {
	if !safeRef(id) {
		return fmt.Errorf("invalid app id %q", id)
	}
	if backend == "" {
		backend = s.preferredBackend(id)
	}
	src := s.registry.byID(backend)
	if src == nil {
		return fmt.Errorf("the %s backend is not available on this machine", backend)
	}
	ref, err := resolveRef(s, id, backend)
	if err != nil {
		return err
	}
	emit := func(ev InstallEvent) {
		if ev.Percent == 0 {
			ev.Percent = -1
		}
		_ = progress.Send(ev)
	}
	emit(InstallEvent{Type: "start", Message: "Installing " + id, Percent: -1})
	if err := src.install(ctx, ref, emit); err != nil {
		emit(InstallEvent{Type: "error", Message: err.Error(), Percent: -1})
		return err
	}
	s.invalidateInstalled()
	emit(InstallEvent{Type: "done", Message: "Installed.", Percent: 100})
	return nil
}

// Remove uninstalls an app. Flatpak scope is discovered automatically.
func (s *Store) Remove(ctx context.Context, id, backend string, progress *mygo.Channel[InstallEvent]) error {
	if !safeRef(id) {
		return fmt.Errorf("invalid app id %q", id)
	}
	if backend == "" {
		backend = s.preferredBackend(id)
	}
	src := s.registry.byID(backend)
	if src == nil {
		return fmt.Errorf("the %s backend is not available on this machine", backend)
	}
	ref, err := resolveRef(s, id, backend)
	if err != nil {
		return err
	}
	scope := ""
	if sets := s.installedSets(ctx); sets[backend] != nil {
		scope = sets[backend][ref]
	}
	emit := func(ev InstallEvent) { _ = progress.Send(ev) }
	emit(InstallEvent{Type: "start", Message: "Removing…", Percent: -1})
	if err := src.remove(ctx, ref, scope, emit); err != nil {
		emit(InstallEvent{Type: "error", Message: err.Error(), Percent: -1})
		return err
	}
	s.invalidateInstalled()
	emit(InstallEvent{Type: "done", Message: "Removed.", Percent: 100})
	return nil
}

// Launch starts an installed app.
func (s *Store) Launch(ctx context.Context, id, backend string) error {
	if !safeRef(id) {
		return fmt.Errorf("invalid app id %q", id)
	}
	if backend == "" {
		backend = s.preferredBackend(id)
	}
	src := s.registry.byID(backend)
	if src == nil {
		return fmt.Errorf("the %s backend is not available on this machine", backend)
	}
	ref, err := resolveRef(s, id, backend)
	if err != nil {
		return err
	}
	scope := ""
	if backend == "flatpak" {
		if sets := s.installedSets(ctx); sets[backend] != nil {
			scope = sets[backend][ref]
		}
	}
	return src.launch(ctx, ref, scope)
}

// Installed lists everything the store knows about on this machine.
func (s *Store) Installed(ctx context.Context) ([]InstalledApp, error) {
	sets := s.installedSets(ctx)
	seen := map[string]bool{}
	var out []InstalledApp
	add := func(id string, srcs []InstalledSource) {
		if seen[id] {
			return
		}
		seen[id] = true
		var sum AppSummary
		if app := s.catalog.Find(id); app != nil {
			sum = s.summary(*app, sets)
		} else {
			sum = AppSummary{ID: id, Name: id, Summary: "", Icon: iconFor(id), Source: srcs[0].Backend, Sources: []string{srcs[0].Backend}, Installed: true}
		}
		out = append(out, InstalledApp{AppSummary: sum, Sources: srcs})
	}
	// flatpak apps by their flathub id
	if refs := sets["flatpak"]; refs != nil {
		for ref, scope := range refs {
			add(ref, []InstalledSource{{Backend: "flatpak", Ref: ref, Scope: scope}})
		}
	}
	// native/snap apps from the catalog
	for _, app := range s.catalog.Apps {
		installed, srcs := s.lookupInstalled(sets, app.ID)
		if !installed {
			continue
		}
		// drop the flatpak source we already handled above; keep native/snap
		var native []InstalledSource
		for _, src := range srcs {
			if src.Backend != "flatpak" {
				native = append(native, src)
			}
		}
		if len(native) == 0 {
			continue
		}
		add(app.ID, native)
	}
	sort.Slice(out, func(i, j int) bool { return strings.ToLower(out[i].Name) < strings.ToLower(out[j].Name) })
	return out, nil
}

// Updates lists pending updates across every available backend.
func (s *Store) Updates(ctx context.Context) ([]UpdateInfo, error) {
	var out []UpdateInfo
	for _, src := range s.registry.sources {
		u, err := src.updates(ctx)
		if err != nil {
			continue
		}
		out = append(out, u...)
	}
	// enrich from the catalog: match by ref or by native-package mapping
	byNative := map[string]string{}
	for _, app := range s.catalog.Apps {
		for _, native := range nativeBackendIDs {
			if ref := app.Native[native]; ref != "" {
				byNative[native+"/"+ref] = app.ID
			}
		}
		if ref := app.Native["snap"]; ref != "" {
			byNative["snap/"+ref] = app.ID
		}
	}
	for i := range out {
		if app := s.catalog.Find(out[i].Ref); app != nil {
			out[i].Name = app.Name
			out[i].Icon = iconFor(app.ID)
			continue
		}
		if id, ok := byNative[out[i].Backend+"/"+out[i].Ref]; ok {
			out[i].ID = id
			out[i].Name = s.catalog.Find(id).Name
			out[i].Icon = iconFor(id)
			continue
		}
		out[i].ID = out[i].Ref
		out[i].Icon = ""
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Backend != out[j].Backend {
			return out[i].Backend < out[j].Backend
		}
		return out[i].Name < out[j].Name
	})
	return out, nil
}

// UpdateAll upgrades everything this store can see, streaming progress.
func (s *Store) UpdateAll(ctx context.Context, progress *mygo.Channel[InstallEvent]) error {
	emit := func(ev InstallEvent) { _ = progress.Send(ev) }
	emit(InstallEvent{Type: "start", Message: "Checking for updates…", Percent: -1})
	updates, _ := s.Updates(ctx)
	for _, src := range s.registry.sources {
		var refs []UpdateInfo
		for _, u := range updates {
			if u.Backend == src.info().ID {
				refs = append(refs, u)
			}
		}
		if err := src.applyUpdates(ctx, refs, emit); err != nil {
			emit(InstallEvent{Type: "error", Message: err.Error(), Percent: -1})
			return err
		}
	}
	s.invalidateInstalled()
	emit(InstallEvent{Type: "done", Message: "Everything is up to date.", Percent: 100})
	return nil
}
