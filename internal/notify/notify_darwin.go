//go:build darwin

package notify

import (
	"os/exec"
)

// Send displays a macOS notification with the given title and message.
func Send(title, message string) error {
	if runes := []rune(message); len(runes) > 100 {
		message = string(runes[:97]) + "..."
	}
	script := `display notification "` + escapeAppleScript(message) + `" with title "` + escapeAppleScript(title) + `"`
	return exec.Command("osascript", "-e", script).Run()
}

func escapeAppleScript(s string) string {
	var out []byte
	for i := 0; i < len(s); i++ {
		switch s[i] {
		case '\\':
			out = append(out, '\\', '\\')
		case '"':
			out = append(out, '\\', '"')
		default:
			out = append(out, s[i])
		}
	}
	return string(out)
}
