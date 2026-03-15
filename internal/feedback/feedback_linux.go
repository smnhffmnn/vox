//go:build linux

package feedback

import (
	"os/exec"
	"time"
)

// PlayStart plays the recording-start sound in the background.
func PlayStart() {
	go playStartSound()
}

// PlayStop plays the recording-stop sound in the background.
func PlayStop() {
	go playStopSound()
}

func playStartSound() {
	paths := []string{
		"/usr/share/sounds/freedesktop/stereo/bell.oga",
		"/usr/share/sounds/freedesktop/stereo/message.oga",
	}
	tryPlay(paths)
}

func playStopSound() {
	paths := []string{
		"/usr/share/sounds/freedesktop/stereo/complete.oga",
		"/usr/share/sounds/freedesktop/stereo/dialog-information.oga",
	}
	tryPlay(paths)
}

// PlayHandsfreeStart plays a double beep to indicate hands-free mode activation.
func PlayHandsfreeStart() {
	go func() {
		paths := []string{
			"/usr/share/sounds/freedesktop/stereo/bell.oga",
			"/usr/share/sounds/freedesktop/stereo/message.oga",
		}
		tryPlay(paths)
		time.Sleep(150 * time.Millisecond)
		tryPlay(paths)
	}()
}

func tryPlay(paths []string) {
	for _, p := range paths {
		if err := exec.Command("paplay", p).Run(); err == nil {
			return
		}
	}
	for _, p := range paths {
		if err := exec.Command("pw-play", p).Run(); err == nil {
			return
		}
	}
}
