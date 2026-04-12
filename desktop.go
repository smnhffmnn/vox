//go:build !nogui

package main

import (
	"embed"
	"fmt"
	"log"
	"os"

	"github.com/wailsapp/wails/v3/pkg/application"
	"github.com/wailsapp/wails/v3/pkg/events"
)

//go:embed all:frontend/dist
var assets embed.FS

func printUsage() {
	fmt.Fprintf(os.Stderr, `vox — Speech-to-Text Dictation

Cross-platform dictation tool with global hotkey, real-time
transcription, and intelligent text cleanup. Runs in the system tray.

Usage:
  vox                              Start the desktop app
  vox transcribe -f <file>         Transcribe an audio file (headless)
  vox --version                    Print version
  vox --help                       Show this help

Config: ~/.config/vox/config.yaml
More info: https://github.com/smnhffmnn/vox
`)
}

func runDesktop() {
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

	window.RegisterHook(events.Common.WindowClosing, func(e *application.WindowEvent) {
		e.Cancel()
		window.Hide()
	})

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

	app.ui = &desktopUI{
		wailsApp:       wailsApp,
		window:         window,
		overlayWindow:  overlayWindow,
		systemTray:     systemTray,
		trayStatusItem: statusItem,
	}

	if err := wailsApp.Run(); err != nil {
		log.Fatal(err)
	}
}

// desktopUI implements UIBridge using Wails v3.
type desktopUI struct {
	wailsApp       *application.App
	window         *application.WebviewWindow
	overlayWindow  *application.WebviewWindow
	systemTray     *application.SystemTray
	trayStatusItem *application.MenuItem
}

func (d *desktopUI) SetTrayIcon(icon []byte) {
	if d.systemTray != nil {
		d.systemTray.SetIcon(icon)
	}
}

func (d *desktopUI) SetTrayLabel(label string) {
	if d.trayStatusItem != nil {
		d.trayStatusItem.SetLabel(label)
	}
}

func (d *desktopUI) ShowOverlay(x, y int) {
	if d.overlayWindow != nil {
		d.overlayWindow.Show()
		d.overlayWindow.SetRelativePosition(x, y)
	}
}

func (d *desktopUI) HideOverlay() {
	if d.overlayWindow != nil {
		d.overlayWindow.Hide()
	}
}

func (d *desktopUI) EmitEvent(name string, data any) {
	if d.wailsApp != nil {
		d.wailsApp.Event.Emit(name, data)
	}
}

func (d *desktopUI) ShowWindow() {
	if d.window != nil {
		d.window.Show()
		d.window.Focus()
	}
}
