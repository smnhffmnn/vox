package main

import (
	"embed"
	"fmt"
	"log"
	"os"

	"github.com/wailsapp/wails/v3/pkg/application"
)

//go:embed all:frontend/dist
var assets embed.FS

func main() {
	if len(os.Args) > 1 && os.Args[1] == "--version" {
		fmt.Println("vox " + version)
		os.Exit(0)
	}

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

	// Floating overlay (always-on-top, frameless, transparent, visible on all Spaces)
	overlayWindow := wailsApp.Window.NewWithOptions(application.WebviewWindowOptions{
		Name:              "overlay",
		Title:             "",
		URL:               "/overlay.html",
		Width:             240,
		Height:            36,
		AlwaysOnTop:       true,
		Frameless:         true,
		DisableResize:     true,
		Hidden:            true,
		IgnoreMouseEvents: true,
		BackgroundType:    application.BackgroundTypeTransparent,
		BackgroundColour:  application.RGBA{Red: 0, Green: 0, Blue: 0, Alpha: 0},
		Mac: application.MacWindow{
			Backdrop:      application.MacBackdropTransparent,
			DisableShadow: true,
			WindowLevel:   application.MacWindowLevelScreenSaver,
			CollectionBehavior: application.MacWindowCollectionBehaviorCanJoinAllSpaces |
				application.MacWindowCollectionBehaviorStationary |
				application.MacWindowCollectionBehaviorIgnoresCycle |
				application.MacWindowCollectionBehaviorFullScreenAuxiliary,
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
	app.overlayWindow = overlayWindow
	app.systemTray = systemTray
	app.trayStatusItem = statusItem

	if err := wailsApp.Run(); err != nil {
		log.Fatal(err)
	}
}
