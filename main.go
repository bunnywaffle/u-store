package main

import (
	"log"

	"github.com/egoist/mygo"

	"universal-store/internal/store"
)

func main() {
	svc, err := store.New()
	if err != nil {
		log.Fatalf("universal-store: %v", err)
	}
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
