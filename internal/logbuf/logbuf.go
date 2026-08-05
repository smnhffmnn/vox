// Package logbuf is an in-memory diagnostic log with a UI-facing read side.
//
// vox runs as a desktop app started from Finder or Homebrew, where nothing
// reads stderr. Every diagnostic therefore goes through this buffer as well,
// so the app can show the user why something failed without a terminal.
package logbuf

import (
	"fmt"
	"os"
	"sync"
	"time"
)

// Level classifies a record.
const (
	LevelInfo  = "info"
	LevelWarn  = "warn"
	LevelError = "error"
)

// Pipeline steps, used to tell the user where in the chain a failure happened.
const (
	StepApp       = "app"
	StepRecording = "recording"
	StepSTT       = "stt"
	StepCleanup   = "cleanup"
	StepInsert    = "insert"
	StepHistory   = "history"
)

// Record is one diagnostic entry.
type Record struct {
	Time    time.Time `json:"time"`
	Level   string    `json:"level"`
	Step    string    `json:"step"`
	Message string    `json:"message"`
}

// maxRecords bounds memory use. Roughly a day of heavy dictation.
const maxRecords = 500

var (
	mu      sync.Mutex
	records []Record
	sink    func(Record)
)

// SetSink registers a callback invoked for every record after it is stored.
// The app uses it to push errors to the UI. Called synchronously, so the
// callback must not block or log.
func SetSink(fn func(Record)) {
	mu.Lock()
	sink = fn
	mu.Unlock()
}

// logf stores a record and mirrors it to stderr, which keeps existing terminal
// and headless workflows working unchanged.
func logf(level, step, format string, args ...any) {
	rec := Record{
		Time:    time.Now(),
		Level:   level,
		Step:    step,
		Message: fmt.Sprintf(format, args...),
	}

	mu.Lock()
	records = append(records, rec)
	if len(records) > maxRecords {
		records = records[len(records)-maxRecords:]
	}
	fn := sink
	mu.Unlock()

	fmt.Fprintf(os.Stderr, "vox: [%s/%s] %s\n", rec.Level, rec.Step, rec.Message)

	if fn != nil {
		fn(rec)
	}
}

// Infof records an informational message.
func Infof(step, format string, args ...any) { logf(LevelInfo, step, format, args...) }

// Warnf records a recoverable problem.
func Warnf(step, format string, args ...any) { logf(LevelWarn, step, format, args...) }

// Errorf records a failure the user is likely to notice.
func Errorf(step, format string, args ...any) { logf(LevelError, step, format, args...) }

// Records returns a copy of the buffer, newest first.
func Records() []Record {
	mu.Lock()
	defer mu.Unlock()

	out := make([]Record, len(records))
	for i, r := range records {
		out[len(records)-1-i] = r
	}
	return out
}

// Reset clears the buffer. Used by tests and by the UI's clear action.
func Reset() {
	mu.Lock()
	records = nil
	mu.Unlock()
}
