//go:build darwin

package inject

import (
	"fmt"
	"os/exec"
	"strings"
)

func clipboard(text string) error {
	cmd := exec.Command("pbcopy")
	cmd.Stdin = strings.NewReader(text)
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("pbcopy fehlgeschlagen: %w", err)
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
	script := `tell application "System Events" to keystroke "` + escapeAppleScript(text) + `"`
	cmd := exec.Command("osascript", "-e", script)
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("osascript keystroke fehlgeschlagen: %w", err)
	}
	return nil
}

func escapeAppleScript(s string) string {
	s = strings.ReplaceAll(s, `\`, `\\`)
	s = strings.ReplaceAll(s, `"`, `\"`)
	return s
}
