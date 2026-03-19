package main

import (
	"embed"
	"log"

	"github.com/wailsapp/wails/v3/pkg/application"
)

//go:embed all:frontend/dist
var assets embed.FS

func main() {
	app := NewApp()

	wailsApp := application.New(application.Options{
		Name: "Vox",
		Assets: application.AssetOptions{
			Handler: application.AssetFileServerFS(assets),
		},
		Mac: application.MacOptions{
			ApplicationShouldTerminateAfterLastWindowClosed: false,
		},
		Services: []application.Service{
			application.NewService(app),
		},
	})

	window := wailsApp.Window.NewWithOptions(application.WebviewWindowOptions{
		Title:     "Vox",
		Width:     860,
		Height:    620,
		MinWidth:  640,
		MinHeight: 480,
		Hidden:    false,
		Mac: application.MacWindow{
			TitleBar:                application.MacTitleBarHiddenInset,
			Backdrop:                application.MacBackdropTranslucent,
			InvisibleTitleBarHeight: 50,
		},
	})

	// System tray with native support on all platforms
	trayMenu := wailsApp.NewMenu()
	statusItem := trayMenu.Add("Idle")
	statusItem.SetEnabled(false)
	trayMenu.AddSeparator()
	trayMenu.Add("Settings").OnClick(func(ctx *application.Context) {
		window.Show()
		window.Focus()
	})
	trayMenu.AddSeparator()
	trayMenu.Add("Quit Vox").OnClick(func(ctx *application.Context) {
		wailsApp.Quit()
	})

	systemTray := wailsApp.SystemTray.New()
	systemTray.SetMenu(trayMenu)
	systemTray.SetIcon(iconIdle)

	// Store references for app
	app.wailsApp = wailsApp
	app.window = window
	app.systemTray = systemTray
	app.trayStatusItem = statusItem

	if err := wailsApp.Run(); err != nil {
		log.Fatal(err)
	}
}
