//go:build darwin

package hotkey

import (
	"sync"
	"testing"
	"time"
)

// wireModifierHotkey installs the package globals the way Listen does for a
// modifier hotkey, with a recorder capturing the delivered callback sequence.
// The returned wait function blocks until n callbacks arrived (or fails the
// test after a timeout), then returns the sequence observed so far.
func wireModifierHotkey(t *testing.T, delay time.Duration) (wait func(n int) []string) {
	t.Helper()

	var recMu sync.Mutex
	var calls []string
	record := func(name string) {
		recMu.Lock()
		calls = append(calls, name)
		recMu.Unlock()
	}

	mu.Lock()
	onPressF = func() { record("press") }
	onReleaseF = func() { record("release") }
	decider = newStartDecider(delay, func() {
		mu.Lock()
		f := onPressF
		mu.Unlock()
		if f != nil {
			go f()
		}
	})
	mu.Unlock()

	t.Cleanup(func() {
		mu.Lock()
		onPressF = nil
		onReleaseF = nil
		decider = nil
		mu.Unlock()
	})

	return func(n int) []string {
		deadline := time.Now().Add(2 * time.Second)
		for {
			recMu.Lock()
			got := append([]string(nil), calls...)
			recMu.Unlock()
			if len(got) >= n {
				return got
			}
			if time.Now().After(deadline) {
				t.Fatalf("timed out waiting for %d callbacks, got %v", n, got)
			}
			time.Sleep(time.Millisecond)
		}
	}
}

// TestGoHotkeyUp_TapDeliversPressThenRelease pins the VOX-16 contract: a bare
// tap shorter than modifierStartDelay (a human tap is 50–120ms, the delay is
// 150ms) must still arrive as press followed by release. In toggle mode the
// release is what starts and stops the recording — v0.6.0 swallowed it, so
// short taps did nothing.
func TestGoHotkeyUp_TapDeliversPressThenRelease(t *testing.T) {
	wait := wireModifierHotkey(t, 150*time.Millisecond)

	goHotkeyDown()
	time.Sleep(80 * time.Millisecond) // tap well below the 150ms window
	goHotkeyUp()

	got := wait(2)
	if len(got) != 2 || got[0] != "press" || got[1] != "release" {
		t.Fatalf("tap delivered %v, want [press release]", got)
	}
}

// TestGoHotkeyUp_ComboReleaseStaysSuppressed pins the VOX-1 contract the fix
// must not undo: Option+L (another key within the delay window) delivers
// neither press nor release — including via the release path, which in toggle
// mode would otherwise start a recording.
func TestGoHotkeyUp_ComboReleaseStaysSuppressed(t *testing.T) {
	wait := wireModifierHotkey(t, 60*time.Millisecond)

	goHotkeyDown()
	time.Sleep(10 * time.Millisecond)
	goOtherKeyDown() // the L of Option+L
	time.Sleep(10 * time.Millisecond)
	goHotkeyUp()

	time.Sleep(120 * time.Millisecond) // past the delay window: nothing may fire
	if got := wait(0); len(got) != 0 {
		t.Fatalf("combo delivered %v, want nothing", got)
	}
}

// TestGoHotkeyUp_HoldDeliversPressThenRelease: holding past the delay is the
// path that already worked — press fires when the window elapses, release on
// key-up. Pinned so the tap fix cannot regress it.
func TestGoHotkeyUp_HoldDeliversPressThenRelease(t *testing.T) {
	wait := wireModifierHotkey(t, 30*time.Millisecond)

	goHotkeyDown()
	got := wait(1) // press fires once the window elapses, before release
	if len(got) < 1 || got[0] != "press" {
		t.Fatalf("hold delivered %v before release, want press first", got)
	}
	goHotkeyUp()

	got = wait(2)
	if len(got) != 2 || got[1] != "release" {
		t.Fatalf("hold delivered %v, want [press release]", got)
	}
}
