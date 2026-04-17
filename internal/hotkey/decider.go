package hotkey

import (
	"sync"
	"time"
)

// EventKind identifies the kind of key event relevant to the start decision.
type EventKind int

const (
	// HotkeyDown: the configured hotkey was pressed (initial press or auto-repeat).
	HotkeyDown EventKind = iota
	// HotkeyUp: the configured hotkey was released.
	HotkeyUp
	// OtherKey: any non-hotkey key was pressed.
	OtherKey
)

// Event is a single timestamped key event observed within the start-decision window.
type Event struct {
	T    time.Duration // relative to an arbitrary origin; monotonically non-decreasing
	Kind EventKind
}

// ShouldStart decides whether a recording should start given the sequence of
// events observed within `delay` after the initial event.
//
// Returns true only if the first event is a HotkeyDown and no HotkeyUp or
// OtherKey event falls within [first.T, first.T+delay). Auto-repeat HotkeyDown
// events from the same hotkey are ignored. Events at or after first.T+delay
// are considered outside the window.
//
// This guards modifier hotkeys (e.g. right Option) against accidental start
// when the user actually wants to type a modifier-combo such as Option+L
// (which is "@" on the German keyboard layout).
func ShouldStart(events []Event, delay time.Duration) bool {
	if len(events) == 0 || events[0].Kind != HotkeyDown {
		return false
	}
	start := events[0].T
	for i := 1; i < len(events); i++ {
		e := events[i]
		if e.T-start >= delay {
			break
		}
		switch e.Kind {
		case HotkeyUp, OtherKey:
			return false
		}
	}
	return true
}

// startDecider applies the modifier-aware delay over a live event stream.
// It fires onStart only if, after delay, no other key was pressed and the
// hotkey was not released. Safe for concurrent use.
//
// Currently wired only in hotkey_darwin.go. Linux/Windows may share the same
// problem (modifier-key hotkey triggering on modifier-combo presses); tracking
// as follow-up — the decider itself is platform-independent and can be reused.
type startDecider struct {
	delay   time.Duration
	onStart func()

	mu      sync.Mutex
	pending bool // delay window active, start not yet fired
	started bool // onStart has fired, waiting for release
	timer   *time.Timer
}

func newStartDecider(delay time.Duration, onStart func()) *startDecider {
	return &startDecider{delay: delay, onStart: onStart}
}

// hotkeyDown is called when the target hotkey is pressed. Auto-repeat events
// while pending or already started are ignored — only the initial down starts
// a delay window.
func (s *startDecider) hotkeyDown() {
	s.mu.Lock()
	if s.pending || s.started {
		s.mu.Unlock()
		return
	}
	s.pending = true
	s.timer = time.AfterFunc(s.delay, s.fire)
	s.mu.Unlock()
}

func (s *startDecider) fire() {
	s.mu.Lock()
	if !s.pending {
		s.mu.Unlock()
		return
	}
	s.pending = false
	s.started = true
	s.timer = nil
	cb := s.onStart
	s.mu.Unlock()
	if cb != nil {
		cb()
	}
}

// hotkeyUp is called when the target hotkey is released.
//   - cancelled=true: a pending start was cancelled (release came within delay).
//     The caller should NOT propagate a release event — no start ever fired.
//   - wasStarted=true: a prior start had fired. The caller SHOULD propagate the
//     release event.
//   - both false: spurious release (no prior down). No-op.
func (s *startDecider) hotkeyUp() (cancelled bool, wasStarted bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.pending {
		s.pending = false
		if s.timer != nil {
			s.timer.Stop()
			s.timer = nil
		}
		return true, false
	}
	if s.started {
		s.started = false
		return false, true
	}
	return false, false
}

// otherKey is called when any non-target key is pressed. If a start is pending,
// it is cancelled. Returns true iff a pending start was cancelled.
func (s *startDecider) otherKey() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	if !s.pending {
		return false
	}
	s.pending = false
	if s.timer != nil {
		s.timer.Stop()
		s.timer = nil
	}
	return true
}
