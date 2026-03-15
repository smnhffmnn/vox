//go:build linux

package feedback

import (
	"os/exec"
)

// PlayStart plays the recording-start sound in the background.
func PlayStart() {
	go playSound()
}

// PlayStop plays the recording-stop sound in the background.
func PlayStop() {
	go playSound()
}

func playSound() {
	// Try freedesktop theme sounds via paplay, fall back to pw-play
	// Use bell sound as a generic notification tone
	paths := []string{
		"/usr/share/sounds/freedesktop/stereo/bell.oga",
		"/usr/share/sounds/freedesktop/stereo/message.oga",
		"/usr/share/sounds/freedesktop/stereo/complete.oga",
	}

	for _, p := range paths {
		if err := exec.Command("paplay", p).Run(); err == nil {
			return
		}
	}

	// Last resort: pw-play with same paths
	for _, p := range paths {
		if err := exec.Command("pw-play", p).Run(); err == nil {
			return
		}
	}
}
