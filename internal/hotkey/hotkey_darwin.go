//go:build darwin

package hotkey

/*
#cgo CFLAGS: -x objective-c
#cgo LDFLAGS: -framework Cocoa -framework Carbon

// Implemented in hotkey_darwin.c
void voxSetTargetKeyCode(int code);
void voxStartMonitor(void);
void voxStopMonitor(void);
void voxStartEscapeMonitor(void);
void voxStopEscapeMonitor(void);
*/
import "C"

import (
	"sync"
)

var (
	mu         sync.Mutex
	onPressF   func()
	onReleaseF func()

	escapeMu  sync.Mutex
	onEscapeF func()
)

//export goHotkeyDown
func goHotkeyDown() {
	mu.Lock()
	f := onPressF
	mu.Unlock()
	if f != nil {
		go f()
	}
}

//export goHotkeyUp
func goHotkeyUp() {
	mu.Lock()
	f := onReleaseF
	mu.Unlock()
	if f != nil {
		go f()
	}
}

//export goEscapePressed
func goEscapePressed() {
	escapeMu.Lock()
	f := onEscapeF
	escapeMu.Unlock()
	if f != nil {
		go f()
	}
}

// StartEscapeMonitor registers a global Escape key listener.
func StartEscapeMonitor(onEscape func()) {
	escapeMu.Lock()
	onEscapeF = onEscape
	escapeMu.Unlock()
	C.voxStartEscapeMonitor()
}

// StopEscapeMonitor removes the global Escape key listener.
func StopEscapeMonitor() {
	C.voxStopEscapeMonitor()
	escapeMu.Lock()
	onEscapeF = nil
	escapeMu.Unlock()
}

type darwinListener struct {
	key       Key
	closeCh   chan struct{}
	closeOnce sync.Once
}

// New creates a new hotkey listener for the given key.
func New(key Key) Listener {
	return &darwinListener{
		key:     key,
		closeCh: make(chan struct{}),
	}
}

func (d *darwinListener) Listen(onPress func(), onRelease func()) error {
	mu.Lock()
	onPressF = onPress
	onReleaseF = onRelease
	mu.Unlock()

	keyCode := darwinKeyCode(d.key)
	C.voxSetTargetKeyCode(C.int(keyCode))
	C.voxStartMonitor()

	<-d.closeCh
	return nil
}

func (d *darwinListener) Close() error {
	d.closeOnce.Do(func() {
		C.voxStopMonitor()
		close(d.closeCh)
	})
	return nil
}

func darwinKeyCode(k Key) int {
	switch k {
	case RightOption, RightAlt:
		return 61
	case F13:
		return 105
	case F14:
		return 107
	case F15:
		return 113
	case F16:
		return 106
	case F17:
		return 64
	case F18:
		return 79
	case F19:
		return 80
	case F20:
		return 90
	default:
		return 61
	}
}
