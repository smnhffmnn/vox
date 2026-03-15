//go:build darwin

package feedback

import (
	"os/exec"
	"time"
)

const (
	startSound = "/System/Library/Sounds/Tink.aiff"
	stopSound  = "/System/Library/Sounds/Pop.aiff"
)

// PlayStart plays the recording-start sound in the background.
func PlayStart() {
	go exec.Command("afplay", startSound).Run()
}

// PlayStop plays the recording-stop sound in the background.
func PlayStop() {
	go exec.Command("afplay", stopSound).Run()
}

// PlayHandsfreeStart plays a double beep to indicate hands-free mode activation.
func PlayHandsfreeStart() {
	go func() {
		exec.Command("afplay", startSound).Run()
		time.Sleep(150 * time.Millisecond)
		exec.Command("afplay", startSound).Run()
	}()
}
