package store

import (
	"context"
	"fmt"
	"os"
	"testing"
	"time"
)

// TestContainerProbe prints a diagnostic report of the store's backend
// behavior inside the CURRENT distro. It is a no-op outside a deliberately
// chosen container (STORE_PROBE=1) so normal test runs stay quiet.
func TestContainerProbe(t *testing.T) {
	if os.Getenv("STORE_PROBE") != "1" {
		t.Skip("set STORE_PROBE=1 to run the container diagnostic")
	}

	d := DetectDistro()
	fmt.Printf("PROBE distro: id=%q idlike=%v family=%q pretty=%q arch=%s atomic=%v\n",
		d.ID, d.IDLike, d.Family, d.Pretty, d.Arch, d.Atomic)

	s, err := New()
	if err != nil {
		t.Fatalf("store must construct: %v", err)
	}

	fmt.Println("PROBE backends:")
	for _, info := range s.registry.infos() {
		fmt.Printf("  - %s (%s) kind=%s available=%v\n", info.Name, info.ID, info.Kind, info.Available)
	}

	fmt.Println("PROBE local-install plan (fake files):")
	for _, tc := range [][2]string{
		{"deb", "cool-app_1.2.3_amd64.deb"},
		{"rpm", "cool-app-1.2.3-1.x86_64.rpm"},
		{"arch", "cool-app-1.2.3-1-x86_64.pkg.tar.zst"},
		{"apk", "cool-app-1.32-r7.apk"},
		{"snap", "cool-app_1.2.3_amd64.snap"},
		{"flatpak", "org.example.CoolApp.flatpak"},
		{"appimage", "Cool-App-x86_64.AppImage"},
	} {
		p := LocalPackage{Kind: tc[0], FileName: tc[1], Path: "/tmp/" + tc[1]}
		s.planLocalInstall(&p)
		fmt.Printf("  - %-8s supported=%v needsAuth=%v cmd=%q reason=%q\n",
			p.Kind, p.Supported, p.NeedsAuth, p.Command, p.Reason)
	}

	q := os.Getenv("PROBE_QUERY")
	if q == "" {
		q = "firefox"
	}
	ctx, cancel := context.WithTimeout(context.Background(), 180*time.Second)
	defer cancel()
	res := s.SearchLocal(ctx, q)
	fmt.Printf("PROBE local search %q: hits=%d errors=%v\n", q, len(res.Hits), res.Errors)
	for i, h := range res.Hits {
		if i >= 8 {
			break
		}
		fmt.Printf("  hit: %-40s installed=%v backend=%s\n", h.Name, h.Installed, h.Backend)
	}

	start := time.Now()
	updates, err := s.Updates(ctx)
	fmt.Printf("PROBE updates: n=%d err=%v (%s)\n", len(updates), err, time.Since(start).Round(time.Second))
}
