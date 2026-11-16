package store

import (
	"context"
	"fmt"
	"strings"
	"time"
)

// nativeDetails is one package's metadata from this machine's repositories,
// shaped exactly like AppDetails so native hits render in the same detail view.
func (s *Store) nativeDetails(ctx context.Context, backend, ref string) (AppDetails, error) {
	switch backend {
	case "dnf":
		return dnfNativeDetails(ctx, ref)
	case "apt":
		return aptNativeDetails(ctx, ref)
	case "pacman":
		return pacmanNativeDetails(ctx, ref)
	case "zypper":
		return zypperNativeDetails(ctx, ref)
	case "apk":
		return apkNativeDetails(ctx, ref)
	default:
		return AppDetails{}, fmt.Errorf("no details support for %q", backend)
	}
}

// NativeDetails returns metadata for a repo package, shaped like AppDetails.
// Installed state comes from the same detection as everything else.
func (s *Store) NativeDetails(ctx context.Context, backend, ref string) (AppDetails, error) {
	if backend == "" || ref == "" {
		return AppDetails{}, fmt.Errorf("a backend and a package name are required")
	}
	if !safeRef(ref) {
		return AppDetails{}, fmt.Errorf("invalid package name %q", ref)
	}
	sets := s.installedSets(ctx)
	details, err := s.nativeDetails(ctx, backend, ref)
	if err != nil {
		// Still render something useful even if the repo query fails.
		installed := false
		if m, ok := sets[backend]; ok {
			_, installed = m[ref]
		}
		details = AppDetails{
			AppSummary: AppSummary{
				ID:            ref,
				Name:          ref,
				Summary:       fmt.Sprintf("Package from %s (details unavailable offline)", backend),
				Icon:          s.nativeIcon(ctx, backend, ref, installed),
				Source:        backend,
				Sources:       []string{backend},
				NativePackage: ref,
				Installed:     installed,
			},
			Origin:      "native",
			Description: `<p>Package <code>` + ref + `</code> from your system's repositories.</p>`,
		}
		return details, nil
	}
	var installed bool
	if m, ok := sets[backend]; ok {
		_, installed = m[ref]
	}
	details.Installed = installed
	details.Icon = s.nativeIcon(ctx, backend, ref, installed)

	details.Sources = []string{backend}
	details.Source = backend
	details.NativePackage = ref
	return details, nil
}

// nativeIcon picks the best icon for a repo package: catalog's Flathub icon,
// then the desktop-icon protocol for installed packages, else "" (letter tile).
func (s *Store) nativeIcon(ctx context.Context, backend, ref string, installed bool) string {
	if id := s.catalog.IDForNativeRef(backend, ref); id != "" {
		return iconFor(id)
	}
	if installed {
		if path := PackageIconPath(ctx, ref); path != "" {
			return "us-icon://localhost/" + ref
		}
	}
	return ""
}

func dnfNativeDetails(ctx context.Context, ref string) (AppDetails, error) {
	out, err := queryCmd(ctx, 30*time.Second, "dnf", "repoquery", "--quiet",
		"--qf", "%{name}|%{version}-%{release}|%{license}|%{summary}|%{repository}|%{url}\n", ref)
	if err != nil {
		return AppDetails{}, err
	}
	line := strings.TrimSpace(strings.SplitN(out, "\n", 2)[0])
	parts := strings.SplitN(line, "|", 6)
	name, version, license, summary, repo, url := ref, "", "", "", "", ""
	if len(parts) > 0 {
		name = parts[0]
	}
	if len(parts) > 1 {
		version = parts[1]
	}
	if len(parts) > 2 {
		license = parts[2]
	}
	if len(parts) > 3 {
		summary = parts[3]
	}
	if len(parts) > 4 {
		repo = parts[4]
	}
	if len(parts) > 5 {
		url = strings.TrimSpace(parts[5])
	}
	return nativeDetailsShell(name, backendDNF, ref, version, license, summary, repo, url), nil
}

const backendDNF = "dnf"

func aptNativeDetails(ctx context.Context, ref string) (AppDetails, error) {
	out, err := queryCmd(ctx, 30*time.Second, "apt-cache", "show", ref)
	if err != nil {
		return AppDetails{}, err
	}
	fields := map[string]string{}
	for _, line := range strings.Split(out, "\n") {
		key, value, ok := strings.Cut(line, ":")
		if ok {
			key = strings.TrimSpace(key)
			value = strings.TrimSpace(value)
			fields[key] = value
		}
	}
	name := fields["Package"]
	if name == "" {
		name = ref
	}
	version := fields["Version"]
	summary := fields["Description"]
	if summary == "" {
		summary = fields["Description-en"]
	}
	license := fields["License"]
	url := fields["Homepage"]
	return nativeDetailsShell(name, "apt", ref, version, license, summary, "", url), nil
}

func pacmanNativeDetails(ctx context.Context, ref string) (AppDetails, error) {
	out, err := queryCmd(ctx, 30*time.Second, "pacman", "-Si", ref)
	if err != nil {
		// fall back to local package query (package not in repos but installed)
		out, err = queryCmd(ctx, 30*time.Second, "pacman", "-Qi", ref)
		if err != nil {
			return AppDetails{}, err
		}
	}
	fields := pacmanFields(out)
	name := fields["name"]
	if name == "" {
		name = ref
	}
	version := fields["Version"]
	license := fields["Licenses"]
	summary := fields["Description"]
	repo := fields["Repository"] // -Si prints it under Repository
	if repo == "" {
		repo = fields["Repository"] // may be elsewhere on -Qi
	}
	url := fields["URL"]
	return nativeDetailsShell(name, "pacman", ref, version, license, summary, repo, url), nil
}

func pacmanFields(out string) map[string]string {
	fields := map[string]string{}
	for _, line := range strings.Split(out, "\n") {
		line = strings.TrimSpace(line)
		if !strings.Contains(line, ":") {
			continue
		}
		key, value, _ := strings.Cut(line, ":")
		fields[strings.TrimSpace(key)] = strings.TrimSpace(value)
	}
	return fields
}

func zypperNativeDetails(ctx context.Context, ref string) (AppDetails, error) {
	out, err := queryCmd(ctx, 45*time.Second, "zypper", "--non-interactive", "info", "--type", "package", ref)
	if err != nil {
		return AppDetails{}, err
	}
	fields := map[string]string{}
	for _, line := range strings.Split(out, "\n") {
		key, value, ok := strings.Cut(line, ":")
		if ok {
			fields[strings.TrimSpace(key)] = strings.TrimSpace(value)
		}
	}
	return nativeDetailsShell(ref, "zypper", ref, fields["Version"], fields["License"], fields["Summary"], fields["Repository"], fields["Homepage"]), nil
}

func apkNativeDetails(ctx context.Context, ref string) (AppDetails, error) {
	// apk gives version/license/size but not a nice summary; read what we can.
	out, err := queryCmd(ctx, 20*time.Second, "apk", "info", "-L", ref)
	if err != nil {
		out, err = queryCmd(ctx, 20*time.Second, "apk", "search", "-x", ref)
		if err != nil {
			return AppDetails{}, err
		}
	}
	first := strings.Fields(out)
	name, version := ref, ""
	if len(first) > 0 {
		if i := strings.LastIndex(first[0], "-"); i > 0 {
			name = first[0][:i]
			version = first[0][i+1:]
		}
	}
	return nativeDetailsShell(name, "apk", ref, version, "", "", "", ""), nil
}

// nativeDetailsShell builds the shared AppDetails shape for native packages.
func nativeDetailsShell(name, backend, ref, version, license, summary, repo, url string) AppDetails {
	hdr := "Package from your system's repositories"
	if repo != "" {
		hdr = "Package from the " + repo + " repository"
	}
	var desc strings.Builder
	fmt.Fprintf(&desc, "<p>%s.</p><ul>", hdr)
	fmt.Fprintf(&desc, "<li><strong>Package</strong>: <code>%s</code></li>", name)
	if version != "" {
		fmt.Fprintf(&desc, "<li><strong>Version</strong>: %s</li>", version)
	}
	if license != "" {
		fmt.Fprintf(&desc, "<li><strong>License</strong>: %s</li>", license)
	}
	if repo != "" {
		fmt.Fprintf(&desc, "<li><strong>Repository</strong>: %s</li>", repo)
	}
	desc.WriteString("</ul>")
	p := AppDetails{
		AppSummary: AppSummary{
			ID:            ref,
			Name:          name,
			Summary:       summary,
			Icon:          "",
			Categories:    []string{"utility"},
			Developer:     backend,
			Verified:      false,
			Source:        backend,
			Sources:       []string{backend},
			NativePackage: ref,
			Installed:     false,
		},
		Description: desc.String(),
		License:     license,
		Homepage:    url,
		Origin:      "native",
	}
	return p
}
