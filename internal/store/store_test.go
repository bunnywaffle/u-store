package store

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestCatalogLoads(t *testing.T) {
	c, err := LoadCatalog()
	if err != nil {
		t.Fatalf("catalog must parse: %v", err)
	}
	if len(c.Apps) < 30 {
		t.Fatalf("expected a decent catalog, got %d apps", len(c.Apps))
	}
	if len(c.Categories) == 0 {
		t.Fatal("expected categories")
	}
	for _, app := range c.Apps {
		if app.ID == "" || app.Name == "" || app.Summary == "" {
			t.Fatalf("app entry missing required fields: %+v", app)
		}
		if !strings.Contains(iconFor(app.ID), app.ID) {
			t.Fatalf("icon URL should contain the app id")
		}
		for _, cat := range app.Categories {
			found := false
			for _, c2 := range c.Categories {
				if c2.ID == cat {
					found = true
				}
			}
			if !found {
				t.Fatalf("app %s references unknown category %q", app.ID, cat)
			}
		}
	}
}

func TestCatalogNativeLookup(t *testing.T) {
	c, err := LoadCatalog()
	if err != nil {
		t.Fatal(err)
	}
	if got := c.NativePackage("org.mozilla.firefox", "dnf"); got != "firefox" {
		t.Fatalf("firefox dnf package = %q", got)
	}
	if got := c.NativePackage("org.mozilla.firefox", "made-up"); got != "" {
		t.Fatalf("unknown backend should yield empty, got %q", got)
	}
	if got := c.NativePackage("com.nonexistent.App", "dnf"); got != "" {
		t.Fatalf("unknown app should yield empty, got %q", got)
	}
}

func TestFamilyOf(t *testing.T) {
	cases := []struct {
		id    string
		like  []string
		want  string
	}{
		{"fedora", nil, "dnf"},
		{"ubuntu", []string{"debian"}, "apt"},
		{"linuxmint", []string{"ubuntu", "debian"}, "apt"},
		{"arch", nil, "pacman"},
		{"endeavouros", []string{"arch"}, "pacman"},
		{"opensuse-tumbleweed", []string{"opensuse"}, "zypper"},
		{"alpine", nil, "apk"},
		{"gentoo", nil, ""},
	}
	for _, tc := range cases {
		if got := familyOf(tc.id, tc.like); got != tc.want {
			t.Errorf("familyOf(%q, %v) = %q, want %q", tc.id, tc.like, got, tc.want)
		}
	}
}

func TestParsePercent(t *testing.T) {
	cases := map[string]float64{
		"Installing: 45%":              45,
		"[#####              ]  12%":   12,
		"100% complete":                100,
		"no numbers here":              -1,
		"build 25 of 100 things":       -1,
		"downloading … 7% (eta 3s)":    7,
	}
	for line, want := range cases {
		if got := parsePercent(line); got != want {
			t.Errorf("parsePercent(%q) = %v, want %v", line, got, want)
		}
	}
}

func TestSafeRef(t *testing.T) {
	good := []string{"org.mozilla.firefox", "firefox", "libreoffice-fresh", "code_1.2+x86", "at.package@date"}
	for _, s := range good {
		if !safeRef(s) {
			t.Errorf("safeRef(%q) = false, want true", s)
		}
	}
	bad := []string{"", "../../etc/passwd", "a;rm -rf /", "a|b", "x y", strings.Repeat("a", 300)}
	for _, s := range bad {
		if safeRef(s) {
			t.Errorf("safeRef(%q) = true, want false", s)
		}
	}
}

func TestIsSafeAppID(t *testing.T) {
	if !isSafeAppID("org.mozilla.firefox") {
		t.Error("firefox id should be safe")
	}
	for _, bad := range []string{"", "../x", "a/b", "a%2fb", "..", "a?b", "a#b", "a b"} {
		if isSafeAppID(bad) {
			t.Errorf("isSafeAppID(%q) = true, want false", bad)
		}
	}
}

func TestDetectDistroFromFile(t *testing.T) {
	dir := t.TempDir()
	fake := filepath.Join(dir, "os-release")
	content := `ID=fedora
ID_LIKE="rhel fedora"
NAME="Fedora Linux"
PRETTY_NAME="Fedora Linux 44 (KDE Plasma Desktop Edition)"
`
	if err := os.WriteFile(fake, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	old := etcOSRelease
	etcOSRelease = fake
	defer func() { etcOSRelease = old }()

	d := DetectDistro()
	if d.ID != "fedora" {
		t.Errorf("ID = %q", d.ID)
	}
	if d.Family != "dnf" {
		t.Errorf("Family = %q, want dnf", d.Family)
	}
	if d.Pretty != "Fedora Linux 44 (KDE Plasma Desktop Edition)" {
		t.Errorf("Pretty = %q", d.Pretty)
	}
}

func TestSummaryReflectsInstalledState(t *testing.T) {
	c, err := LoadCatalog()
	if err != nil {
		t.Fatal(err)
	}
	s := &Store{
		catalog:  c,
		registry: &registry{},
		installed: map[string]map[string]string{},
	}
	app, ok := c.byID["org.mozilla.firefox"]
	if !ok {
		t.Fatal("firefox must be in the catalog")
	}

	// nothing installed
	sum := s.summary(*app, map[string]map[string]string{"flatpak": {}})
	if sum.Installed {
		t.Error("firefox should not report installed with an empty set")
	}

	// flatpak install detected
	sum = s.summary(*app, map[string]map[string]string{
		"flatpak": {"org.mozilla.firefox": "user"},
	})
	if !sum.Installed {
		t.Error("firefox should report installed when flatpak lists it")
	}

	// native install detected (dnf set contains the native ref)
	sum = s.summary(*app, map[string]map[string]string{
		"flatpak": {},
		"dnf":     {"firefox": ""},
	})
	if !sum.Installed {
		t.Error("firefox should report installed when the dnf set contains it")
	}
}
