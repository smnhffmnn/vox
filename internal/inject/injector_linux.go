//go:build linux

package inject

import (
	"fmt"
	"os/exec"
	"strings"
)

func clipboard(text string) error {
	cmd := exec.Command("wl-copy")
	cmd.Stdin = strings.NewReader(text)
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("wl-copy fehlgeschlagen (ist wl-clipboard installiert?): %w", err)
	}
	return nil
}

func wtype(text string) error {
	cmd := exec.Command("wtype", "--", text)
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("wtype fehlgeschlagen (ist wtype installiert?): %w", err)
	}
	return nil
}

func ydotool(text string) error {
	cmd := exec.Command("ydotool", "type", "--", text)
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("ydotool fehlgeschlagen (ist ydotool installiert?): %w", err)
	}
	return nil
}
