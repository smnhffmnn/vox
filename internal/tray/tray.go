package tray

// State represents the current tray icon state.
type State int

const (
	StateIdle       State = iota
	StateRecording
	StateProcessing
)

// Tray controls the system tray icon.
type Tray interface {
	// Run starts the tray. It blocks and calls onReady when initialized.
	// onReady is called on a goroutine — safe to do work there.
	Run(onReady func(), onQuit func())
	// Quit stops the tray event loop, causing Run to return. Safe to call multiple times.
	Quit()
	// SetState changes the tray icon and tooltip.
	SetState(state State)
	// SetStatus sets the status text in the menu.
	SetStatus(text string)
	// SetSettingsPort enables the "Settings" menu item that opens the UI.
	SetSettingsPort(port int)
}
