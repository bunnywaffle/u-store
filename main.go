package main

import (
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/egoist/mygo"

	"universal-store/internal/store"
)

func main() {
	// Work around the classic KDE-Wayland WebKitGTK jank: DMA-BUF sometimes
	// renders everything slowly. Can override with USTORE_DISABLE_DMABUF=0.
	if os.Getenv("USTORE_DISABLE_DMABUF") != "0" {
		os.Setenv("WEBKIT_DISABLE_DMABUF_RENDERER", "1")
	}

	svc, err := store.New()
	if err != nil {
		log.Fatalf("universal-store: %v", err)
	}

	// Serves a native package's desktop-app icon by package name.
	// Resolution and file access are confined to system data dirs.
	mygo.Protocol.HandleFunc("us-icon", func(w http.ResponseWriter, r *http.Request) {
		ref := strings.TrimPrefix(r.URL.Path, "/")
		ref = strings.TrimPrefix(ref, ".png") // compatibility, if asked by extension
		if ref == "" || !store.SafeRef(ref) {
			http.NotFound(w, r)
			return
		}
		path := store.PackageIconPath(r.Context(), ref)
		if path == "" {
			http.NotFound(w, r)
			return
		}
		ext := strings.ToLower(filepath.Ext(path))
		switch ext {
		case ".svg":
			w.Header().Set("Content-Type", "image/svg+xml")
		case ".xpm":
			w.Header().Set("Content-Type", "image/x-xpixmap")
		case ".jpg", ".jpeg":
			w.Header().Set("Content-Type", "image/jpeg")
		default:
			w.Header().Set("Content-Type", "image/png")
		}
		w.Header().Set("Cache-Control", "max-age=3600")
		http.ServeFile(w, r, path)
	})

	mygo.Bind(svc)

	mygo.App.WhenReady(func() {
		// Apply the persisted theme before the window exists, so the first
		// paint matches what the user chose.
		mode := store.LoadTheme()
		store.AppTheme(mode)
		store.ApplyThemeMode(svc, mode)
		mygo.NewWindow(mygo.WindowOptions{
			Title:           "U Store",
			Width:           1180,
			Height:          780,
			MinWidth:        900,
			MinHeight:       600,
			BackgroundColor: "light-dark(#f4f4f4, #0b0b0b)",
			StateKey:        "main",
			TitleBarStyle:   mygo.TitleBarHidden,
			TitleBarHeight:  48,
			URL:             "/",
		})
	})
	if err := mygo.App.Run(); err != nil {
		log.Fatal(err)
	}
}
