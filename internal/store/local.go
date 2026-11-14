package store

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/egoist/mygo"
)

// LocalPackage describes a package file on disk and what this machine can do
// with it. Produced by InspectLocalPackage (used by both the file picker and
// drag-and-drop) and consumed by the "Install file" view.
type LocalPackage struct {
	Path      string `json:"path"`
	FileName  string `json:"fileName"`
	Size      int64  `json:"size"`
	Kind      string `json:"kind"`     // deb | rpm | arch | apk | snap | flatpak | appimage
	KindName  string `json:"kindName"` // human-readable, e.g. "Debian package"
	Name      string `json:"name"`     // package name when known, else the file stem
	Version   string `json:"version"`
	Summary   string `json:"summary"`
	Command   string `json:"command"`  // exactly what will run, shown before install
	Action    string `json:"action"`   // one-line summary of what installing means
	Supported bool   `json:"supported"`
	Reason    string `json:"reason"`   // why it is unsupported here, and what to do instead
	NeedsAuth bool   `json:"needsAuth"`

	argv []string // the command after elevation (or directly, when not needed)
	env  []string // environment for elevated execution (e.g. DEBIAN_FRONTEND)
}

var localKinds = []struct {
	exts []string
	kind, name string
}{
	{[]string{".deb"}, "deb", "Debian package"},
	{[]string{".rpm"}, "rpm", "RPM package"},
	{[]string{".pkg.tar.zst", ".pkg.tar.xz", ".pkg.tar.gz", ".pkg.tar.zstd"}, "arch", "Arch Linux package"},
	{[]string{".apk"}, "apk", "Alpine package"},
	{[]string{".snap"}, "snap", "Snap package"},
	{[]string{".flatpak"}, "flatpak", "Flatpak bundle"},
	{[]string{".appimage"}, "appimage", "AppImage"},
}

// detectLocalKind classifies a filename; ok=false means unsupported file type.
func detectLocalKind(name string) (kind, kindName string, ok bool) {
	lower := strings.ToLower(name)
	for _, k := range localKinds {
		for _, ext := range k.exts {
			if strings.HasSuffix(lower, ext) {
				return k.kind, k.name, true
			}
		}
	}
	return "", "", false
}

// InspectLocalPackage reads what it can about a package file and decides
// whether this machine can install it, and how.
func (s *Store) InspectLocalPackage(ctx context.Context, path string) (LocalPackage, error) {
	if path == "" {
		return LocalPackage{}, fmt.Errorf("no file given")
	}
	path, err := filepath.Abs(path)
	if err != nil {
		return LocalPackage{}, err
	}
	fi, err := os.Stat(path)
	if err != nil {
		return LocalPackage{}, fmt.Errorf("could not read %q: %w", path, err)
	}
	if !fi.Mode().IsRegular() {
		return LocalPackage{}, fmt.Errorf("%q is not a regular file", path)
	}
	base := filepath.Base(path)
	kind, kindName, ok := detectLocalKind(base)
	if !ok {
		return LocalPackage{
			Path: path, FileName: base, Size: fi.Size(),
			Kind: "unknown", KindName: "Unknown file type",
			Name:   strings.TrimSuffix(base, filepath.Ext(base)),
			Reason: "U Store installs .deb, .rpm, .pkg.tar.*, .apk, .snap, .flatpak and .appimage files. For anything else, see the file's own install instructions.",
		}, nil
	}
	p := LocalPackage{Path: path, FileName: base, Size: fi.Size(), Kind: kind, KindName: kindName}
	readLocalMetadata(&p)
	if p.Name == "" {
		p.Name = strings.TrimSuffix(base, pathExtSuffix(base, kind))
	}
	s.planLocalInstall(&p)
	return p, nil
}

// PickLocalPackage opens the system's file picker (filtered to packages)
// and reports what was chosen. A canceled pick returns a zero LocalPackage.
func (s *Store) PickLocalPackage(ctx context.Context) (LocalPackage, error) {
	opts := mygo.OpenDialogOptions{
		Parent: mygo.CallerWindow(ctx),
		Title:  "Install a package file",
		Filters: []mygo.FileFilter{
			{Name: "Linux packages", Extensions: []string{"deb", "rpm", "apk", "snap", "flatpak", "appimage"}},
			{Name: "Arch packages (.pkg.tar.*)", Extensions: []string{"zst", "xz", "gz"}},
			{Name: "All files", Extensions: []string{"*"}},
		},
	}
	paths, err := mygo.Dialog.Open(opts)
	if err != nil {
		return LocalPackage{}, err
	}
	if len(paths) == 0 {
		return LocalPackage{}, nil
	}
	return s.InspectLocalPackage(ctx, paths[0])
}

// elevate wraps argv for elevated execution, when the desktop has polkit.
func elevate(argv []string, env ...string) []string {
	prefix := []string{}
	if !isRoot() {
		prefix = append(prefix, "pkexec")
	}
	prefix = append(prefix, "env")
	prefix = append(prefix, env...)
	return append(prefix, argv...)
}

// planLocalInstall fills Command/Action/Supported/Reason for this machine.
func (s *Store) planLocalInstall(p *LocalPackage) {
	family := s.distro.Family
	display := func(argv []string) string { return strings.Join(argv, " ") }

	switch p.Kind {
	case "deb":
		if family == "apt" {
			p.env = []string{"DEBIAN_FRONTEND=noninteractive"}
			p.argv = elevate([]string{"apt-get", "install", "-y", p.Path}, p.env...)
			p.Command = display(p.argv)
			p.Action = "Install with apt — dependencies are resolved from your repositories."
			p.Supported, p.NeedsAuth = true, true
			return
		}
		p.Reason = "This is a Debian-family package, but this system uses " + familyLabel(family) + ". Install the same app as a Flatpak, or ask the vendor for an " + familyLabel(family) + " package."
	case "rpm":
		if s.distro.Atomic {
			p.Reason = "This system is atomic (rpm-ostree): installing layered RPMs from the store needs a reboot and is out of scope. Use Flatpak, or run `rpm-ostree install <package>` in a terminal yourself."
			return
		}
		switch family {
		case "dnf":
			p.argv = elevate([]string{"dnf", "install", "-y", p.Path})
			p.Command = display(p.argv)
			p.Action = "Install with dnf — dependencies are resolved from your repositories."
			p.Supported, p.NeedsAuth = true, true
		case "zypper":
			p.argv = elevate([]string{"zypper", "--non-interactive", "install", p.Path})
			p.Command = display(p.argv)
			p.Action = "Install with zypper — dependencies are resolved from your repositories."
			p.Supported, p.NeedsAuth = true, true
		default:
			p.Reason = "This is an RPM package, but this system uses " + familyLabel(family) + ". Install the same app as a Flatpak, or ask the vendor for an " + familyLabel(family) + " package."
		}
	case "arch":
		if family == "pacman" {
			p.argv = elevate([]string{"pacman", "-U", "--needed", "--noconfirm", p.Path})
			p.Command = display(p.argv)
			p.Action = "Install with pacman from the package archive you chose."
			p.Supported, p.NeedsAuth = true, true
			return
		}
		p.Reason = "This is an Arch Linux package. This system uses " + familyLabel(family) + " — install the same app as a Flatpak instead."
	case "apk":
		if family == "apk" {
			p.argv = elevate([]string{"apk", "add", p.Path})
			p.Command = display(p.argv)
			p.Action = "Install with apk from the package you chose."
			p.Supported, p.NeedsAuth = true, true
			return
		}
		p.Reason = "This is an Alpine package. This system uses " + familyLabel(family) + ". (Android APKs are never supported on Linux desktops.)"
	case "snap":
		if s.registry.byID("snap") != nil {
			p.argv = elevate([]string{"snap", "install", "--dangerous", p.Path})
			p.Command = display(p.argv)
			p.Action = "Install with snapd. --dangerous accepts the local, unsigned snap you chose."
			p.Supported, p.NeedsAuth = true, true
			return
		}
		p.Reason = "Snap is not installed on this machine. Install snapd first, or look for this app as a Flatpak."
	case "flatpak":
		if s.registry.byID("flatpak") != nil {
			p.argv = []string{"flatpak", "install", "--user", "-y", p.Path}
			p.Command = display(p.argv)
			p.Action = "Install the bundle with Flatpak for your user — no administrator password needed."
			p.Supported, p.NeedsAuth = true, false
			return
		}
		p.Reason = "Flatpak is not installed on this machine. Install flatpak first (or use your distro's package manager)."
	case "appimage":
		p.Command = "cp '" + p.Path + "' ~/Applications/ && chmod 755 '~/Applications/" + p.FileName + "'"
		p.Action = "Copy into ~/Applications, mark it executable, and add a menu entry. No administrator password needed."
		p.Supported, p.NeedsAuth = true, false
		return
	}
}

func familyLabel(family string) string {
	switch family {
	case "apt":
		return "APT (Debian family)"
	case "dnf":
		return "dnf (Fedora family)"
	case "pacman":
		return "pacman (Arch family)"
	case "zypper":
		return "Zypper (openSUSE family)"
	case "apk":
		return "apk (Alpine)"
	default:
		return "no known package manager"
	}
}

func pathExtSuffix(base, kind string) string {
	lower := strings.ToLower(base)
	for _, k := range localKinds {
		if k.kind != kind {
			continue
		}
		for _, ext := range k.exts {
			if strings.HasSuffix(lower, ext) {
				return base[len(base)-len(ext):]
			}
		}
	}
	return filepath.Ext(base)
}

// readLocalMetadata enriches the descriptor with name/version/summary using
// whatever inspection tools exist on the machine. All failures are non-fatal:
// an unknown name never blocks installing.
func readLocalMetadata(p *LocalPackage) {
	switch p.Kind {
	case "deb":
		if hasBinary("dpkg-deb") {
			if out, err := queryCmd(context.Background(), 10*time.Second, "dpkg-deb", "-f", p.Path, "Package", "Version", "Description"); err == nil {
				lines := strings.SplitN(out, "\n", 3)
				if len(lines) >= 1 {
					p.Name = strings.TrimSpace(lines[0])
				}
				if len(lines) >= 2 {
					p.Version = strings.TrimSpace(lines[1])
				}
				if len(lines) >= 3 {
					p.Summary = strings.TrimSpace(lines[2])
				}
			}
		}
	case "rpm":
		if hasBinary("rpm") {
			if out, err := queryCmd(context.Background(), 10*time.Second, "rpm", "-qp", "--qf", "%{NAME}\t%{VERSION}-%{RELEASE}\t%{SUMMARY}\n", p.Path); err == nil {
				fields := strings.SplitN(strings.TrimSpace(out), "\t", 3)
				if len(fields) >= 1 {
					p.Name = fields[0]
				}
				if len(fields) >= 2 {
					p.Version = fields[1]
				}
				if len(fields) >= 3 {
					p.Summary = fields[2]
				}
			}
		}
	case "arch":
		stem := p.FileName
		for _, ext := range []string{".pkg.tar.zst", ".pkg.tar.xz", ".pkg.tar.gz", ".pkg.tar.zstd"} {
			if len(stem) > len(ext) && strings.EqualFold(stem[len(stem)-len(ext):], ext) {
				stem = stem[:len(stem)-len(ext)]
				break
			}
		}
		parts := strings.Split(stem, "-")
		if len(parts) >= 4 {
			p.Name = strings.Join(parts[:len(parts)-3], "-")
			p.Version = parts[len(parts)-3] + "-" + parts[len(parts)-2]
		}
	case "snap":
		stem := strings.TrimSuffix(p.FileName, ".snap")
		if i := strings.LastIndex(stem, "_"); i > 0 {
			p.Name = stem[:i]
			p.Version = stem[i+1:]
		} else {
			p.Name = stem
		}
	case "flatpak":
		p.Name = strings.TrimSuffix(p.FileName, ".flatpak")
	case "appimage":
		stem := p.FileName
		if i := strings.LastIndex(strings.ToLower(stem), ".appimage"); i > 0 {
			stem = stem[:i]
		}
		p.Name = stem
	case "apk":
		stem := strings.TrimSuffix(p.FileName, ".apk")
		parts := strings.Split(stem, "-")
		for i := len(parts) - 1; i >= 0; i-- {
			if len(parts[i]) > 1 && parts[i][0] == 'r' && parts[i][1] >= '0' && parts[i][1] <= '9' {
				if i >= 2 {
					p.Name = strings.Join(parts[:i-1], "-")
					p.Version = strings.Join(parts[i-1:i+1], "-")
				}
				break
			}
		}
	}
}

// InstallLocal installs or integrates a previously inspected package file.
func (s *Store) InstallLocal(ctx context.Context, path string, progress *mygo.Channel[InstallEvent]) error {
	p, err := s.InspectLocalPackage(ctx, path)
	if err != nil {
		return err
	}
	if !p.Supported {
		if p.Reason == "" {
			p.Reason = "This file cannot be installed on this system."
		}
		return fmt.Errorf("%s", p.Reason)
	}
	emit := func(ev InstallEvent) { _ = progress.Send(ev) }
	emit(InstallEvent{Type: "start", Message: "Installing " + p.FileName, Percent: -1})

	ctx, cancel := context.WithTimeout(ctx, 20*time.Minute)
	defer cancel()

	switch p.Kind {
	case "flatpak":
		// Installing a local bundle does not strictly need Flathub, but the
		// app itself may pull its runtime from there.
		_ = EnsureFlathub(ctx)
		if err := runLines(ctx, emit, p.argv[0], p.argv[1:]...); err != nil {
			emit(InstallEvent{Type: "error", Message: err.Error(), Percent: -1})
			return fmt.Errorf("flatpak install failed: %w", err)
		}
	case "appimage":
		if err := integrateAppImage(ctx, p.Path, emit); err != nil {
			emit(InstallEvent{Type: "error", Message: err.Error(), Percent: -1})
			return err
		}
	default:
		if err := runLines(ctx, emit, p.argv[0], p.argv[1:]...); err != nil {
			emit(InstallEvent{Type: "error", Message: err.Error(), Percent: -1})
			return fmt.Errorf("package install failed: %w", err)
		}
	}
	emit(InstallEvent{Type: "done", Message: "Installed.", Percent: 100})
	return nil
}

// integrateAppImage copies the AppImage into ~/Applications and adds a
// desktop entry so it appears in application menus.
func integrateAppImage(ctx context.Context, path string, emit func(InstallEvent)) error {
	home, err := os.UserHomeDir()
	if err != nil {
		return err
	}
	dir := filepath.Join(home, "Applications")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	base := filepath.Base(path)
	dst := filepath.Join(dir, base)
	emit(InstallEvent{Type: "line", Line: "Copying into ~/Applications…", Percent: -1})
	data, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("could not read the AppImage: %w", err)
	}
	if err := os.WriteFile(dst, data, 0o755); err != nil {
		return fmt.Errorf("could not write to ~/Applications: %w", err)
	}

	name := strings.TrimSuffix(base, filepath.Ext(base))
	if i := strings.LastIndex(strings.ToLower(name), ".appimage"); i > 0 {
		name = name[:i]
	}
	pretty := strings.NewReplacer("-", " ", "_", " ").Replace(name)
	slug := strings.ToLower(strings.NewReplacer(" ", "-", "_", "-", ".", "-").Replace(pretty))
	desktop := fmt.Sprintf(`[Desktop Entry]
Type=Application
Name=%s
Exec=%q
Icon=application-x-executable
Terminal=false
Categories=Utility;
`, pretty, dst)
	desktopDir := filepath.Join(home, ".local", "share", "applications")
	if err := os.MkdirAll(desktopDir, 0o755); err == nil {
		entry := filepath.Join(desktopDir, slug+".desktop")
		if err := os.WriteFile(entry, []byte(desktop), 0o644); err == nil {
			emit(InstallEvent{Type: "line", Line: "Added menu entry: " + entry, Percent: -1})
			if bin, err := exec.LookPath("update-desktop-database"); err == nil {
				_ = exec.CommandContext(ctx, bin, desktopDir).Run()
			}
		}
	}
	return nil
}
