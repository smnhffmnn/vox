//go:build darwin

package feedback

import (
	"os/exec"
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
