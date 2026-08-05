package logbuf

import (
	"strings"
	"sync"
	"testing"
)

func TestRecords_NewestFirst(t *testing.T) {
	Reset()
	t.Cleanup(func() { SetSink(nil); Reset() })

	Infof(StepApp, "first")
	Warnf(StepSTT, "second")
	Errorf(StepInsert, "third")

	got := Records()
	if len(got) != 3 {
		t.Fatalf("got %d records, want 3", len(got))
	}
	want := []string{"third", "second", "first"}
	for i, w := range want {
		if got[i].Message != w {
			t.Errorf("Records()[%d].Message = %q, want %q", i, got[i].Message, w)
		}
	}
}

func TestLevelsAndSteps(t *testing.T) {
	Reset()
	t.Cleanup(func() { SetSink(nil); Reset() })

	Infof(StepRecording, "device %s", "mic")
	Warnf(StepCleanup, "degraded")
	Errorf(StepSTT, "boom %d", 401)

	got := Records()
	if got[0].Level != LevelError || got[0].Step != StepSTT || got[0].Message != "boom 401" {
		t.Errorf("error record = %+v", got[0])
	}
	if got[1].Level != LevelWarn || got[1].Step != StepCleanup {
		t.Errorf("warn record = %+v", got[1])
	}
	if got[2].Level != LevelInfo || got[2].Message != "device mic" {
		t.Errorf("info record = %+v", got[2])
	}
	if got[0].Time.IsZero() {
		t.Error("records must be timestamped")
	}
}

func TestRing_DropsOldestBeyondCap(t *testing.T) {
	Reset()
	t.Cleanup(func() { SetSink(nil); Reset() })

	for i := 0; i < maxRecords+50; i++ {
		Infof(StepApp, "msg-%d", i)
	}

	got := Records()
	if len(got) != maxRecords {
		t.Fatalf("buffer holds %d records, want the cap of %d", len(got), maxRecords)
	}
	// Newest first: the very last message must be at index 0.
	if got[0].Message != "msg-549" {
		t.Errorf("newest record = %q, want msg-549", got[0].Message)
	}
	// The oldest survivor is msg-50; everything before it was dropped.
	if got[len(got)-1].Message != "msg-50" {
		t.Errorf("oldest record = %q, want msg-50", got[len(got)-1].Message)
	}
}

func TestSink_ReceivesEveryRecord(t *testing.T) {
	Reset()
	t.Cleanup(func() { SetSink(nil); Reset() })

	var mu sync.Mutex
	var seen []Record
	SetSink(func(r Record) {
		mu.Lock()
		seen = append(seen, r)
		mu.Unlock()
	})

	Infof(StepApp, "one")
	Errorf(StepSTT, "two")

	mu.Lock()
	defer mu.Unlock()
	if len(seen) != 2 {
		t.Fatalf("sink saw %d records, want 2", len(seen))
	}
	if seen[0].Message != "one" || seen[1].Message != "two" {
		t.Errorf("sink order: got %q then %q", seen[0].Message, seen[1].Message)
	}
	if seen[1].Level != LevelError {
		t.Errorf("sink level = %q, want %q", seen[1].Level, LevelError)
	}
}

func TestSink_ClearedByNil(t *testing.T) {
	Reset()
	t.Cleanup(func() { SetSink(nil); Reset() })

	calls := 0
	SetSink(func(Record) { calls++ })
	Infof(StepApp, "counted")
	SetSink(nil)
	Infof(StepApp, "not counted")

	if calls != 1 {
		t.Errorf("sink called %d times, want 1", calls)
	}
}

func TestReset_EmptiesBuffer(t *testing.T) {
	Reset()
	t.Cleanup(func() { SetSink(nil); Reset() })

	Infof(StepApp, "gone")
	Reset()

	if got := Records(); len(got) != 0 {
		t.Errorf("after Reset: %d records, want 0", len(got))
	}
}

// Logf holds no lock while calling the sink, so concurrent writers must not
// deadlock or race. Run with -race to make this meaningful.
func TestConcurrentWrites(t *testing.T) {
	Reset()
	t.Cleanup(func() { SetSink(nil); Reset() })

	var mu sync.Mutex
	sinkCalls := 0
	SetSink(func(Record) {
		mu.Lock()
		sinkCalls++
		mu.Unlock()
	})

	var wg sync.WaitGroup
	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func(n int) {
			defer wg.Done()
			for j := 0; j < 25; j++ {
				Infof(StepApp, "goroutine-%d-%d", n, j)
			}
		}(i)
	}
	wg.Wait()

	if got := Records(); len(got) != maxRecords {
		t.Errorf("got %d records, want %d", len(got), maxRecords)
	}
	mu.Lock()
	defer mu.Unlock()
	if sinkCalls != 500 {
		t.Errorf("sink called %d times, want 500", sinkCalls)
	}
	for _, r := range Records() {
		if !strings.HasPrefix(r.Message, "goroutine-") {
			t.Fatalf("corrupted record: %q", r.Message)
		}
	}
}
