//go:build windows

package inject

import (
	"fmt"
	"os/exec"
	"strings"
)

func clipboard(text string) error {
	cmd := exec.Command("powershell", "-NoProfile", "-Command", "Set-Clipboard -Value $input")
	cmd.Stdin = strings.NewReader(text)
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("Set-Clipboard failed: %w", err)
	}
	return nil
}

func wtype(text string) error {
	// On Windows, use clipboard + paste via SendKeys for reliable text injection.
	if err := clipboard(text); err != nil {
		return err
	}
	cmd := exec.Command("powershell", "-NoProfile", "-Command",
		`Add-Type -AssemblyName System.Windows.Forms; [System.Windows.Forms.SendKeys]::SendWait("^v")`)
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("SendKeys paste failed: %w", err)
	}
	return nil
}

func ydotool(text string) error {
	return wtype(text)
}
