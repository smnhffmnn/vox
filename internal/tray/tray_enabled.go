//go:build tray

package tray

import (
	"fyne.io/systray"
)

// Icons as minimal 16x16 PNGs (single-color circles).
// Generated programmatically — see icondata.go for the raw bytes.

type enabledTray struct {
	statusItem *systray.MenuItem
}

// New returns a real system tray implementation.
func New() Tray {
	return &enabledTray{}
}

func (t *enabledTray) Run(onReady func(), onQuit func()) {
	systray.Run(func() {
		systray.SetIcon(iconIdle)
		systray.SetTooltip("vox — idle")

		t.statusItem = systray.AddMenuItem("Idle", "")
		t.statusItem.Disable()
		systray.AddSeparator()
		quit := systray.AddMenuItem("Quit", "Quit vox")

		go func() {
			<-quit.ClickedCh
			systray.Quit()
		}()

		if onReady != nil {
			go onReady()
		}
	}, func() {
		if onQuit != nil {
			onQuit()
		}
	})
}

func (t *enabledTray) SetState(state State) {
	switch state {
	case StateIdle:
		systray.SetIcon(iconIdle)
		systray.SetTooltip("vox — idle")
	case StateRecording:
		systray.SetIcon(iconRecording)
		systray.SetTooltip("vox — recording")
	case StateProcessing:
		systray.SetIcon(iconProcessing)
		systray.SetTooltip("vox — processing")
	}
}

func (t *enabledTray) SetStatus(text string) {
	if t.statusItem != nil {
		t.statusItem.SetTitle(text)
	}
}
