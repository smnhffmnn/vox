package history

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// setHome redirects the user home directory to a temporary directory for
// the duration of the test. Sets both HOME (Unix) and USERPROFILE (Windows)
// so tests stay hermetic on all platforms supported by os.UserHomeDir.
func setHome(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	t.Setenv("HOME", dir)
	t.Setenv("USERPROFILE", dir)
	return dir
}

func sampleEntry(text string) Entry {
	return Entry{
		Timestamp:   time.Date(2026, 4, 16, 12, 0, 0, 0, time.UTC),
		Language:    "de",
		RawText:     text,
		CleanedText: text,
		AppContext:  "test",
		DurationSec: 1.5,
		Backend:     "whisper",
	}
}

func TestNewHistory_NoFile(t *testing.T) {
	setHome(t)

	h := NewHistory(10)
	if got := h.Entries(); len(got) != 0 {
		t.Errorf("new history with no file: got %d entries, want 0", len(got))
	}
}

func TestAdd_GrowsEntries(t *testing.T) {
	setHome(t)

	h := NewHistory(10)
	if err := h.Add(sampleEntry("one")); err != nil {
		t.Fatalf("Add(one): %v", err)
	}
	if err := h.Add(sampleEntry("two")); err != nil {
		t.Fatalf("Add(two): %v", err)
	}

	entries := h.Entries()
	if len(entries) != 2 {
		t.Fatalf("got %d entries, want 2", len(entries))
	}
	// Entries() is newest-first.
	if entries[0].RawText != "two" || entries[1].RawText != "one" {
		t.Errorf("order: got [%q, %q], want [two, one]",
			entries[0].RawText, entries[1].RawText)
	}
}

func TestEntries_NewestFirst(t *testing.T) {
	setHome(t)

	h := NewHistory(10)
	for _, text := range []string{"a", "b", "c", "d"} {
		if err := h.Add(sampleEntry(text)); err != nil {
			t.Fatalf("Add(%q): %v", text, err)
		}
	}

	got := h.Entries()
	want := []string{"d", "c", "b", "a"}
	if len(got) != len(want) {
		t.Fatalf("got %d entries, want %d", len(got), len(want))
	}
	for i, w := range want {
		if got[i].RawText != w {
			t.Errorf("Entries()[%d] = %q, want %q", i, got[i].RawText, w)
		}
	}
}

func TestEntries_ReturnsCopy(t *testing.T) {
	setHome(t)

	h := NewHistory(10)
	if err := h.Add(sampleEntry("orig")); err != nil {
		t.Fatalf("Add: %v", err)
	}

	got := h.Entries()
	got[0].RawText = "mutated"

	again := h.Entries()
	if again[0].RawText != "orig" {
		t.Errorf("Entries() should return a copy; got mutation: %q", again[0].RawText)
	}
}

func TestRoundtrip_PersistsAcrossInstances(t *testing.T) {
	setHome(t)

	entry := sampleEntry("persisted")
	h1 := NewHistory(10)
	if err := h1.Add(entry); err != nil {
		t.Fatalf("Add: %v", err)
	}

	h2 := NewHistory(10)
	got := h2.Entries()
	if len(got) != 1 {
		t.Fatalf("got %d entries, want 1", len(got))
	}
	g := got[0]
	if g.RawText != entry.RawText ||
		g.Language != entry.Language ||
		g.CleanedText != entry.CleanedText ||
		g.AppContext != entry.AppContext ||
		g.Backend != entry.Backend ||
		g.DurationSec != entry.DurationSec {
		t.Errorf("entry did not round-trip: got %+v, want %+v", g, entry)
	}
	if !g.Timestamp.Equal(entry.Timestamp) {
		t.Errorf("timestamp round-trip: got %v, want %v", g.Timestamp, entry.Timestamp)
	}
}

func TestRotation_TruncatesInMemoryAndOnDisk(t *testing.T) {
	home := setHome(t)

	h := NewHistory(3)
	for _, text := range []string{"a", "b", "c", "d", "e"} {
		if err := h.Add(sampleEntry(text)); err != nil {
			t.Fatalf("Add(%q): %v", text, err)
		}
	}

	// In-memory state: last 3, newest-first.
	got := h.Entries()
	want := []string{"e", "d", "c"}
	if len(got) != len(want) {
		t.Fatalf("got %d entries, want %d", len(got), len(want))
	}
	for i, w := range want {
		if got[i].RawText != w {
			t.Errorf("Entries()[%d] = %q, want %q", i, got[i].RawText, w)
		}
	}

	// File on disk must have been rewritten, not appended forever.
	data, err := os.ReadFile(filepath.Join(home, ".config", "vox", "history.jsonl"))
	if err != nil {
		t.Fatalf("reading history file: %v", err)
	}
	lines := strings.Split(strings.TrimRight(string(data), "\n"), "\n")
	if len(lines) != 3 {
		t.Errorf("history file has %d lines, want 3", len(lines))
	}

	// Fresh instance must see the same rotated state.
	h2 := NewHistory(10)
	got2 := h2.Entries()
	if len(got2) != 3 {
		t.Fatalf("after reload, got %d entries, want 3", len(got2))
	}
	for i, w := range want {
		if got2[i].RawText != w {
			t.Errorf("after reload: Entries()[%d] = %q, want %q", i, got2[i].RawText, w)
		}
	}
}

func TestLoad_SkipsInvalidLines(t *testing.T) {
	home := setHome(t)

	path := filepath.Join(home, ".config", "vox", "history.jsonl")
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	content := `{"raw_text":"valid1","language":"de"}
not-json
{"raw_text":"valid2","language":"en"}
{malformed
`
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}

	h := NewHistory(10)
	got := h.Entries()
	if len(got) != 2 {
		t.Fatalf("got %d entries, want 2 (invalid lines must be skipped silently)", len(got))
	}
	// Newest-first after load.
	if got[0].RawText != "valid2" || got[1].RawText != "valid1" {
		t.Errorf("order: got [%q, %q], want [valid2, valid1]",
			got[0].RawText, got[1].RawText)
	}
}

func TestLoad_TrimsToMaxSize(t *testing.T) {
	home := setHome(t)

	path := filepath.Join(home, ".config", "vox", "history.jsonl")
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	var b strings.Builder
	for _, text := range []string{"a", "b", "c", "d", "e"} {
		b.WriteString(`{"raw_text":"` + text + `"}` + "\n")
	}
	if err := os.WriteFile(path, []byte(b.String()), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}

	h := NewHistory(2)
	got := h.Entries()
	if len(got) != 2 {
		t.Fatalf("got %d entries, want 2", len(got))
	}
	// load() keeps the last N; Entries() reverses → [e, d].
	if got[0].RawText != "e" || got[1].RawText != "d" {
		t.Errorf("order: got [%q, %q], want [e, d]", got[0].RawText, got[1].RawText)
	}
}
