package store

import (
	"context"
	"fmt"
	"strings"
	"time"
)

// LocalHit is one package available in the machine's own repositories.
// The store searches these alongside Flathub so anything your distro ships
// is installable too — without a curated catalog entry.
type LocalHit struct {
	Name      string `json:"name"`
	Summary   string `json:"summary"`
	Backend   string `json:"backend"`
	Installed bool   `json:"installed"`
}

// localSearcher is implemented by sources that can query the local repos.
type localSearcher interface {
	localSearch(ctx context.Context, query string) ([]LocalHit, error)
}

// sanitizeSearchToken strips anything that could be a glob/regex metacharacter.
func sanitizeSearchToken(q string) string {
	var b strings.Builder
	for _, r := range q {
		ok := r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' || r >= '0' && r <= '9' ||
			r == '+' || r == '.' || r == '-' || r == '_'
		if ok {
			b.WriteRune(r)
		}
	}
	return strings.TrimSpace(b.String())
}

const maxLocalHits = 50

func capLocalHits(hits []LocalHit) []LocalHit {
	if len(hits) > maxLocalHits {
		return hits[:maxLocalHits]
	}
	return hits
}

// splitRepoQueryLines parses "name\tsummary" lines from repo tools.
func parseTabLines(out string) [][2]string {
	var rows [][2]string
	for _, line := range strings.Split(out, "\n") {
		line = strings.TrimSpace(line)
		if line == "" || !strings.Contains(line, "\t") {
			continue
		}
		name, summary, _ := strings.Cut(line, "\t")
		name = strings.TrimSpace(name)
		summary = strings.TrimSpace(summary)
		if name == "" || strings.ContainsAny(name, " \t|") {
			continue
		}
		rows = append(rows, [2]string{name, summary})
	}
	return rows
}

// --- dnf (incl. yum) ---

func (d dnfSource) localSearch(ctx context.Context, query string) ([]LocalHit, error) {
	token := sanitizeSearchToken(query)
	if token == "" {
		return nil, nil
	}
	out, err := queryCmd(ctx, 120*time.Second, d.bin(), "repoquery", "--quiet",
		"--qf", "%{name}\t%{summary}\n", "*"+token+"*")
	if err != nil {
		return nil, err
	}
	sets := map[string]string{}
	if installed, err := d.installedRefs(ctx); err == nil {
		sets = installed
	}
	var hits []LocalHit
	for _, row := range parseTabLines(out) {
		_, installed := sets[row[0]]
		hits = append(hits, LocalHit{Name: row[0], Summary: row[1], Backend: "dnf", Installed: installed})
	}
	return capLocalHits(hits), nil
}

// --- apt ---

func (aptSource) localSearch(ctx context.Context, query string) ([]LocalHit, error) {
	token := sanitizeSearchToken(query)
	if token == "" {
		return nil, nil
	}
	out, err := queryCmd(ctx, 45*time.Second, "apt-cache", "search", token)
	if err != nil {
		return nil, err
	}
	sets := map[string]string{}
	installed, err := aptSource{}.installedRefs(ctx)
	if err == nil {
		sets = installed
	}
	var hits []LocalHit
	for _, line := range strings.Split(out, "\n") {
		name, summary, ok := strings.Cut(line, " - ")
		name = strings.TrimSpace(name)
		if !ok || name == "" {
			continue
		}
		if strings.ContainsAny(name, " \t") {
			continue
		}
		_, installed := sets[name]
		hits = append(hits, LocalHit{Name: name, Summary: strings.TrimSpace(summary), Backend: "apt", Installed: installed})
	}
	return capLocalHits(hits), nil
}

// --- pacman ---

func (pacmanSource) localSearch(ctx context.Context, query string) ([]LocalHit, error) {
	token := sanitizeSearchToken(query)
	if token == "" {
		return nil, nil
	}
	out, err := queryCmd(ctx, 45*time.Second, "pacman", "-Ss", token)
	if err != nil {
		return nil, err
	}
	sets := map[string]string{}
	installed, err := pacmanSource{}.installedRefs(ctx)
	if err == nil {
		sets = installed
	}
	var hits []LocalHit
	lines := strings.Split(out, "\n")
	for i := 0; i < len(lines); i++ {
		line := lines[i]
		// "extra/firefox 131.0-1 (installed)"
		fields := strings.Fields(line)
		if len(fields) < 2 || !strings.Contains(fields[0], "/") {
			continue
		}
		name := strings.SplitN(fields[0], "/", 2)[1]
		_, installed := sets[name]
		summary := ""
		if i+1 < len(lines) {
			next := strings.TrimSpace(lines[i+1])
			if next != "" && !strings.Contains(next, "/") {
				summary = next
				i++
			}
		}
		hits = append(hits, LocalHit{Name: name, Summary: summary, Backend: "pacman", Installed: installed})
	}
	return capLocalHits(hits), nil
}

// --- zypper (openSUSE), best effort ---

func (zypperSource) localSearch(ctx context.Context, query string) ([]LocalHit, error) {
	token := sanitizeSearchToken(query)
	if token == "" {
		return nil, nil
	}
	out, err := queryCmd(ctx, 60*time.Second, "zypper", "--non-interactive", "search", "--match-substrings", token)
	if err != nil {
		return nil, err
	}
	sets := map[string]string{}
	installed, err := zypperSource{}.installedRefs(ctx)
	if err == nil {
		sets = installed
	}
	var hits []LocalHit
	for _, line := range strings.Split(out, "\n") {
		fields := strings.Split(line, "|")
		if len(fields) < 3 {
			continue
		}
		name := strings.TrimSpace(fields[1])
		if name == "" || name == "Name" || strings.ContainsAny(name, " ") {
			continue
		}
		summary := strings.TrimSpace(fields[len(fields)-1])
		if summary == name {
			summary = ""
		}
		_, installed := sets[name]
		hits = append(hits, LocalHit{Name: name, Summary: summary, Backend: "zypper", Installed: installed})
	}
	return capLocalHits(hits), nil
}

// --- apk (Alpine), best effort ---

func (apkSource) localSearch(ctx context.Context, query string) ([]LocalHit, error) {
	token := sanitizeSearchToken(query)
	if token == "" {
		return nil, nil
	}
	out, err := queryCmd(ctx, 30*time.Second, "apk", "search", "-q", token)
	if err != nil {
		return nil, err
	}
	sets := map[string]string{}
	installed, err := apkSource{}.installedRefs(ctx)
	if err == nil {
		sets = installed
	}
	var hits []LocalHit
	for _, line := range strings.Fields(out) {
		name := line
		// strip the -<version>-rN suffix
		if i := strings.LastIndex(name, "-"); i > 0 {
			name = name[:i]
		}
		if i := strings.LastIndex(name, "-"); i > 0 {
			name = name[:i]
		}
		_, installed := sets[name]
		hits = append(hits, LocalHit{Name: name, Summary: "An Alpine package", Backend: "apk", Installed: installed})
	}
	return capLocalHits(hits), nil
}

// --- snap ---

func (snapSource) localSearch(ctx context.Context, query string) ([]LocalHit, error) {
	token := sanitizeSearchToken(query)
	if token == "" {
		return nil, nil
	}
	out, err := queryCmd(ctx, 45*time.Second, "snap", "find", token)
	if err != nil {
		return nil, err
	}
	sets := map[string]string{}
	installed, err := snapSource{}.installedRefs(ctx)
	if err == nil {
		sets = installed
	}
	var hits []LocalHit
	for i, line := range strings.Split(out, "\n") {
		if i == 0 {
			continue // header
		}
		fields := strings.Fields(line)
		if len(fields) < 2 {
			continue
		}
		name := fields[0]
		if strings.HasPrefix(name, "-") {
			continue
		}
		_, installed := sets[name]
		hits = append(hits, LocalHit{Name: name, Summary: strings.Join(fields[2:], " "), Backend: "snap", Installed: installed})
	}
	return capLocalHits(hits), nil
}

// LocalSearchResult carries both what worked and what didn't, so the UI can
// show hits from one repo beside an explanation of another's failure.
type LocalSearchResult struct {
	Hits   []LocalHit `json:"hits"`
	Errors []string   `json:"errors"`
}

// SearchLocal queries the machine's own repositories for packages matching
// the query. Backend failures are reported inside the result, never thrown:
// a slow or unreachable repo must not blank the ones that answered.
func (s *Store) SearchLocal(ctx context.Context, query string) LocalSearchResult {
	var result LocalSearchResult
	query = strings.TrimSpace(query)
	if query == "" {
		return result
	}
	var out []LocalHit
	for _, src := range s.registry.sources {
		ls, ok := src.(localSearcher)
		if !ok {
			continue
		}
		hits, err := ls.localSearch(ctx, query)
		if err != nil {
			result.Errors = append(result.Errors, fmt.Sprintf("%s: %v", src.info().Name, err))
			continue
		}
		out = append(out, hits...)
	}
	seen := map[string]bool{}
	for _, h := range out {
		if seen[h.Backend+"/"+h.Name] {
			continue
		}
		seen[h.Backend+"/"+h.Name] = true
		result.Hits = append(result.Hits, h)
	}
	if len(result.Hits) > maxLocalHits {
		result.Hits = result.Hits[:maxLocalHits]
	}
	return result
}

// localInstallRef resolves an install ref for a package that may not be in
// the curated catalog: catalog mapping first, then the raw name itself,
// so native hits install without needing a catalog entry.
func (s *Store) localInstallRef(id, backend string) string {
	if s.catalog.NativePackage(id, backend) != "" {
		return s.catalog.NativePackage(id, backend)
	}
	return id
}
