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
_Bool voxEscapeMonitorCanConsume(void);
void voxGetMainScreenInfo(int *x, int *y, int *width, int *height, int *menuBarHeight);
*/
import "C"

import (
	"sync"
	"time"
)

// modifierStartDelay is the window during which a modifier-hotkey press can be
// cancelled by another key press (e.g. Option+L = @ on DE layout). See mission
// card 005_tdd-modifier-aware-delay / vox-open-issues.md issue 5.
const modifierStartDelay = 50 * time.Millisecond

var (
	mu         sync.Mutex
	onPressF   func()
	onReleaseF func()

	// decider is non-nil only when the hotkey is a modifier; routes press/up/other
	// events through the modifier-aware delay guard.
	decider *startDecider

	escapeMu  sync.Mutex
	onEscapeF func()
)

//export goHotkeyDown
func goHotkeyDown() {
	mu.Lock()
	d := decider
	f := onPressF
	mu.Unlock()
	if d != nil {
		// Modifier hotkey: run through the delay guard. onStart (wired in
		// Listen) will call onPressF once the delay elapses without a cancel.
		d.hotkeyDown()
		return
	}
	if f != nil {
		go f()
	}
}

//export goHotkeyUp
func goHotkeyUp() {
	mu.Lock()
	d := decider
	f := onReleaseF
	mu.Unlock()
	if d != nil {
		_, wasStarted := d.hotkeyUp()
		if !wasStarted {
			// Release within the delay window, or no start ever fired —
			// do not propagate a release that has no matching start.
			return
		}
		if f != nil {
			go f()
		}
		return
	}
	if f != nil {
		go f()
	}
}

//export goOtherKeyDown
func goOtherKeyDown() {
	mu.Lock()
	d := decider
	mu.Unlock()
	if d != nil {
		d.otherKey()
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
// Returns true if ESC events will be consumed (not passed to the active app),
// false if running in degraded mode (listen-only, ESC leaks through) because
// Accessibility permission was not granted.
func StartEscapeMonitor(onEscape func()) bool {
	escapeMu.Lock()
	onEscapeF = onEscape
	escapeMu.Unlock()
	C.voxStartEscapeMonitor()
	// voxStartEscapeMonitor dispatches to the main queue; give it a moment to
	// create the tap before we check the result.
	time.Sleep(50 * time.Millisecond)
	return bool(C.voxEscapeMonitorCanConsume())
}

// StopEscapeMonitor removes the global Escape key listener.
func StopEscapeMonitor() {
	C.voxStopEscapeMonitor()
	escapeMu.Lock()
	onEscapeF = nil
	escapeMu.Unlock()
}

// ScreenInfo holds the visible area and menu bar height of the main screen.
type ScreenInfo struct {
	X, Y, Width, Height int
	MenuBarHeight       int // includes notch on MacBook Pro
}

func GetMainScreenInfo() ScreenInfo {
	var x, y, w, h, mb C.int
	C.voxGetMainScreenInfo(&x, &y, &w, &h, &mb)
	return ScreenInfo{X: int(x), Y: int(y), Width: int(w), Height: int(h), MenuBarHeight: int(mb)}
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
	if isModifierHotkey(d.key) {
		decider = newStartDecider(modifierStartDelay, func() {
			mu.Lock()
			f := onPressF
			mu.Unlock()
			if f != nil {
				go f()
			}
		})
	} else {
		decider = nil
	}
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
		mu.Lock()
		decider = nil
		mu.Unlock()
		close(d.closeCh)
	})
	return nil
}

// isModifierHotkey reports whether the given hotkey is a modifier key that
// can be part of a typed combo (e.g. Option for Option+L = @). Non-modifier
// hotkeys like the F13–F20 row are not at risk and skip the delay.
func isModifierHotkey(k Key) bool {
	switch k {
	case RightOption, RightAlt:
		return true
	default:
		return false
	}
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
