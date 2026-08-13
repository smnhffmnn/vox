//go:build darwin

package inject

/*
#cgo CFLAGS: -x objective-c
#cgo LDFLAGS: -framework Cocoa

#include <stddef.h>

// Implemented in pasteboard_darwin.c
_Bool voxWriteGeneralPasteboard(const void *bytes, size_t len);
*/
import "C"

import (
	"fmt"
	"os/exec"
	"strings"
	"unsafe"
)

func clipboard(text string) error {
	// NSPasteboard instead of pbcopy: pbcopy interprets its stdin according
	// to LC_CTYPE, and a GUI app launched from Dock or Finder inherits no
	// locale variables — UTF-8 input was read as MacRoman, so "ö" landed as
	// "√∂" on the pasteboard (VOX-15). NSString copies the bytes during the
	// call, so handing C a pointer into the Go string is safe.
	var p unsafe.Pointer
	if len(text) > 0 {
		p = unsafe.Pointer(unsafe.StringData(text))
	}
	if !bool(C.voxWriteGeneralPasteboard(p, C.size_t(len(text)))) {
		return fmt.Errorf("writing to the pasteboard failed")
	}
	return nil
}

func wtype(text string) error {
	return keystroke(text)
}

func ydotool(text string) error {
	return keystroke(text)
}

func keystroke(text string) error {
	lines := strings.Split(text, "\n")
	var parts []string
	for i, line := range lines {
		if i > 0 {
			parts = append(parts, `key code 36`) // Return
		}
		if line != "" {
			parts = append(parts, `keystroke "`+escapeAppleScript(line)+`"`)
		}
	}
	if len(parts) == 0 {
		return nil
	}
	script := `tell application "System Events"` + "\n"
	for _, p := range parts {
		script += "\t" + p + "\n"
	}
	script += `end tell`
	cmd := exec.Command("osascript", "-e", script)
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("osascript keystroke failed: %w", err)
	}
	return nil
}

func escapeAppleScript(s string) string {
	s = strings.ReplaceAll(s, `\`, `\\`)
	s = strings.ReplaceAll(s, `"`, `\"`)
	s = strings.ReplaceAll(s, "\t", `\t`)
	s = strings.ReplaceAll(s, "\r", "")
	return s
}
