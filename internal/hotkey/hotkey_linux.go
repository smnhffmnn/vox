//go:build linux

package hotkey

import (
	"encoding/binary"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"unsafe"
)

// Linux input event constants
const (
	evKey       = 0x01
	keyPress    = 1
	keyRelease  = 0
	inputEventSize = 24 // sizeof(struct input_event) on 64-bit Linux

	// Key codes
	keyRightAlt  = 100
	keyF13       = 183
	keyF14       = 184
	keyF15       = 185
	keyF16       = 186
	keyF17       = 187
	keyF18       = 188
	keyF19       = 189
	keyF20       = 190
)

// inputEvent matches struct input_event from linux/input.h
type inputEvent struct {
	TimeSec  int64
	TimeUsec int64
	Type     uint16
	Code     uint16
	Value    int32
}

type linuxListener struct {
	key       Key
	files     []*os.File
	closeCh   chan struct{}
	closeOnce sync.Once
	mu        sync.Mutex
}

// New creates a new hotkey listener for the given key.
func New(key Key) Listener {
	return &linuxListener{
		key:     key,
		closeCh: make(chan struct{}),
	}
}

func (l *linuxListener) Listen(onPress func(), onRelease func()) error {
	keyCode := linuxKeyCode(l.key)

	// Find all keyboard input devices
	devices, err := findKeyboardDevices()
	if err != nil {
		return fmt.Errorf("find keyboard devices: %w", err)
	}
	if len(devices) == 0 {
		return fmt.Errorf("no keyboard devices found (are you in the input group?)")
	}

	var wg sync.WaitGroup

	for _, dev := range devices {
		f, err := os.Open(dev)
		if err != nil {
			continue // skip devices we can't read
		}
		l.mu.Lock()
		l.files = append(l.files, f)
		l.mu.Unlock()

		wg.Add(1)
		go func(f *os.File) {
			defer wg.Done()
			buf := make([]byte, inputEventSize)
			for {
				select {
				case <-l.closeCh:
					return
				default:
				}

				n, err := f.Read(buf)
				if err != nil || n < inputEventSize {
					return
				}

				ev := readInputEvent(buf)
				if ev.Type != evKey || ev.Code != keyCode {
					continue
				}

				switch ev.Value {
				case keyPress:
					if onPress != nil {
						go onPress()
					}
				case keyRelease:
					if onRelease != nil {
						go onRelease()
					}
				}
			}
		}(f)
	}

	// Block until closed
	<-l.closeCh
	wg.Wait()
	return nil
}

func (l *linuxListener) Close() error {
	l.closeOnce.Do(func() {
		// Close files first to unblock any goroutines stuck in f.Read()
		l.mu.Lock()
		for _, f := range l.files {
			f.Close()
		}
		l.files = nil
		l.mu.Unlock()
		// Then signal the channel so Listen() returns
		close(l.closeCh)
	})
	return nil
}

func readInputEvent(buf []byte) inputEvent {
	return inputEvent{
		TimeSec:  int64(binary.LittleEndian.Uint64(buf[0:8])),
		TimeUsec: int64(binary.LittleEndian.Uint64(buf[8:16])),
		Type:     binary.LittleEndian.Uint16(buf[16:18]),
		Code:     binary.LittleEndian.Uint16(buf[18:20]),
		Value:    int32(binary.LittleEndian.Uint32(buf[20:24])),
	}
}

func findKeyboardDevices() ([]string, error) {
	// Read /proc/bus/input/devices to find keyboards
	data, err := os.ReadFile("/proc/bus/input/devices")
	if err != nil {
		// Fallback: try all event devices
		return globEventDevices()
	}

	var devices []string
	var currentHandlers string
	isKeyboard := false

	for _, line := range strings.Split(string(data), "\n") {
		if strings.HasPrefix(line, "H: Handlers=") {
			currentHandlers = line
		}
		if strings.HasPrefix(line, "B: EV=") {
			// Check if this device supports EV_KEY (bit 1)
			evBits := strings.TrimPrefix(line, "B: EV=")
			evBits = strings.TrimSpace(evBits)
			// A keyboard typically has EV_KEY (0x01) set — check if bit 1 is set
			// Common EV values for keyboards: 120013 (includes EV_SYN, EV_KEY, EV_MSC, EV_LED, EV_REP)
			if strings.Contains(evBits, "120013") || strings.Contains(evBits, "10001") {
				isKeyboard = true
			}
		}
		if line == "" {
			if isKeyboard && currentHandlers != "" {
				for _, part := range strings.Fields(currentHandlers) {
					if strings.HasPrefix(part, "event") {
						devices = append(devices, filepath.Join("/dev/input", part))
					}
				}
			}
			isKeyboard = false
			currentHandlers = ""
		}
	}

	if len(devices) == 0 {
		return globEventDevices()
	}

	return devices, nil
}

func globEventDevices() ([]string, error) {
	matches, err := filepath.Glob("/dev/input/event*")
	if err != nil {
		return nil, err
	}
	return matches, nil
}

func linuxKeyCode(k Key) uint16 {
	switch k {
	case RightOption, RightAlt:
		return keyRightAlt
	case F13:
		return keyF13
	case F14:
		return keyF14
	case F15:
		return keyF15
	case F16:
		return keyF16
	case F17:
		return keyF17
	case F18:
		return keyF18
	case F19:
		return keyF19
	case F20:
		return keyF20
	default:
		return keyRightAlt
	}
}

// StartEscapeMonitor is a no-op on Linux (TODO: implement via evdev).
func StartEscapeMonitor(onEscape func()) {}

// StopEscapeMonitor is a no-op on Linux.
func StopEscapeMonitor() {}

// Ensure unsafe is used (needed for sizeof check)
var _ = unsafe.Sizeof(inputEvent{})
