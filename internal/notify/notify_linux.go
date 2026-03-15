//go:build linux

package notify

import (
	"os/exec"
)

// Send displays a Linux desktop notification with the given title and message.
func Send(title, message string) error {
	if runes := []rune(message); len(runes) > 100 {
		message = string(runes[:97]) + "..."
	}
	return exec.Command("notify-send", title, message).Run()
}
