//go:build darwin

package feedback

import (
	"os/exec"
	"time"
)

const (
	startSound  = "/System/Library/Sounds/Tink.aiff"
	stopSound   = "/System/Library/Sounds/Pop.aiff"
	cancelSound = "/System/Library/Sounds/Basso.aiff"
)

// PlayStart plays the recording-start sound in the background.
func PlayStart() {
	go exec.Command("afplay", startSound).Run()
}

// PlayStop plays the recording-stop sound in the background.
func PlayStop() {
	go exec.Command("afplay", stopSound).Run()
}

// PlayCancel plays the recording-discarded sound in the background. On a
// fullscreen space the overlay is invisible (VOX-2), so this sound can be the
// only feedback that an ESC abort actually happened.
func PlayCancel() {
	go exec.Command("afplay", cancelSound).Run()
}

// PlayHandsfreeStart plays a double beep to indicate hands-free mode activation.
func PlayHandsfreeStart() {
	go func() {
		exec.Command("afplay", startSound).Run()
		time.Sleep(150 * time.Millisecond)
		exec.Command("afplay", startSound).Run()
	}()
}
