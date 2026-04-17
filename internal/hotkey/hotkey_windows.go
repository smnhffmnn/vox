//go:build windows

package hotkey

import (
	"sync"
	"syscall"
	"unsafe"
)

var (
	user32                  = syscall.NewLazyDLL("user32.dll")
	kernel32                = syscall.NewLazyDLL("kernel32.dll")
	ntdll                   = syscall.NewLazyDLL("ntdll.dll")
	procSetWindowsHookEx    = user32.NewProc("SetWindowsHookExW")
	procCallNextHookEx      = user32.NewProc("CallNextHookEx")
	procUnhookWindowsHookEx = user32.NewProc("UnhookWindowsHookEx")
	procGetMessage          = user32.NewProc("GetMessageW")
	procPostThreadMessage   = user32.NewProc("PostThreadMessageW")
	procGetCurrentThreadId  = kernel32.NewProc("GetCurrentThreadId")
	procRtlMoveMemory       = ntdll.NewProc("RtlMoveMemory")
)

const (
	whKeyboardLL = 13
	wmKeydown    = 0x0100
	wmKeyup      = 0x0101
	wmSyskeydown = 0x0104
	wmSyskeyup   = 0x0105
	wmQuit       = 0x0012

	vkRightAlt = 0xA5 // VK_RMENU
	vkF13      = 0x7C
	vkF14      = 0x7D
	vkF15      = 0x7E
	vkF16      = 0x7F
	vkF17      = 0x80
	vkF18      = 0x81
	vkF19      = 0x82
	vkF20      = 0x83
)

type kbdllHookStruct struct {
	VkCode      uint32
	ScanCode    uint32
	Flags       uint32
	Time        uint32
	DwExtraInfo uintptr
}

type windowsMsg struct {
	Hwnd    uintptr
	Message uint32
	WParam  uintptr
	LParam  uintptr
	Time    uint32
	Pt      struct{ X, Y int32 }
}

type windowsListener struct {
	key       Key
	hook      uintptr
	threadID  uint32
	closeCh   chan struct{}
	closeOnce sync.Once
	onPress   func()
	onRelease func()
	pressed   bool // guards against key-repeat firing multiple onPress
}

// New creates a new hotkey listener for the given key.
func New(key Key) Listener {
	return &windowsListener{
		key:     key,
		closeCh: make(chan struct{}),
	}
}

func (w *windowsListener) Listen(onPress func(), onRelease func()) error {
	w.onPress = onPress
	w.onRelease = onRelease

	tid, _, _ := procGetCurrentThreadId.Call()
	w.threadID = uint32(tid)

	targetVK := windowsKeyCode(w.key)

	hookProc := syscall.NewCallback(func(nCode int, wParam uintptr, lParam uintptr) uintptr {
		if nCode >= 0 {
			vkCode := readVkCode(lParam)
			if vkCode == targetVK {
				switch wParam {
				case wmKeydown, wmSyskeydown:
					if !w.pressed {
						w.pressed = true
						if w.onPress != nil {
							go w.onPress()
						}
					}
				case wmKeyup, wmSyskeyup:
					w.pressed = false
					if w.onRelease != nil {
						go w.onRelease()
					}
				}
			}
		}
		ret, _, _ := procCallNextHookEx.Call(0, uintptr(nCode), wParam, lParam)
		return ret
	})

	hook, _, err := procSetWindowsHookEx.Call(whKeyboardLL, hookProc, 0, 0)
	if hook == 0 {
		return err
	}
	w.hook = hook

	// Message loop (required for low-level keyboard hooks)
	var m windowsMsg
	for {
		ret, _, _ := procGetMessage.Call(uintptr(unsafe.Pointer(&m)), 0, 0, 0)
		if ret == 0 || int32(ret) == -1 {
			break
		}
	}

	return nil
}

func (w *windowsListener) Close() error {
	w.closeOnce.Do(func() {
		if w.hook != 0 {
			procUnhookWindowsHookEx.Call(w.hook)
		}
		// Post WM_QUIT to break the message loop
		procPostThreadMessage.Call(uintptr(w.threadID), wmQuit, 0, 0)
		close(w.closeCh)
	})
	return nil
}

// readVkCode reads the VkCode (first uint32) from a KBDLLHOOKSTRUCT at the
// address passed as lParam by the OS in a low-level keyboard hook callback.
// Uses RtlMoveMemory to copy the data without a direct uintptr→unsafe.Pointer
// conversion, keeping go vet clean.
func readVkCode(lParam uintptr) uint32 {
	var vkCode uint32
	procRtlMoveMemory.Call(
		uintptr(unsafe.Pointer(&vkCode)),
		lParam,
		unsafe.Sizeof(vkCode),
	)
	return vkCode
}

// StartEscapeMonitor is a no-op on Windows (TODO: implement via keyboard hook).
// Always returns false (cannot consume ESC).
func StartEscapeMonitor(onEscape func()) bool { return false }

// StopEscapeMonitor is a no-op on Windows.
func StopEscapeMonitor() {}

// ScreenInfo holds the visible area and menu bar height of the main screen.
type ScreenInfo struct {
	X, Y, Width, Height int
	MenuBarHeight       int
}

// GetMainScreenInfo returns a default screen size on Windows.
func GetMainScreenInfo() ScreenInfo {
	return ScreenInfo{X: 0, Y: 0, Width: 1920, Height: 1080, MenuBarHeight: 0}
}

func windowsKeyCode(k Key) uint32 {
	switch k {
	case RightOption, RightAlt:
		return vkRightAlt
	case F13:
		return vkF13
	case F14:
		return vkF14
	case F15:
		return vkF15
	case F16:
		return vkF16
	case F17:
		return vkF17
	case F18:
		return vkF18
	case F19:
		return vkF19
	case F20:
		return vkF20
	default:
		return vkRightAlt
	}
}
