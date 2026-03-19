//go:build linux

package inject

import (
	"fmt"
	"os"
	"os/exec"
	"strings"
)

func clipboard(text string) error {
	// Try Wayland first, fall back to X11
	if os.Getenv("WAYLAND_DISPLAY") != "" {
		cmd := exec.Command("wl-copy")
		cmd.Stdin = strings.NewReader(text)
		if err := cmd.Run(); err == nil {
			return nil
		}
	}
	// X11 fallback: try xclip, then xsel
	if isAvailable("xclip") {
		cmd := exec.Command("xclip", "-selection", "clipboard")
		cmd.Stdin = strings.NewReader(text)
		if err := cmd.Run(); err == nil {
			return nil
		}
	}
	if isAvailable("xsel") {
		cmd := exec.Command("xsel", "--clipboard", "--input")
		cmd.Stdin = strings.NewReader(text)
		if err := cmd.Run(); err == nil {
			return nil
		}
	}
	// Last resort: try wl-copy anyway for better error message
	cmd := exec.Command("wl-copy")
	cmd.Stdin = strings.NewReader(text)
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("clipboard failed (install wl-clipboard, xclip, or xsel): %w", err)
	}
	return nil
}

func wtype(text string) error {
	cmd := exec.Command("wtype", "--", text)
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("wtype failed (is wtype installed?): %w", err)
	}
	return nil
}

func ydotool(text string) error {
	cmd := exec.Command("ydotool", "type", "--", text)
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("ydotool failed (is ydotool installed?): %w", err)
	}
	return nil
}

func isAvailable(name string) bool {
	_, err := exec.LookPath(name)
	return err == nil
}
