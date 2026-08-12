package main

//go:generate go run ../../tools/genresources

import (
	"log"
	"os"

	"zapret-manager/internal/admin"
	"zapret-manager/internal/app"
	"zapret-manager/internal/ui"
	"zapret-manager/internal/version"
)

func main() {
	// Before any WebView2 init: opaque #0a0b0e (AARRGGBB). Prevents white startup flash.
	_ = os.Setenv("WEBVIEW2_DEFAULT_BACKGROUND_COLOR", "FF0A0B0E")

	if err := admin.EnsureElevated(); err != nil {
		log.Fatal(err)
	}
	admin.HideConsole()
	if err := ui.Run(app.New()); err != nil {
		admin.ErrorBox(version.Title(), err.Error())
		log.Fatal(err)
	}
}
