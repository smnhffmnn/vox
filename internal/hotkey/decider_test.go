package hotkey

import (
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestShouldStart(t *testing.T) {
	const delay = 50 * time.Millisecond
	ms := time.Millisecond

	tests := []struct {
		name   string
		events []Event
		want   bool
	}{
		{
			name:   "empty events do not start",
			events: nil,
			want:   false,
		},
		{
			name: "first event not HotkeyDown does not start",
			events: []Event{
				{T: 0, Kind: OtherKey},
			},
			want: false,
		},
		{
			name: "modifier alone, no further events within delay, starts",
			events: []Event{
				{T: 0, Kind: HotkeyDown},
			},
			want: true,
		},
		{
			name: "modifier alone, other key AFTER delay, still starts",
			events: []Event{
				{T: 0, Kind: HotkeyDown},
				{T: 60 * ms, Kind: OtherKey},
			},
			want: true,
		},
		{
			name: "modifier + other key within delay cancels (Option+L = @ scenario)",
			events: []Event{
				{T: 0, Kind: HotkeyDown},
				{T: 10 * ms, Kind: OtherKey},
			},
			want: false,
		},
		{
			name: "modifier released within delay cancels (short tap)",
			events: []Event{
				{T: 0, Kind: HotkeyDown},
				{T: 20 * ms, Kind: HotkeyUp},
			},
			want: false,
		},
		{
			name: "auto-repeat HotkeyDown does not cancel",
			events: []Event{
				{T: 0, Kind: HotkeyDown},
				{T: 10 * ms, Kind: HotkeyDown},
				{T: 20 * ms, Kind: HotkeyDown},
			},
			want: true,
		},
		{
			name: "auto-repeat followed by other key still cancels",
			events: []Event{
				{T: 0, Kind: HotkeyDown},
				{T: 5 * ms, Kind: HotkeyDown},
				{T: 15 * ms, Kind: OtherKey},
			},
			want: false,
		},
		{
			name: "other key exactly at delay boundary is outside window",
			events: []Event{
				{T: 0, Kind: HotkeyDown},
				{T: delay, Kind: OtherKey},
			},
			want: true,
		},
		{
			name: "other key just before boundary cancels",
			events: []Event{
				{T: 0, Kind: HotkeyDown},
				{T: delay - ms, Kind: OtherKey},
			},
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := ShouldStart(tt.events, delay); got != tt.want {
				t.Errorf("ShouldStart = %v, want %v", got, tt.want)
			}
		})
	}
}

// TestStartDecider_StartsAfterDelay: modifier alone, no other keys within delay.
func TestStartDecider_StartsAfterDelay(t *testing.T) {
	var started int32
	d := newStartDecider(20*time.Millisecond, func() {
		atomic.AddInt32(&started, 1)
	})
	d.hotkeyDown()
	time.Sleep(80 * time.Millisecond)
	if got := atomic.LoadInt32(&started); got != 1 {
		t.Fatalf("onStart fired %d times, want 1", got)
	}
}

// TestStartDecider_OtherKeyCancels: a non-hotkey press within delay cancels start.
func TestStartDecider_OtherKeyCancels(t *testing.T) {
	var started int32
	d := newStartDecider(50*time.Millisecond, func() {
		atomic.AddInt32(&started, 1)
	})
	d.hotkeyDown()
	time.Sleep(5 * time.Millisecond)
	if !d.otherKey() {
		t.Fatalf("otherKey during pending: got false, want true")
	}
	time.Sleep(100 * time.Millisecond)
	if got := atomic.LoadInt32(&started); got != 0 {
		t.Fatalf("onStart fired %d times, want 0", got)
	}
}

// TestStartDecider_ReleaseWithinDelay: release within delay cancels start.
func TestStartDecider_ReleaseWithinDelay(t *testing.T) {
	var started int32
	d := newStartDecider(50*time.Millisecond, func() {
		atomic.AddInt32(&started, 1)
	})
	d.hotkeyDown()
	time.Sleep(5 * time.Millisecond)
	cancelled, wasStarted := d.hotkeyUp()
	if !cancelled {
		t.Errorf("cancelled = false, want true (release during delay)")
	}
	if wasStarted {
		t.Errorf("wasStarted = true, want false (start never fired)")
	}
	time.Sleep(100 * time.Millisecond)
	if got := atomic.LoadInt32(&started); got != 0 {
		t.Fatalf("onStart fired %d times, want 0", got)
	}
}

// TestStartDecider_ReleaseAfterStart: release after delay expired triggers release.
func TestStartDecider_ReleaseAfterStart(t *testing.T) {
	var started int32
	d := newStartDecider(10*time.Millisecond, func() {
		atomic.AddInt32(&started, 1)
	})
	d.hotkeyDown()
	time.Sleep(40 * time.Millisecond)
	if got := atomic.LoadInt32(&started); got != 1 {
		t.Fatalf("onStart fired %d times before release, want 1", got)
	}
	cancelled, wasStarted := d.hotkeyUp()
	if cancelled {
		t.Errorf("cancelled = true, want false (delay already expired)")
	}
	if !wasStarted {
		t.Errorf("wasStarted = false, want true (start had fired)")
	}
}

// TestStartDecider_AutoRepeatIgnored: multiple hotkeyDown events start the delay only once.
func TestStartDecider_AutoRepeatIgnored(t *testing.T) {
	var started int32
	d := newStartDecider(20*time.Millisecond, func() {
		atomic.AddInt32(&started, 1)
	})
	d.hotkeyDown()
	for i := 0; i < 5; i++ {
		time.Sleep(2 * time.Millisecond)
		d.hotkeyDown() // simulated auto-repeat while modifier held
	}
	time.Sleep(80 * time.Millisecond)
	if got := atomic.LoadInt32(&started); got != 1 {
		t.Fatalf("onStart fired %d times despite auto-repeat, want 1", got)
	}
}

// TestStartDecider_AutoRepeatThenOtherKeyCancels: auto-repeat does not protect
// against a genuine other-key cancel within the delay window.
func TestStartDecider_AutoRepeatThenOtherKeyCancels(t *testing.T) {
	var started int32
	d := newStartDecider(50*time.Millisecond, func() {
		atomic.AddInt32(&started, 1)
	})
	d.hotkeyDown()
	time.Sleep(5 * time.Millisecond)
	d.hotkeyDown() // auto-repeat
	time.Sleep(5 * time.Millisecond)
	if !d.otherKey() {
		t.Fatalf("otherKey after auto-repeat: got false, want true")
	}
	time.Sleep(80 * time.Millisecond)
	if got := atomic.LoadInt32(&started); got != 0 {
		t.Fatalf("onStart fired %d times, want 0", got)
	}
}

// TestStartDecider_SpuriousUpIgnored: hotkeyUp with no prior down is a no-op.
func TestStartDecider_SpuriousUpIgnored(t *testing.T) {
	d := newStartDecider(10*time.Millisecond, func() {})
	cancelled, wasStarted := d.hotkeyUp()
	if cancelled || wasStarted {
		t.Fatalf("hotkeyUp with no prior down: got (cancelled=%v, wasStarted=%v), want (false, false)", cancelled, wasStarted)
	}
}

// TestStartDecider_RestartAfterRelease: decider is reusable across press cycles.
func TestStartDecider_RestartAfterRelease(t *testing.T) {
	var started int32
	d := newStartDecider(10*time.Millisecond, func() {
		atomic.AddInt32(&started, 1)
	})
	// Cycle 1: press and release after start fires.
	d.hotkeyDown()
	time.Sleep(40 * time.Millisecond)
	d.hotkeyUp()
	// Cycle 2: should work fresh.
	d.hotkeyDown()
	time.Sleep(40 * time.Millisecond)
	d.hotkeyUp()
	if got := atomic.LoadInt32(&started); got != 2 {
		t.Fatalf("onStart fired %d times across 2 cycles, want 2", got)
	}
}

// TestStartDecider_ConcurrentSafe: basic -race smoke test for concurrent event feeds.
func TestStartDecider_ConcurrentSafe(t *testing.T) {
	d := newStartDecider(5*time.Millisecond, func() {})
	var wg sync.WaitGroup
	for i := 0; i < 50; i++ {
		wg.Add(3)
		go func() { defer wg.Done(); d.hotkeyDown() }()
		go func() { defer wg.Done(); d.otherKey() }()
		go func() { defer wg.Done(); d.hotkeyUp() }()
	}
	wg.Wait()
	time.Sleep(20 * time.Millisecond) // let any pending timers fire
}
