//go:build linux

package permissions

import (
	"os"
	"os/exec"
	"os/user"
	"strings"
)

func checkPlatform() Status {
	return Status{
		Accessibility: checkInputGroup(),
		Microphone:    true, // Linux doesn't gate microphone access via permissions
	}
}

// checkInputGroup checks if the current user is in the "input" group (needed for evdev hotkey).
func checkInputGroup() bool {
	u, err := user.Current()
	if err != nil {
		return false
	}
	// Root always has access
	if u.Uid == "0" {
		return true
	}
	// Check if user is in "input" group
	out, err := exec.Command("groups", u.Username).Output()
	if err != nil {
		return false
	}
	for _, g := range strings.Fields(string(out)) {
		if g == "input" {
			return true
		}
	}
	// Also check if /dev/input/event0 is readable
	f, err := os.Open("/dev/input/event0")
	if err == nil {
		f.Close()
		return true
	}
	return false
}

func openAccessibilitySettings() {
	// No standard way to open input group settings on Linux
}

func openMicrophoneSettings() {
	// No standard way to open audio settings on Linux
}
