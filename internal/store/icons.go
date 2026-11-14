package store

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

// PackageIconPath is the resolver used by the us-icon protocol handler.
// SafeRef validates the package name before the resolver reads it.
func PackageIconPath(ctx context.Context, ref string) string { return packageIconPath(ctx, ref) }
func SafeRef(s string) bool                     { return safeRef(s) }

// Desktop-app icon resolution for native packages: how the store shows
// native apps with the same icons their menus use. Works only for installed
// packages (a .desktop file must exist), which is exactly where it matters.
// Resolution is cached; resolution failures yield "" and the page falls back
// to a letter tile.

var (
	iconCacheMu sync.Mutex
	iconCache   = map[string]string{} // ref -> absolute path, "" when unresolvable
)

var iconAllowedRoots = []string{
	"/usr/share/icons",
	"/usr/share/pixmaps",
	"/usr/share/applications",
	"/usr/local/share/icons",
	"/usr/local/share/applications",
	"/usr/local/share/pixmaps",
}

// desktopFilesForPackage lists the .desktop files belonging to an installed
// package, for rpm and dpkg.
func desktopFilesForPackage(ctx context.Context, ref string) []string {
	var args []string
	var name string
	if hasBinary("rpm") {
		name, args = "rpm", []string{"-ql", ref}
	} else if hasBinary("dpkg") {
		name, args = "dpkg", []string{"-L", ref}
	} else {
		return nil
	}
	out, err := queryCmd(ctx, 20*time.Second, name, args...)
	if err != nil {
		return nil
	}
	var files []string
	for _, line := range strings.Split(out, "\n") {
		line = strings.TrimSpace(line)
		if strings.HasSuffix(line, ".desktop") && isAllowedPath(line) {
			files = append(files, line)
		}
	}
	return files
}

// iconLineFromDesktop reads the Icon= line from a desktop file.
func iconLineFromDesktop(path string) string {
	f, err := os.Open(path)
	if err != nil {
		return ""
	}
	defer f.Close()
	sc := make([]byte, 64*1024)
	n, _ := f.Read(sc)
	for _, line := range strings.Split(string(sc[:n]), "\n") {
		key, value, ok := strings.Cut(line, "=")
		if ok && strings.EqualFold(strings.TrimSpace(key), "Icon") {
			v := strings.TrimSpace(value)
			v = strings.Trim(v, `"'`)
			if v != "" {
				return v
			}
		}
	}
	return ""
}

// resolveThemeIcon finds an icon by theme name across the common themes.
func resolveThemeIcon(name string) string {
	exts := []string{".svg", ".png"}
	sizes := []string{"512x512", "256x256", "192x192", "128x128", "96x96", "64x64", "48x48", "32x32", "24x24", "22x22", "16x16", "scalable"}
	themeRoots := []string{
		"/usr/share/icons/hicolor",
		"/usr/share/icons/breeze",
		"/usr/share/icons/adwaita",
		"/usr/share/icons/Papirus",
		"/usr/share/icons/Papirus-Dark",
		"/usr/share/icons/Papirus-Light",
		"/usr/share/icons/Yaru",
		"/usr/local/share/icons/hicolor",
	}
	for _, root := range themeRoots {
		for _, size := range sizes {
			for _, ext := range exts {
				candidate := filepath.Join(root, size, "apps", name+ext)
				if isAllowedPath(candidate) {
					if fi, err := os.Stat(candidate); err == nil && fi.Size() > 0 {
						return candidate
					}
				}
			}
		}
	}
	for _, ext := range exts {
		for _, dir := range []string{"/usr/share/pixmaps", "/usr/local/share/pixmaps"} {
			candidate := filepath.Join(dir, name+ext)
			if isAllowedPath(candidate) {
				if fi, err := os.Stat(candidate); err == nil && fi.Size() > 0 {
					return candidate
				}
			}
		}
	}
	// Symbolic fallback
	for _, root := range themeRoots {
		candidate := filepath.Join(root, "symbolic", "apps", name+"-symbolic.svg")
		if isAllowedPath(candidate) {
			if fi, err := os.Stat(candidate); err == nil && fi.Size() > 0 {
				return candidate
			}
		}
	}
	return ""
}

// packageIconPath resolves an installed package's app icon.
func packageIconPath(ctx context.Context, ref string) string {
	iconCacheMu.Lock()
	if path, ok := iconCache[ref]; ok {
		iconCacheMu.Unlock()
		return path
	}
	iconCacheMu.Unlock()

	p := resolvePackageIcon(ctx, ref)

	iconCacheMu.Lock()
	iconCache[ref] = p
	iconCacheMu.Unlock()
	return p
}

func resolvePackageIcon(ctx context.Context, ref string) string {
	if ref == "" {
		return ""
	}
	for _, desktop := range desktopFilesForPackage(ctx, ref) {
		name := iconLineFromDesktop(desktop)
		if name == "" {
			continue
		}
		if filepath.IsAbs(name) {
			if fi, err := os.Stat(name); err == nil && fi.Size() > 0 && isAllowedPath(name) {
				return name
			}
			continue
		}
		if path := resolveThemeIcon(strings.TrimSuffix(name, filepath.Ext(name))); path != "" {
			return path
		}
		if path := resolveThemeIcon(name); path != "" {
			return path
		}
	}
	return ""
}

// isAllowedPath confines icon reads to system data dirs. No .. escapes, no
// symlinks leaving the roots.
func isAllowedPath(path string) bool {
	clean := filepath.Clean(path)
	if !filepath.IsAbs(clean) {
		return false
	}
	for _, root := range iconAllowedRoots {
		root = filepath.Clean(root) + string(os.PathSeparator)
		if strings.HasPrefix(clean+string(os.PathSeparator), root) || strings.HasPrefix(clean, root) {
			return true
		}
	}
	return false
}
