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
		return fmt.Errorf("pbcopy failed: %w", err)
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
