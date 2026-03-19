//go:build windows

package feedback

import (
	"syscall"
	"time"
)

var (
	user32Win  = syscall.NewLazyDLL("user32.dll")
	messageBeep = user32Win.NewProc("MessageBeep")
)

const (
	mbOK       = 0x00000000
	mbIconInfo = 0x00000040
)

// PlayStart plays the recording-start sound in the background.
func PlayStart() {
	go messageBeep.Call(uintptr(mbOK))
}

// PlayStop plays the recording-stop sound in the background.
func PlayStop() {
	go messageBeep.Call(uintptr(mbIconInfo))
}

// PlayHandsfreeStart plays a double beep to indicate hands-free mode activation.
func PlayHandsfreeStart() {
	go func() {
		messageBeep.Call(uintptr(mbOK))
		time.Sleep(150 * time.Millisecond)
		messageBeep.Call(uintptr(mbOK))
	}()
}
