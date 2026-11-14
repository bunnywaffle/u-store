package store

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func writeTemp(t *testing.T, name, content string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), name)
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestInspectDebOnFedoraUnsupported(t *testing.T) {
	path := writeTemp(t, "cool-app_1.2.3_amd64.deb", "fake deb bytes")
	s := &Store{distro: Distro{Family: "dnf"}, registry: &registry{}}
	p, err := s.InspectLocalPackage(context.Background(), path)
	if err != nil {
		t.Fatal(err)
	}
	if p.Kind != "deb" {
		t.Fatalf("kind = %q", p.Kind)
	}
	if p.Supported {
		t.Fatal("a .deb must not report supported on a dnf system")
	}
	if !strings.Contains(p.Reason, "Flatpak") {
		t.Fatalf("reason should suggest flatpak, got %q", p.Reason)
	}
}

func TestInspectDebOnDebianFamilySupported(t *testing.T) {
	path := writeTemp(t, "cool-app_1.2.3_amd64.deb", "fake deb bytes")
	s := &Store{distro: Distro{Family: "apt"}, registry: &registry{}}
	p, err := s.InspectLocalPackage(context.Background(), path)
	if err != nil {
		t.Fatal(err)
	}
	if !p.Supported {
		t.Fatalf("a .deb must be supported on apt systems: %+v", p)
	}
	joined := strings.Join(p.argv, " ")
	if !strings.Contains(joined, "apt-get") {
		t.Fatalf("expected apt-get in argv, got %q", joined)
	}
	if !strings.Contains(joined, path) {
		t.Fatalf("expected the file path in argv, got %q", joined)
	}
}

func TestInspectRpmOnFedoraSupported(t *testing.T) {
	path := writeTemp(t, "cool-app-1.2.3-1.fc44.x86_64.rpm", "fake rpm bytes")
	s := &Store{distro: Distro{Family: "dnf"}, registry: &registry{}}
	p, err := s.InspectLocalPackage(context.Background(), path)
	if err != nil {
		t.Fatal(err)
	}
	if p.Kind != "rpm" || !p.Supported {
		t.Fatalf("rpm should be supported on dnf: %+v", p)
	}
	if !strings.Contains(strings.Join(p.argv, " "), "dnf") {
		t.Fatalf("expected dnf in argv, got %q", strings.Join(p.argv, " "))
	}
}

func TestInspectArchPackageFilenameParsing(t *testing.T) {
	path := writeTemp(t, "cool-app-1.2.3-1-x86_64.pkg.tar.zst", "fake arch package")
	s := &Store{distro: Distro{Family: "pacman"}, registry: &registry{}}
	p, err := s.InspectLocalPackage(context.Background(), path)
	if err != nil {
		t.Fatal(err)
	}
	if p.Kind != "arch" {
		t.Fatalf("kind = %q", p.Kind)
	}
	if p.Name != "cool-app" {
		t.Fatalf("name = %q, want cool-app", p.Name)
	}
	if p.Version != "1.2.3-1" {
		t.Fatalf("version = %q, want 1.2.3-1", p.Version)
	}
	if !p.Supported {
		t.Fatal("arch packages are supported on pacman")
	}
}

func TestInspectApkFilenameParsing(t *testing.T) {
	path := writeTemp(t, "cool-app-1.32-r7.apk", "fake apk")
	s := &Store{distro: Distro{Family: "apk"}, registry: &registry{}}
	p, err := s.InspectLocalPackage(context.Background(), path)
	if err != nil {
		t.Fatal(err)
	}
	if p.Name != "cool-app" {
		t.Fatalf("name = %q, want cool-app", p.Name)
	}
	if p.Version != "1.32-r7" {
		t.Fatalf("version = %q, want 1.32-r7", p.Version)
	}
}

func TestInspectUnknownFile(t *testing.T) {
	path := writeTemp(t, "setup.exe", "fake exe")
	s := &Store{distro: Distro{Family: "dnf"}, registry: &registry{}}
	p, err := s.InspectLocalPackage(context.Background(), path)
	if err != nil {
		t.Fatal(err)
	}
	if p.Kind != "unknown" || p.Supported {
		t.Fatalf("expected unsupported unknown, got %+v", p)
	}
}

func TestInspectMissingFile(t *testing.T) {
	s := &Store{distro: Distro{Family: "dnf"}, registry: &registry{}}
	_, err := s.InspectLocalPackage(context.Background(), "/nonexistent/file.deb")
	if err == nil {
		t.Fatal("expected an error for a missing file")
	}
}

func TestInspectFlatpakNeedsFlathubRegistry(t *testing.T) {
	path := writeTemp(t, "org.mozilla.firefox.flatpak", "fake bundle")
	s := &Store{distro: Distro{Family: "dnf"}, registry: &registry{}}
	p, err := s.InspectLocalPackage(context.Background(), path)
	if err != nil {
		t.Fatal(err)
	}
	if p.Supported {
		t.Fatal("flatpak install must be unsupported when flatpak is missing")
	}
}

func TestAppImageIntegrationDoesNotNeedAuth(t *testing.T) {
	path := writeTemp(t, "Cool-App-x86_64.AppImage", "fake appimage")
	s := &Store{distro: Distro{Family: "dnf"}, registry: &registry{}}
	p, err := s.InspectLocalPackage(context.Background(), path)
	if err != nil {
		t.Fatal(err)
	}
	if p.Kind != "appimage" || !p.Supported {
		t.Fatalf("appimage should always be supported: %+v", p)
	}
	if p.NeedsAuth {
		t.Fatal("AppImage integration must not require authentication")
	}
}
