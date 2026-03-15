//go:build linux

package notify

import (
	"os/exec"
)

// Send displays a Linux desktop notification with the given title and message.
func Send(title, message string) error {
	if len(message) > 100 {
		message = message[:97] + "..."
	}
	return exec.Command("notify-send", title, message).Run()
}
