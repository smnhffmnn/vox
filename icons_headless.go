//go:build nogui

package main

// Headless builds have no tray — icon data is unused but the
// variables must exist because app.go references them.
var (
	trayIconIdle       []byte
	trayIconRecording  []byte
	trayIconProcessing []byte
)
