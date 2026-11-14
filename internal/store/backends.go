package store

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"
	"time"
)

// Source is one way of installing apps on this machine. Implementations are
// small and stateless; to add a package manager, add a Source here and one
// line in detectSources.
type Source interface {
	info() BackendInfo
	// installedRefs maps ref -> scope ("user"/"system" for flatpak, "" otherwise).
	installedRefs(ctx context.Context) (map[string]string, error)
	install(ctx context.Context, ref string, emit func(InstallEvent)) error
	remove(ctx context.Context, ref, scope string, emit func(InstallEvent)) error
	// updates lists pending updates; ref/backend/scope identify them for UpdateAll.
	updates(ctx context.Context) ([]UpdateInfo, error)
	// applyUpdates upgrades the given refs (or everything when refs is empty).
	applyUpdates(ctx context.Context, refs []UpdateInfo, emit func(InstallEvent)) error
	// launch starts an installed app.
	launch(ctx context.Context, ref, scope string) error
}

// registry pairs a Source with how it was detected.
type registry struct {
	sources  []Source
	nativeID string
}

// detectSources probes the machine once at startup.
func detectSources(d Distro) *registry {
	r := &registry{nativeID: d.Family}
	var srcs []Source
	if hasBinary("flatpak") {
		srcs = append(srcs, newFlatpakSource())
	}
	var native Source
	switch d.Family {
	case "apt":
		if hasBinary("apt-get") || hasBinary("apt") {
			native = &aptSource{}
		}
	case "dnf":
		if hasBinary("dnf") {
			native = &dnfSource{}
		} else if hasBinary("yum") {
			native = &dnfSource{yum: true}
		}
	case "pacman":
		if hasBinary("pacman") {
			native = &pacmanSource{}
		}
	case "zypper":
		if hasBinary("zypper") {
			native = &zypperSource{}
		}
	case "apk":
		if hasBinary("apk") {
			native = &apkSource{}
		}
	}
	if native != nil {
		srcs = append(srcs, native)
	}
	if hasBinary("snap") {
		srcs = append(srcs, &snapSource{})
	}
	r.sources = srcs
	return r
}

// byID returns the source with the given id, or nil.
func (r *registry) byID(id string) Source {
	for _, s := range r.sources {
		if s.info().ID == id {
			return s
		}
	}
	return nil
}

// infos reports every known source, available or not, in preference order.
func (r *registry) infos() []BackendInfo {
	all := []BackendInfo{
		{ID: "flatpak", Name: "Flatpak", Kind: "universal",
			Description: "Sandboxed, cross-distro apps from Flathub. Works the same on every Linux."},
	}
	if n := nativeInfoFor(r.nativeID); n.ID != "" {
		all = append(all, n)
	}
	all = append(all, BackendInfo{ID: "snap", Name: "Snap", Kind: "universal",
		Description: "Canonical's sandboxed packages, common on Ubuntu and derivatives."})

	out := make([]BackendInfo, 0, len(all))
	for _, info := range all {
		for _, s := range r.sources {
			if s.info().ID == info.ID {
				info.Available = true
			}
		}
		out = append(out, info)
	}
	return out
}

func nativeInfoFor(family string) BackendInfo {
	switch family {
	case "apt":
		return BackendInfo{ID: "apt", Name: "APT (Debian/Ubuntu)", Kind: "native",
			Description: "The distro's own packages via apt. Best integration with the desktop."}
	case "dnf":
		return BackendInfo{ID: "dnf", Name: "DNF (Fedora/RHEL)", Kind: "native",
			Description: "The distro's own packages via dnf. Best integration with the desktop."}
	case "pacman":
		return BackendInfo{ID: "pacman", Name: "pacman (Arch)", Kind: "native",
			Description: "The distro's own packages via pacman. Best integration with the desktop."}
	case "zypper":
		return BackendInfo{ID: "zypper", Name: "Zypper (openSUSE)", Kind: "native",
			Description: "The distro's own packages via zypper. Best integration with the desktop."}
	case "apk":
		return BackendInfo{ID: "apk", Name: "apk (Alpine)", Kind: "native",
			Description: "The distro's own packages via apk. Best integration with the desktop."}
	default:
		return BackendInfo{}
	}
}

// --- shared helpers -------------------------------------------------------

// isRoot reports whether we can run privileged commands directly.
func isRoot() bool { return os.Geteuid() == 0 }

// elevated prefixes a command with pkexec when we are not root.
func elevated(args ...string) []string {
	if isRoot() {
		return args
	}
	return append([]string{"pkexec", "env"}, args...)
}

// runLines runs name(args...) and calls emit for every output line and every
// parsed percentage found in it. Lines are split on both \n and \r so
// progress bars that redraw in place still surface.
func runLines(ctx context.Context, emit func(InstallEvent), name string, args ...string) error {
	cmd := exec.CommandContext(ctx, name, args...)
	cmd.WaitDelay = 10 * time.Second
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return err
	}
	cmd.Stderr = cmd.Stdout // merge: progress tools print to either
	if err := cmd.Start(); err != nil {
		return err
	}
	scan := func(r io.Reader) {
		buf := make([]byte, 4096)
		var acc []byte
		flush := func(part []byte) {
			line := strings.TrimRight(string(part), " \t")
			if line == "" {
				return
			}
			emit(InstallEvent{Type: "line", Line: line, Percent: parsePercent(line)})
		}
		for {
			n, err := r.Read(buf)
			for _, b := range buf[:n] {
				if b == '\n' || b == '\r' {
					flush(acc)
					acc = acc[:0]
					continue
				}
				acc = append(acc, b)
			}
			if err != nil {
				break
			}
		}
		flush(acc)
	}
	scan(stdout)
	waitErr := cmd.Wait()
	if ctx.Err() != nil {
		return ctx.Err()
	}
	return waitErr
}

// parsePercent extracts the first "NN%" in a line, or -1.
func parsePercent(line string) float64 {
	for i := 0; i < len(line); i++ {
		if line[i] < '0' || line[i] > '9' {
			continue
		}
		j := i
		num := 0
		for j < len(line) && line[j] >= '0' && line[j] <= '9' {
			num = num*10 + int(line[j]-'0')
			j++
		}
		if j < len(line) && line[j] == '%' && num <= 100 {
			return float64(num)
		}
		i = j
	}
	return -1
}

// queryCmd runs a short read-only command and returns its stdout.
func queryCmd(ctx context.Context, timeout time.Duration, name string, args ...string) (string, error) {
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	cmd := exec.CommandContext(ctx, name, args...)
	cmd.WaitDelay = 5 * time.Second
	out, err := cmd.Output()
	if err != nil {
		return "", err
	}
	return string(out), nil
}

// lookPathOK guards package-name arguments before they reach exec.
func safeRef(s string) bool {
	if s == "" || len(s) > 200 || strings.Contains(s, "..") {
		return false
	}
	for _, r := range s {
		ok := r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' || r >= '0' && r <= '9' ||
			r == '.' || r == '-' || r == '_' || r == '+' || r == '@' || r == ':'
		if !ok {
			return false
		}
	}
	return true
}

// --- flatpak --------------------------------------------------------------

type flatpakSource struct{}

func newFlatpakSource() *flatpakSource { return &flatpakSource{} }

func (flatpakSource) info() BackendInfo {
	return BackendInfo{ID: "flatpak", Name: "Flatpak", Kind: "universal", Available: true,
		Description: "Sandboxed, cross-distro apps from Flathub. Works the same on every Linux."}
}

const (
	flathubRepoName = "flathub"
	flathubRepoURL  = "https://dl.flathub.org/repo/flathub.flatpakrepo"
)

// flathubScopes lists scopes that carry a flathub remote.
func flathubScopes(ctx context.Context) []string {
	var scopes []string
	for _, scope := range []string{"user", "system"} {
		args := []string{"remotes", "--" + scope, "--columns=name"}
		out, err := queryCmd(ctx, 10*time.Second, "flatpak", args...)
		if err == nil && containsLine(out, flathubRepoName) {
			scopes = append(scopes, scope)
		}
	}
	return scopes
}

// EnsureFlathub adds the flathub remote for the user when missing.
func EnsureFlathub(ctx context.Context) error {
	if len(flathubScopes(ctx)) > 0 {
		return nil
	}
	ctx, cancel := context.WithTimeout(ctx, 60*time.Second)
	defer cancel()
	return exec.CommandContext(ctx, "flatpak", "remote-add", "--user", "--if-not-exists",
		flathubRepoName, flathubRepoURL).Run()
}

func (flatpakSource) installedRefs(ctx context.Context) (map[string]string, error) {
	refs := map[string]string{}
	for _, scope := range []string{"user", "system"} {
		out, err := queryCmd(ctx, 20*time.Second, "flatpak", "list", "--app", "--"+scope, "--columns=application")
		if err != nil {
			continue // a scope may not exist for unprivileged users; not fatal
		}
		for _, line := range strings.Fields(out) {
			line = strings.TrimSpace(line)
			if line != "" && line != "Application" {
				if _, ok := refs[line]; !ok {
					refs[line] = scope
				}
			}
		}
	}
	return refs, nil
}

func (flatpakSource) install(ctx context.Context, ref string, emit func(InstallEvent)) error {
	if !safeRef(ref) {
		return fmt.Errorf("invalid flatpak ref %q", ref)
	}
	emit(InstallEvent{Type: "start", Message: "Preparing Flathub…", Percent: -1})
	if err := EnsureFlathub(ctx); err != nil {
		return fmt.Errorf("could not add the Flathub remote: %w", err)
	}
	emit(InstallEvent{Type: "progress", Message: "Downloading and installing…", Percent: -1})
	ctx, cancel := context.WithTimeout(ctx, 15*time.Minute)
	defer cancel()
	if err := runLines(ctx, emit, "flatpak", "install", "--user", "-y", "--or-update",
		flathubRepoName, ref); err != nil {
		return fmt.Errorf("flatpak install failed: %w", err)
	}
	return nil
}

func (flatpakSource) remove(ctx context.Context, ref, scope string, emit func(InstallEvent)) error {
	if !safeRef(ref) {
		return fmt.Errorf("invalid flatpak ref %q", ref)
	}
	if scope == "" {
		scope = "user"
	}
	emit(InstallEvent{Type: "start", Message: "Removing…", Percent: -1})
	ctx, cancel := context.WithTimeout(ctx, 10*time.Minute)
	defer cancel()
	args := []string{"uninstall", "--" + scope, "-y", ref}
	var err error
	if scope == "system" && !isRoot() {
		cmd := append([]string{}, elevated(append([]string{"flatpak"}, args...)...)...)
		err = runLines(ctx, emit, cmd[0], cmd[1:]...)
	} else {
		err = runLines(ctx, emit, "flatpak", args...)
	}
	if err != nil {
		return fmt.Errorf("flatpak uninstall failed: %w", err)
	}
	return nil
}

func flatpakUpdatesIn(ctx context.Context, scope string) ([]UpdateInfo, error) {
	out, err := queryCmd(ctx, 60*time.Second, "flatpak", "remote-ls", "--updates", "--app",
		"--"+scope, "--columns=application,version,branch")
	if err != nil {
		return nil, err
	}
	var updates []UpdateInfo
	sc := bufio.NewScanner(strings.NewReader(out))
	for sc.Scan() {
		fields := strings.Fields(sc.Text())
		if len(fields) == 0 || fields[0] == "Application" {
			continue
		}
		u := UpdateInfo{ID: fields[0], Name: fields[0], Backend: "flatpak", Ref: fields[0], Scope: scope}
		if len(fields) >= 2 {
			u.To = fields[1]
		}
		if len(fields) >= 3 {
			u.From = fields[2]
		}
		updates = append(updates, u)
	}
	return updates, nil
}

func (flatpakSource) updates(ctx context.Context) ([]UpdateInfo, error) {
	var all []UpdateInfo
	for _, scope := range []string{"user", "system"} {
		u, err := flatpakUpdatesIn(ctx, scope)
		if err == nil {
			all = append(all, u...)
		}
	}
	return all, nil
}

func (flatpakSource) applyUpdates(ctx context.Context, refs []UpdateInfo, emit func(InstallEvent)) error {
	emit(InstallEvent{Type: "start", Message: "Updating Flatpak apps…", Percent: -1})
	ctx, cancel := context.WithTimeout(ctx, 30*time.Minute)
	defer cancel()
	var targets []UpdateInfo
	for _, r := range refs {
		if r.Backend == "flatpak" {
			targets = append(targets, r)
		}
	}
	if len(targets) == 0 {
		// everything, both scopes
		for _, scope := range []string{"user", "system"} {
			args := []string{"update", "--" + scope, "-y", "--app"}
			if scope == "system" && !isRoot() {
				cmd := elevated(append([]string{"flatpak"}, args...)...)
				_ = runLines(ctx, emit, cmd[0], cmd[1:]...) // best effort
				continue
			}
			_ = runLines(ctx, emit, "flatpak", args...)
		}
		return nil
	}
	for _, scope := range []string{"user", "system"} {
		var scopeRefs []string
		for _, t := range targets {
			if t.Scope == scope {
				scopeRefs = append(scopeRefs, t.Ref)
			}
		}
		if len(scopeRefs) == 0 {
			continue
		}
		args := append([]string{"update", "--" + scope, "-y", "--app"}, scopeRefs...)
		if scope == "system" && !isRoot() {
			cmd := elevated(append([]string{"flatpak"}, args...)...)
			if err := runLines(ctx, emit, cmd[0], cmd[1:]...); err != nil {
				return err
			}
			continue
		}
		if err := runLines(ctx, emit, "flatpak", args...); err != nil {
			return err
		}
	}
	return nil
}

func (flatpakSource) launch(ctx context.Context, ref, scope string) error {
	if !safeRef(ref) {
		return fmt.Errorf("invalid flatpak ref %q", ref)
	}
	args := []string{"run"}
	if scope != "" {
		args = append(args, "--"+scope)
	}
	args = append(args, ref)
	return exec.CommandContext(ctx, "flatpak", args...).Start()
}

func containsLine(haystack, needle string) bool {
	for _, line := range strings.Split(haystack, "\n") {
		if strings.TrimSpace(line) == needle {
			return true
		}
	}
	return false
}

// --- rpm-family (dnf) -----------------------------------------------------

type dnfSource struct{ yum bool }

func (d dnfSource) bin() string {
	if d.yum {
		return "yum"
	}
	if hasBinary("dnf") {
		return "dnf"
	}
	return "yum"
}

func (d dnfSource) info() BackendInfo {
	return BackendInfo{ID: "dnf", Name: "DNF", Kind: "native", Available: true,
		Description: "The distro's own packages via dnf. Best integration with the desktop."}
}

func (d dnfSource) installedRefs(ctx context.Context) (map[string]string, error) {
	refs := map[string]string{}
	out, err := queryCmd(ctx, 45*time.Second, "rpm", "-qa", "--qf", "%{NAME}\n")
	if err != nil {
		return refs, err
	}
	for _, line := range strings.Fields(out) {
		refs[strings.TrimSpace(line)] = ""
	}
	return refs, nil
}

func (d dnfSource) install(ctx context.Context, ref string, emit func(InstallEvent)) error {
	if !safeRef(ref) {
		return fmt.Errorf("invalid package name %q", ref)
	}
	ctx, cancel := context.WithTimeout(ctx, 15*time.Minute)
	defer cancel()
	emit(InstallEvent{Type: "start", Message: "Authenticating…", Percent: -1})
	args := elevated(d.bin(), "install", "-y", ref)
	return runLines(ctx, emit, args[0], args[1:]...)
}

func (d dnfSource) remove(ctx context.Context, ref, _ string, emit func(InstallEvent)) error {
	if !safeRef(ref) {
		return fmt.Errorf("invalid package name %q", ref)
	}
	ctx, cancel := context.WithTimeout(ctx, 15*time.Minute)
	defer cancel()
	emit(InstallEvent{Type: "start", Message: "Authenticating…", Percent: -1})
	args := elevated(d.bin(), "remove", "-y", ref)
	return runLines(ctx, emit, args[0], args[1:]...)
}

func (d dnfSource) updates(ctx context.Context) ([]UpdateInfo, error) {
	// check-update exits 100 when updates exist, 0 when none, 1 on error.
	ctx, cancel := context.WithTimeout(ctx, 60*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, d.bin(), "check-update")
	cmd.WaitDelay = 5 * time.Second
	out, err := cmd.Output()
	exit := 0
	if ee, ok := err.(*exec.ExitError); ok {
		exit = ee.ExitCode()
	} else if err != nil {
		return nil, err
	}
	if exit == 1 {
		return nil, fmt.Errorf("%s check-update failed", d.bin())
	}
	var updates []UpdateInfo
	for _, line := range strings.Split(string(out), "\n") {
		fields := strings.Fields(line)
		// "app.x86_64   1.2.3   repo"
		if len(fields) != 3 {
			continue
		}
		name := fields[0]
		if i := strings.LastIndex(name, "."); i > 0 {
			name = name[:i]
		}
		if name == "Last" || strings.HasPrefix(name, "Obsoleting") {
			continue
		}
		updates = append(updates, UpdateInfo{ID: name, Name: name, Backend: "dnf", Ref: name, To: fields[1]})
	}
	return updates, nil
}

func (d dnfSource) applyUpdates(ctx context.Context, refs []UpdateInfo, emit func(InstallEvent)) error {
	emit(InstallEvent{Type: "start", Message: "Updating packages…", Percent: -1})
	ctx, cancel := context.WithTimeout(ctx, 45*time.Minute)
	defer cancel()
	args := elevated(d.bin(), "upgrade", "-y")
	return runLines(ctx, emit, args[0], args[1:]...)
}

func (d dnfSource) launch(ctx context.Context, ref, _ string) error {
	path, err := exec.LookPath(ref)
	if err != nil {
		return fmt.Errorf("%q is not on PATH", ref)
	}
	return exec.CommandContext(ctx, path).Start()
}

// --- apt ------------------------------------------------------------------

type aptSource struct{}

func (aptSource) info() BackendInfo {
	return BackendInfo{ID: "apt", Name: "APT", Kind: "native", Available: true,
		Description: "The distro's own packages via apt. Best integration with the desktop."}
}

func (aptSource) installedRefs(ctx context.Context) (map[string]string, error) {
	refs := map[string]string{}
	out, err := queryCmd(ctx, 45*time.Second, "dpkg-query", "-W", "-f", "${Package}\t${db:Status-Status}\n")
	if err != nil {
		return refs, err
	}
	for _, line := range strings.Split(out, "\n") {
		name, status, ok := strings.Cut(line, "\t")
		if ok && status == "installed" {
			refs[strings.TrimSpace(name)] = ""
		}
	}
	return refs, nil
}

func (aptSource) install(ctx context.Context, ref string, emit func(InstallEvent)) error {
	if !safeRef(ref) {
		return fmt.Errorf("invalid package name %q", ref)
	}
	ctx, cancel := context.WithTimeout(ctx, 15*time.Minute)
	defer cancel()
	emit(InstallEvent{Type: "start", Message: "Authenticating…", Percent: -1})
	args := elevated("env", "DEBIAN_FRONTEND=noninteractive", "apt-get", "install", "-y", ref)
	return runLines(ctx, emit, args[0], args[1:]...)
}

func (aptSource) remove(ctx context.Context, ref, _ string, emit func(InstallEvent)) error {
	if !safeRef(ref) {
		return fmt.Errorf("invalid package name %q", ref)
	}
	ctx, cancel := context.WithTimeout(ctx, 15*time.Minute)
	defer cancel()
	emit(InstallEvent{Type: "start", Message: "Authenticating…", Percent: -1})
	args := elevated("env", "DEBIAN_FRONTEND=noninteractive", "apt-get", "remove", "-y", ref)
	return runLines(ctx, emit, args[0], args[1:]...)
}

func (aptSource) updates(ctx context.Context) ([]UpdateInfo, error) {
	out, err := queryCmd(ctx, 60*time.Second, "apt", "list", "--upgradable")
	if err != nil {
		return nil, err
	}
	var updates []UpdateInfo
	for _, line := range strings.Split(out, "\n") {
		// "firefox-esr/stable-security 115.9.1esr-1~deb12u1 amd64 [upgradable from: 115.8.0esr-1~deb12u1]"
		fields := strings.Fields(line)
		if len(fields) < 2 || !strings.Contains(line, "[upgradable") {
			continue
		}
		name := strings.SplitN(fields[0], "/", 2)[0]
		u := UpdateInfo{ID: name, Name: name, Backend: "apt", Ref: name, To: fields[1]}
		updates = append(updates, u)
	}
	return updates, nil
}

func (aptSource) applyUpdates(ctx context.Context, refs []UpdateInfo, emit func(InstallEvent)) error {
	emit(InstallEvent{Type: "start", Message: "Updating packages…", Percent: -1})
	ctx, cancel := context.WithTimeout(ctx, 45*time.Minute)
	defer cancel()
	args := elevated("env", "DEBIAN_FRONTEND=noninteractive", "apt-get", "upgrade", "-y")
	return runLines(ctx, emit, args[0], args[1:]...)
}

func (aptSource) launch(ctx context.Context, ref, _ string) error {
	path, err := exec.LookPath(ref)
	if err != nil {
		return fmt.Errorf("%q is not on PATH", ref)
	}
	return exec.CommandContext(ctx, path).Start()
}

// --- pacman ---------------------------------------------------------------

type pacmanSource struct{}

func (pacmanSource) info() BackendInfo {
	return BackendInfo{ID: "pacman", Name: "pacman", Kind: "native", Available: true,
		Description: "The distro's own packages via pacman. Best integration with the desktop."}
}

func (pacmanSource) installedRefs(ctx context.Context) (map[string]string, error) {
	refs := map[string]string{}
	out, err := queryCmd(ctx, 45*time.Second, "pacman", "-Qq")
	if err != nil {
		return refs, err
	}
	for _, line := range strings.Fields(out) {
		refs[strings.TrimSpace(line)] = ""
	}
	return refs, nil
}

func (pacmanSource) install(ctx context.Context, ref string, emit func(InstallEvent)) error {
	if !safeRef(ref) {
		return fmt.Errorf("invalid package name %q", ref)
	}
	ctx, cancel := context.WithTimeout(ctx, 15*time.Minute)
	defer cancel()
	emit(InstallEvent{Type: "start", Message: "Authenticating…", Percent: -1})
	args := elevated("pacman", "-S", "--needed", "--noconfirm", ref)
	return runLines(ctx, emit, args[0], args[1:]...)
}

func (pacmanSource) remove(ctx context.Context, ref, _ string, emit func(InstallEvent)) error {
	if !safeRef(ref) {
		return fmt.Errorf("invalid package name %q", ref)
	}
	ctx, cancel := context.WithTimeout(ctx, 15*time.Minute)
	defer cancel()
	emit(InstallEvent{Type: "start", Message: "Authenticating…", Percent: -1})
	args := elevated("pacman", "-Rns", "--noconfirm", ref)
	return runLines(ctx, emit, args[0], args[1:]...)
}

func (pacmanSource) updates(ctx context.Context) ([]UpdateInfo, error) {
	out, err := queryCmd(ctx, 60*time.Second, "pacman", "-Qu")
	if err != nil && len(strings.TrimSpace(out)) == 0 {
		// pacman -Qu exits 1 when there is nothing to do
		return nil, nil
	}
	var updates []UpdateInfo
	for _, line := range strings.Split(out, "\n") {
		// "firefox 124.0.1-1 -> 125.0.1-1"
		fields := strings.Fields(line)
		if len(fields) < 4 {
			continue
		}
		updates = append(updates, UpdateInfo{ID: fields[0], Name: fields[0], Backend: "pacman", Ref: fields[0], From: fields[1], To: fields[3]})
	}
	return updates, nil
}

func (pacmanSource) applyUpdates(ctx context.Context, refs []UpdateInfo, emit func(InstallEvent)) error {
	emit(InstallEvent{Type: "start", Message: "Updating packages…", Percent: -1})
	ctx, cancel := context.WithTimeout(ctx, 45*time.Minute)
	defer cancel()
	args := elevated("pacman", "-Syu", "--noconfirm")
	return runLines(ctx, emit, args[0], args[1:]...)
}

func (pacmanSource) launch(ctx context.Context, ref, _ string) error {
	path, err := exec.LookPath(ref)
	if err != nil {
		return fmt.Errorf("%q is not on PATH", ref)
	}
	return exec.CommandContext(ctx, path).Start()
}

// --- zypper (openSUSE, rpm-based) -----------------------------------------

type zypperSource struct{}

func (zypperSource) info() BackendInfo {
	return BackendInfo{ID: "zypper", Name: "Zypper", Kind: "native", Available: true,
		Description: "The distro's own packages via zypper. Best integration with the desktop."}
}

func (zypperSource) installedRefs(ctx context.Context) (map[string]string, error) {
	refs := map[string]string{}
	out, err := queryCmd(ctx, 60*time.Second, "rpm", "-qa", "--qf", "%{NAME}\n")
	if err != nil {
		return refs, err
	}
	for _, line := range strings.Fields(out) {
		refs[strings.TrimSpace(line)] = ""
	}
	return refs, nil
}

func (zypperSource) install(ctx context.Context, ref string, emit func(InstallEvent)) error {
	if !safeRef(ref) {
		return fmt.Errorf("invalid package name %q", ref)
	}
	ctx, cancel := context.WithTimeout(ctx, 15*time.Minute)
	defer cancel()
	emit(InstallEvent{Type: "start", Message: "Authenticating…", Percent: -1})
	args := elevated("zypper", "--non-interactive", "install", ref)
	return runLines(ctx, emit, args[0], args[1:]...)
}

func (zypperSource) remove(ctx context.Context, ref, _ string, emit func(InstallEvent)) error {
	if !safeRef(ref) {
		return fmt.Errorf("invalid package name %q", ref)
	}
	ctx, cancel := context.WithTimeout(ctx, 15*time.Minute)
	defer cancel()
	emit(InstallEvent{Type: "start", Message: "Authenticating…", Percent: -1})
	args := elevated("zypper", "--non-interactive", "remove", ref)
	return runLines(ctx, emit, args[0], args[1:]...)
}

func (zypperSource) updates(ctx context.Context) ([]UpdateInfo, error) {
	out, err := queryCmd(ctx, 90*time.Second, "zypper", "--non-interactive", "list-updates")
	if err != nil {
		return nil, nil // offline or no repos: show nothing rather than an error
	}
	var updates []UpdateInfo
	for _, line := range strings.Split(out, "\n") {
		fields := strings.Split(line, "|")
		if len(fields) < 5 {
			continue
		}
		if strings.TrimSpace(fields[0]) != "v" {
			continue
		}
		name := strings.TrimSpace(fields[2])
		if name == "" || name == "Name" {
			continue
		}
		updates = append(updates, UpdateInfo{ID: name, Name: name, Backend: "zypper", Ref: name,
			From: strings.TrimSpace(fields[3]), To: strings.TrimSpace(fields[4])})
	}
	return updates, nil
}

func (zypperSource) applyUpdates(ctx context.Context, refs []UpdateInfo, emit func(InstallEvent)) error {
	emit(InstallEvent{Type: "start", Message: "Updating packages…", Percent: -1})
	ctx, cancel := context.WithTimeout(ctx, 45*time.Minute)
	defer cancel()
	args := elevated("zypper", "--non-interactive", "update")
	return runLines(ctx, emit, args[0], args[1:]...)
}

func (zypperSource) launch(ctx context.Context, ref, _ string) error {
	path, err := exec.LookPath(ref)
	if err != nil {
		return fmt.Errorf("%q is not on PATH", ref)
	}
	return exec.CommandContext(ctx, path).Start()
}

// --- apk (Alpine) ---------------------------------------------------------

type apkSource struct{}

func (apkSource) info() BackendInfo {
	return BackendInfo{ID: "apk", Name: "apk", Kind: "native", Available: true,
		Description: "The distro's own packages via apk. Best integration with the desktop."}
}

func (apkSource) installedRefs(ctx context.Context) (map[string]string, error) {
	refs := map[string]string{}
	out, err := queryCmd(ctx, 30*time.Second, "apk", "info")
	if err != nil {
		return refs, err
	}
	for _, line := range strings.Fields(out) {
		refs[strings.TrimSpace(line)] = ""
	}
	return refs, nil
}

func (apkSource) install(ctx context.Context, ref string, emit func(InstallEvent)) error {
	if !safeRef(ref) {
		return fmt.Errorf("invalid package name %q", ref)
	}
	ctx, cancel := context.WithTimeout(ctx, 15*time.Minute)
	defer cancel()
	emit(InstallEvent{Type: "start", Message: "Authenticating…", Percent: -1})
	args := elevated("apk", "add", ref)
	return runLines(ctx, emit, args[0], args[1:]...)
}

func (apkSource) remove(ctx context.Context, ref, _ string, emit func(InstallEvent)) error {
	if !safeRef(ref) {
		return fmt.Errorf("invalid package name %q", ref)
	}
	ctx, cancel := context.WithTimeout(ctx, 10*time.Minute)
	defer cancel()
	emit(InstallEvent{Type: "start", Message: "Authenticating…", Percent: -1})
	args := elevated("apk", "del", ref)
	return runLines(ctx, emit, args[0], args[1:]...)
}

func (apkSource) updates(ctx context.Context) ([]UpdateInfo, error) {
	out, err := queryCmd(ctx, 60*time.Second, "apk", "version", "-l", "<")
	if err != nil {
		return nil, nil
	}
	var updates []UpdateInfo
	for _, line := range strings.Split(out, "\n") {
		// "firefox-123.0-r0 < 124.0-r0"
		fields := strings.Fields(line)
		if len(fields) < 3 || fields[1] != "<" {
			continue
		}
		name := fields[0]
		if i := strings.LastIndex(name, "-"); i > 0 {
			name = name[:i] // strip version suffix to get the package name
		}
		updates = append(updates, UpdateInfo{ID: name, Name: name, Backend: "apk", Ref: name, From: fields[0], To: fields[2]})
	}
	return updates, nil
}

func (apkSource) applyUpdates(ctx context.Context, refs []UpdateInfo, emit func(InstallEvent)) error {
	emit(InstallEvent{Type: "start", Message: "Updating packages…", Percent: -1})
	ctx, cancel := context.WithTimeout(ctx, 45*time.Minute)
	defer cancel()
	args := elevated("apk", "upgrade")
	return runLines(ctx, emit, args[0], args[1:]...)
}

func (apkSource) launch(ctx context.Context, ref, _ string) error {
	path, err := exec.LookPath(ref)
	if err != nil {
		return fmt.Errorf("%q is not on PATH", ref)
	}
	return exec.CommandContext(ctx, path).Start()
}

// --- snap -----------------------------------------------------------------

type snapSource struct{}

func (snapSource) info() BackendInfo {
	return BackendInfo{ID: "snap", Name: "Snap", Kind: "universal", Available: true,
		Description: "Canonical's sandboxed packages, common on Ubuntu and derivatives."}
}

func (snapSource) installedRefs(ctx context.Context) (map[string]string, error) {
	refs := map[string]string{}
	out, err := queryCmd(ctx, 30*time.Second, "snap", "list")
	if err != nil {
		// snapd not running, or snap unusable: treat as empty, not an error
		return refs, nil
	}
	for i, line := range strings.Split(out, "\n") {
		if i == 0 {
			continue // header
		}
		name := strings.Fields(line)
		if len(name) > 0 {
			refs[name[0]] = ""
		}
	}
	return refs, nil
}

func (snapSource) install(ctx context.Context, ref string, emit func(InstallEvent)) error {
	if !safeRef(ref) {
		return fmt.Errorf("invalid snap name %q", ref)
	}
	ctx, cancel := context.WithTimeout(ctx, 15*time.Minute)
	defer cancel()
	emit(InstallEvent{Type: "start", Message: "Authenticating…", Percent: -1})
	args := elevated("snap", "install", ref)
	return runLines(ctx, emit, args[0], args[1:]...)
}

func (snapSource) remove(ctx context.Context, ref, _ string, emit func(InstallEvent)) error {
	if !safeRef(ref) {
		return fmt.Errorf("invalid snap name %q", ref)
	}
	ctx, cancel := context.WithTimeout(ctx, 10*time.Minute)
	defer cancel()
	emit(InstallEvent{Type: "start", Message: "Authenticating…", Percent: -1})
	args := elevated("snap", "remove", ref)
	return runLines(ctx, emit, args[0], args[1:]...)
}

func (snapSource) updates(ctx context.Context) ([]UpdateInfo, error) {
	// snaps refresh themselves automatically; listing would just add noise.
	return nil, nil
}

func (snapSource) applyUpdates(ctx context.Context, _ []UpdateInfo, emit func(InstallEvent)) error {
	emit(InstallEvent{Type: "start", Message: "Refreshing snaps…", Percent: -1})
	ctx, cancel := context.WithTimeout(ctx, 30*time.Minute)
	defer cancel()
	args := elevated("snap", "refresh")
	return runLines(ctx, emit, args[0], args[1:]...)
}

func (snapSource) launch(ctx context.Context, ref, _ string) error {
	path, err := exec.LookPath(ref)
	if err != nil {
		return fmt.Errorf("%q is not on PATH", ref)
	}
	return exec.CommandContext(ctx, path).Start()
}
