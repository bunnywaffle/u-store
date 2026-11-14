package store

import (
	"os"
	"strings"
	"testing"
)

func TestFamilyDetectionAcrossDistros(t *testing.T) {
	cases := []struct {
		id   string
		like string
		want string
	}{
		// Debian family
		{"debian", "", "apt"}, {"ubuntu", "debian", "apt"}, {"linuxmint", "ubuntu debian", "apt"},
		{"pop", "ubuntu debian", "apt"}, {"zorin", "ubuntu debian", "apt"}, {"neon", "ubuntu debian", "apt"},
		{"kali", "debian", "apt"}, {"mx", "debian", "apt"}, {"elementary", "ubuntu debian", "apt"},
		{"raspbian", "debian", "apt"},
		// Fedora / RHEL family
		{"fedora", "", "dnf"}, {"rhel", "", "dnf"}, {"centos", "rhel", "dnf"},
		{"rocky", "rhel", "dnf"}, {"almalinux", "rhel", "dnf"}, {"ol", "rhel", "dnf"}, {"amzn", "rhel", "dnf"},
		// Arch family
		{"arch", "", "pacman"}, {"manjaro", "arch", "pacman"}, {"endeavouros", "arch", "pacman"},
		{"artix", "arch", "pacman"}, {"garuda", "arch", "pacman"}, {"cachyos", "arch", "pacman"},
		{"steamos", "arch", "pacman"},
		// openSUSE family
		{"opensuse-tumbleweed", "opensuse", "zypper"}, {"opensuse-leap", "opensuse", "zypper"},
		{"sled", "suse", "zypper"}, {"sles", "suse", "zypper"},
		// Alpine
		{"alpine", "", "apk"},
		// Unsupported families must degrade to Flatpak/snap only
		{"gentoo", "", ""}, {"nixos", "", ""}, {"void", "", ""}, {"solus", "", ""},
	}
	for _, tc := range cases {
		var like []string
		if tc.like != "" {
			like = strings.Fields(tc.like)
		}
		if got := familyOf(tc.id, like); got != tc.want {
			t.Errorf("familyOf(%q, %q) = %q, want %q", tc.id, tc.like, got, tc.want)
		}
	}
}

func TestDetectDistroAtomicAndIDParsing(t *testing.T) {
	dir := t.TempDir()
	fake := dir + "/os-release"
	content := "ID=silverblue\nID_LIKE=\"fedora rhel\"\nNAME=\"Fedora Linux\"\nPRETTY_NAME=\"Fedora Linux 44 (Silverblue)\"\n"
	if err := os.WriteFile(fake, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	oldRelease, oldOsTree := etcOSRelease, ostreeBooted
	etcOSRelease = fake
	ostreeBooted = dir + "/ostree-booted" // absent on purpose
	defer func() { etcOSRelease, ostreeBooted = oldRelease, oldOsTree }()

	d := DetectDistro()
	if d.Family != "dnf" {
		t.Errorf("fedora id_like chain must detect dnf, got %q", d.Family)
	}
	if d.Atomic {
		t.Error("no ostree file → must not report atomic")
	}

	// Now simulate OSTree boot
	if err := os.WriteFile(ostreeBooted, []byte{}, 0o644); err != nil {
		t.Fatal(err)
	}
	d = DetectDistro()
	if !d.Atomic {
		t.Error("ostree-booted present → must report atomic")
	}
	if d.Family != "dnf" {
		t.Errorf("family must remain dnf on atomic systems, got %q", d.Family)
	}
}

func TestDetectDistroMissingFile(t *testing.T) {
	old := etcOSRelease
	etcOSRelease = "/nonexistent/os-release-987654321"
	defer func() { etcOSRelease = old }()
	d := DetectDistro()
	if d.ID != "" || d.Family != "" || d.Pretty != "" {
		t.Fatalf("missing os-release must yield empty detection, got %+v", d)
	}
}
