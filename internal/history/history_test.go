package history

import (
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
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

	h := NewHistory(10, 10)
	if got := h.Entries(); len(got) != 0 {
		t.Errorf("new history with no file: got %d entries, want 0", len(got))
	}
}

func TestAdd_GrowsEntries(t *testing.T) {
	setHome(t)

	h := NewHistory(10, 10)
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

	h := NewHistory(10, 10)
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

	h := NewHistory(10, 10)
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
	h1 := NewHistory(10, 10)
	if err := h1.Add(entry); err != nil {
		t.Fatalf("Add: %v", err)
	}

	h2 := NewHistory(10, 10)
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

	h := NewHistory(3, 10)
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
	h2 := NewHistory(10, 10)
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

	h := NewHistory(10, 10)
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

	h := NewHistory(2, 10)
	got := h.Entries()
	if len(got) != 2 {
		t.Fatalf("got %d entries, want 2", len(got))
	}
	// load() keeps the last N; Entries() reverses → [e, d].
	if got[0].RawText != "e" || got[1].RawText != "d" {
		t.Errorf("order: got [%q, %q], want [e, d]", got[0].RawText, got[1].RawText)
	}
}

// --- audio retention, failure bookkeeping, updates ---

// writeAudio creates a dummy recording for the given entry id and returns its path.
func writeAudio(t *testing.T, h *History, id string) string {
	t.Helper()
	dir, err := h.audioDirEnsured()
	if err != nil {
		t.Fatalf("AudioDir: %v", err)
	}
	path := filepath.Join(dir, audioFileName(id))
	if err := os.WriteFile(path, []byte("RIFFdummy"), 0o644); err != nil {
		t.Fatalf("writing audio: %v", err)
	}
	return path
}

func entryWithAudio(id, text string) Entry {
	e := sampleEntry(text)
	e.ID = id
	e.AudioFile = audioFileName(id)
	return e
}

func TestAdd_AssignsIDAndDefaultStatus(t *testing.T) {
	setHome(t)

	h := NewHistory(10, 10)
	if err := h.Add(sampleEntry("one")); err != nil {
		t.Fatalf("Add: %v", err)
	}

	got := h.Entries()[0]
	if got.ID == "" {
		t.Error("Add must assign an id when the caller left it empty")
	}
	if got.Status != StatusOK {
		t.Errorf("status = %q, want %q", got.Status, StatusOK)
	}
	if got.Failed() {
		t.Error("a default entry must not count as failed")
	}
}

func TestAdd_PrunesAudioBeyondKeep(t *testing.T) {
	setHome(t)

	h := NewHistory(10, 2)

	var paths []string
	for _, id := range []string{"a", "b", "c", "d"} {
		paths = append(paths, writeAudio(t, h, id))
		if err := h.Add(entryWithAudio(id, id)); err != nil {
			t.Fatalf("Add(%q): %v", id, err)
		}
	}

	// Newest two keep their audio, older ones lose it.
	for i, id := range []string{"a", "b", "c", "d"} {
		_, err := os.Stat(paths[i])
		wantGone := i < 2
		if wantGone && err == nil {
			t.Errorf("audio for %q should have been pruned", id)
		}
		if !wantGone && err != nil {
			t.Errorf("audio for %q should have been kept: %v", id, err)
		}
	}

	// The text of the pruned entries survives — that is the whole point of
	// retaining text and audio independently.
	if len(h.Entries()) != 4 {
		t.Errorf("got %d entries, want 4 (text must outlive audio)", len(h.Entries()))
	}
}

func TestAdd_KeepsNoAudioWhenKeepIsZero(t *testing.T) {
	setHome(t)

	h := NewHistory(10, 0)
	path := writeAudio(t, h, "only")
	if err := h.Add(entryWithAudio("only", "only")); err != nil {
		t.Fatalf("Add: %v", err)
	}

	if _, err := os.Stat(path); err == nil {
		t.Error("audio_keep=0 must not retain any recording")
	}
}

// A recording that is still being transcribed has no history entry yet.
// Pruning must never touch it, or the failure this retention scheme exists to
// prevent would be caused by the retention itself.
func TestPruneAudio_LeavesUnreferencedFilesAlone(t *testing.T) {
	setHome(t)

	h := NewHistory(10, 1)
	inFlight := writeAudio(t, h, "in-flight")

	for _, id := range []string{"a", "b", "c"} {
		writeAudio(t, h, id)
		if err := h.Add(entryWithAudio(id, id)); err != nil {
			t.Fatalf("Add(%q): %v", id, err)
		}
	}

	if _, err := os.Stat(inFlight); err != nil {
		t.Errorf("unreferenced in-flight recording must survive pruning: %v", err)
	}
}

func TestCleanOrphans_RemovesOldUnreferencedOnly(t *testing.T) {
	setHome(t)

	h := NewHistory(10, 10)

	referenced := writeAudio(t, h, "kept")
	if err := h.Add(entryWithAudio("kept", "kept")); err != nil {
		t.Fatalf("Add: %v", err)
	}

	fresh := writeAudio(t, h, "fresh-orphan")
	stale := writeAudio(t, h, "stale-orphan")
	old := time.Now().Add(-2 * orphanGrace)
	if err := os.Chtimes(stale, old, old); err != nil {
		t.Fatalf("chtimes: %v", err)
	}

	h.CleanOrphans()

	if _, err := os.Stat(referenced); err != nil {
		t.Errorf("referenced recording must survive: %v", err)
	}
	if _, err := os.Stat(fresh); err != nil {
		t.Errorf("orphan inside the grace window must survive: %v", err)
	}
	if _, err := os.Stat(stale); err == nil {
		t.Error("orphan older than the grace window should have been removed")
	}
}

func TestCleanOrphans_SkipsWhenLoadIncomplete(t *testing.T) {
	home := setHome(t)

	path := filepath.Join(home, ".config", "vox", "history.jsonl")
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	// A line beyond the scanner cap aborts the load; the "tail" entry after it —
	// which references tail.wav — is therefore never read into memory.
	huge := strings.Repeat("x", 5*1024*1024)
	content := `{"id":"first","raw_text":"first","timestamp":"2026-04-16T12:00:00Z"}` + "\n" +
		`{"id":"huge","raw_text":"` + huge + `"}` + "\n" +
		`{"id":"tail","audio_file":"tail.wav","timestamp":"2026-04-16T12:00:02Z"}` + "\n"
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatalf("write: %v", err)
	}

	h := NewHistory(10, 10)
	if h.LoadError() == nil {
		t.Fatal("a partial read must be reported via LoadError")
	}

	// tail.wav's entry sits past the truncation point, so it is not in the
	// in-memory set. Without the loadErr guard, CleanOrphans would read the
	// directory, see an old unreferenced file, and delete a recording the file on
	// disk still accounts for.
	tail := writeAudio(t, h, "tail")
	old := time.Now().Add(-2 * orphanGrace)
	if err := os.Chtimes(tail, old, old); err != nil {
		t.Fatalf("chtimes: %v", err)
	}

	h.CleanOrphans()

	if _, err := os.Stat(tail); err != nil {
		t.Errorf("a recording must survive orphan cleanup while the load was incomplete: %v", err)
	}
}

func TestUpdate_MutatesAndPersists(t *testing.T) {
	setHome(t)

	h := NewHistory(10, 10)
	e := sampleEntry("before")
	e.ID = "fixed-id"
	e.Status = StatusFailed
	e.FailedStep = "stt"
	e.ErrorMessage = "boom"
	if err := h.Add(e); err != nil {
		t.Fatalf("Add: %v", err)
	}

	updated, uerr := h.Update("fixed-id", func(x *Entry) {
		x.CleanedText = "after"
		x.Status = StatusOK
		x.FailedStep = ""
		x.ErrorMessage = ""
	})
	if uerr != nil {
		t.Fatalf("Update: %v", uerr)
	}
	if updated.CleanedText != "after" || updated.Failed() {
		t.Errorf("returned entry not updated: %+v", updated)
	}

	// Must survive a reload, not just live in memory.
	h2 := NewHistory(10, 10)
	got := h2.Entries()[0]
	if got.CleanedText != "after" {
		t.Errorf("update did not persist: cleaned_text = %q", got.CleanedText)
	}
	if got.Failed() || got.FailedStep != "" || got.ErrorMessage != "" {
		t.Errorf("failure fields not cleared on disk: %+v", got)
	}
}

func TestUpdate_UnknownIDReportsMissing(t *testing.T) {
	setHome(t)

	h := NewHistory(10, 10)
	if _, err := h.Update("nope", func(*Entry) {}); !errors.Is(err, ErrNotFound) {
		t.Errorf("Update on an unknown id: err = %v, want ErrNotFound", err)
	}
}

func TestFailedEntry_RoundTrips(t *testing.T) {
	setHome(t)

	h := NewHistory(10, 10)
	e := sampleEntry("")
	e.ID = "failed-1"
	e.Status = StatusFailed
	e.FailedStep = "stt"
	e.ErrorMessage = "transcription: 401"
	e.AudioFile = audioFileName("failed-1")
	if err := h.Add(e); err != nil {
		t.Fatalf("Add: %v", err)
	}

	got := NewHistory(10, 10).Entries()[0]
	if !got.Failed() {
		t.Errorf("status did not round-trip: %q", got.Status)
	}
	if got.FailedStep != "stt" || got.ErrorMessage != "transcription: 401" {
		t.Errorf("failure detail did not round-trip: %+v", got)
	}
	if got.AudioFile != audioFileName("failed-1") {
		t.Errorf("audio reference did not round-trip: %q", got.AudioFile)
	}
}

func TestAudioPath_EmptyWhenFileMissing(t *testing.T) {
	setHome(t)

	h := NewHistory(10, 10)
	e := entryWithAudio("gone", "text")

	if got := h.AudioPath(e); got != "" {
		t.Errorf("AudioPath for a missing file = %q, want empty", got)
	}

	writeAudio(t, h, "gone")
	if got := h.AudioPath(e); got == "" {
		t.Error("AudioPath should resolve once the file exists")
	}
}

func TestAudioPath_EmptyWithoutAudioReference(t *testing.T) {
	setHome(t)

	h := NewHistory(10, 10)
	if got := h.AudioPath(sampleEntry("no audio")); got != "" {
		t.Errorf("AudioPath without a reference = %q, want empty", got)
	}
}

func TestAudioUsage_CountsStoredRecordings(t *testing.T) {
	setHome(t)

	h := NewHistory(10, 10)
	writeAudio(t, h, "one")
	writeAudio(t, h, "two")

	files, bytes, err := h.AudioUsage()
	if err != nil {
		t.Fatalf("AudioUsage: %v", err)
	}
	if files != 2 {
		t.Errorf("files = %d, want 2", files)
	}
	if bytes != int64(2*len("RIFFdummy")) {
		t.Errorf("bytes = %d, want %d", bytes, 2*len("RIFFdummy"))
	}
}

func TestLoad_LegacyEntriesGetIDs(t *testing.T) {
	home := setHome(t)

	path := filepath.Join(home, ".config", "vox", "history.jsonl")
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	// Two entries in the pre-id format: one with a timestamp, one without.
	content := `{"raw_text":"old","timestamp":"2026-04-16T12:00:00Z"}
{"raw_text":"older"}
`
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}

	got := NewHistory(10, 10).Entries()
	if len(got) != 2 {
		t.Fatalf("got %d entries, want 2", len(got))
	}
	for _, e := range got {
		if e.ID == "" {
			t.Errorf("legacy entry %q got no id", e.RawText)
		}
	}
	if got[0].ID == got[1].ID {
		t.Errorf("legacy entries must get distinct ids, both are %q", got[0].ID)
	}

	// Ids must be stable across reloads, otherwise the UI would address a
	// different entry after every restart.
	again := NewHistory(10, 10).Entries()
	for i := range got {
		if got[i].ID != again[i].ID {
			t.Errorf("id for %q changed between loads: %q vs %q",
				got[i].RawText, got[i].ID, again[i].ID)
		}
	}
}

// --- untrusted audio references, atomicity, pending, ordering ---

func TestSafeAudioName_RejectsAnythingButAPlainWAV(t *testing.T) {
	cases := []struct {
		in   string
		want string
	}{
		{"42.wav", "42.wav"},
		{"", ""},
		{"../../.ssh/id_rsa", ""},
		{"../42.wav", ""},
		{"./42.wav", ""},
		{"sub/42.wav", ""},
		{".", ""},
		{"..", ""},
		{"42.txt", ""},
		{"42", ""},
	}
	for _, c := range cases {
		if got := safeAudioName(c.in); got != c.want {
			t.Errorf("safeAudioName(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

// A hand-edited or crafted audio_file must not turn into a path outside the
// recordings directory — neither for reading nor, worse, for deleting.
func TestLoad_NeutralisesEscapingAudioReference(t *testing.T) {
	home := setHome(t)

	victim := filepath.Join(home, "precious.txt")
	if err := os.WriteFile(victim, []byte("do not delete"), 0o600); err != nil {
		t.Fatalf("write victim: %v", err)
	}

	path := filepath.Join(home, ".config", "vox", "history.jsonl")
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	line := `{"id":"evil","timestamp":"2026-04-16T12:00:00Z","audio_file":"../../precious.txt"}` + "\n"
	if err := os.WriteFile(path, []byte(line), 0o600); err != nil {
		t.Fatalf("write history: %v", err)
	}

	// audioKeep 0 makes the entry a pruning candidate immediately.
	h := NewHistory(10, 0)

	got := h.Entries()[0]
	if got.AudioFile != "" {
		t.Errorf("escaping audio_file survived load as %q", got.AudioFile)
	}
	if p := h.AudioPath(got); p != "" {
		t.Errorf("AudioPath resolved an escaping reference to %q", p)
	}

	// Adding an entry runs the pruner over the loaded set.
	if err := h.Add(sampleEntry("trigger prune")); err != nil {
		t.Fatalf("Add: %v", err)
	}
	if _, err := os.Stat(victim); err != nil {
		t.Errorf("pruning deleted a file outside the recordings directory: %v", err)
	}
}

func TestWriteAll_LeavesNoTempFileBehind(t *testing.T) {
	home := setHome(t)

	h := NewHistory(2, 10)
	for _, text := range []string{"a", "b", "c"} { // forces a rewrite
		if err := h.Add(sampleEntry(text)); err != nil {
			t.Fatalf("Add(%q): %v", text, err)
		}
	}

	dir := filepath.Join(home, ".config", "vox")
	items, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("readdir: %v", err)
	}
	for _, it := range items {
		if strings.HasSuffix(it.Name(), ".tmp") {
			t.Errorf("rewrite left a temp file behind: %s", it.Name())
		}
	}
}

// A history file that could only be read partially must never be rewritten —
// the file still holds the entries the in-memory set is missing.
func TestPartialLoad_RefusesToRewriteTheFile(t *testing.T) {
	home := setHome(t)

	path := filepath.Join(home, ".config", "vox", "history.jsonl")
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	// A line beyond the scanner's cap aborts the scan; the entry after it is
	// therefore never read.
	huge := strings.Repeat("x", 5*1024*1024)
	content := `{"id":"first","raw_text":"first","timestamp":"2026-04-16T12:00:00Z"}` + "\n" +
		`{"id":"huge","raw_text":"` + huge + `"}` + "\n" +
		`{"id":"last","raw_text":"last","timestamp":"2026-04-16T12:00:02Z"}` + "\n"
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatalf("write: %v", err)
	}

	before, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read before: %v", err)
	}

	h := NewHistory(10, 10)
	if h.LoadError() == nil {
		t.Fatal("a partial read must be reported via LoadError")
	}

	// Any operation that would rewrite the file has to fail instead.
	if _, err := h.Update("first", func(e *Entry) { e.CleanedText = "changed" }); err == nil {
		t.Error("Update must fail while the load was incomplete")
	}

	after, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read after: %v", err)
	}
	if len(before) != len(after) {
		t.Errorf("history file was rewritten despite an incomplete load: %d → %d bytes",
			len(before), len(after))
	}
}

func TestRotation_DeletesAudioOfDroppedEntries(t *testing.T) {
	setHome(t)

	// audioKeep larger than maxSize: pruneAudio alone would never reach the
	// rotated-out entries, so rotation has to delete their audio itself.
	h := NewHistory(2, 10)

	var paths []string
	for _, id := range []string{"a", "b", "c", "d"} {
		paths = append(paths, writeAudio(t, h, id))
		if err := h.Add(entryWithAudio(id, id)); err != nil {
			t.Fatalf("Add(%q): %v", id, err)
		}
	}

	for i, id := range []string{"a", "b", "c", "d"} {
		_, err := os.Stat(paths[i])
		rotatedOut := i < 2
		if rotatedOut && err == nil {
			t.Errorf("audio of rotated-out entry %q still on disk", id)
		}
		if !rotatedOut && err != nil {
			t.Errorf("audio of retained entry %q was deleted: %v", id, err)
		}
	}
}

func TestHoldAudio_ProtectsARecordingFromPruning(t *testing.T) {
	setHome(t)

	h := NewHistory(10, 1)
	held := writeAudio(t, h, "held")
	heldEntry := entryWithAudio("held", "held")
	if err := h.Add(heldEntry); err != nil {
		t.Fatalf("Add: %v", err)
	}

	release := h.HoldAudio(heldEntry)

	// Two newer entries push the held one out of the retention window.
	for _, id := range []string{"newer1", "newer2"} {
		writeAudio(t, h, id)
		if err := h.Add(entryWithAudio(id, id)); err != nil {
			t.Fatalf("Add(%q): %v", id, err)
		}
	}
	if _, err := os.Stat(held); err != nil {
		t.Fatalf("a held recording must survive pruning: %v", err)
	}

	release()
	writeAudio(t, h, "newer3")
	if err := h.Add(entryWithAudio("newer3", "newer3")); err != nil {
		t.Fatalf("Add: %v", err)
	}
	if _, err := os.Stat(held); err == nil {
		t.Error("after release the recording should be pruned")
	}
}

func TestHoldAudio_ReleaseIsIdempotentAndRefCounted(t *testing.T) {
	setHome(t)

	h := NewHistory(10, 0)
	e := entryWithAudio("shared", "shared")

	first := h.HoldAudio(e)
	second := h.HoldAudio(e)
	first()
	first() // repeated release must not drop the second hold

	h.mu.Lock()
	n := h.inUse[audioFileName("shared")]
	h.mu.Unlock()
	if n != 1 {
		t.Errorf("hold count = %d, want 1", n)
	}

	second()
	h.mu.Lock()
	_, still := h.inUse[audioFileName("shared")]
	h.mu.Unlock()
	if still {
		t.Error("the last release must remove the entry")
	}
}

func TestAdoptPending_TurnsInterruptedAttemptsIntoFailures(t *testing.T) {
	setHome(t)

	h := NewHistory(10, 10)
	pending := sampleEntry("half done")
	pending.ID = "pending-1"
	pending.Status = StatusPending
	pending.AudioFile = audioFileName("pending-1")
	if err := h.Add(pending); err != nil {
		t.Fatalf("Add: %v", err)
	}
	done := sampleEntry("finished")
	done.ID = "ok-1"
	if err := h.Add(done); err != nil {
		t.Fatalf("Add: %v", err)
	}

	if n := h.AdoptPending("interrupted"); n != 1 {
		t.Fatalf("adopted %d entries, want 1", n)
	}

	got, ok := NewHistory(10, 10).Get("pending-1")
	if !ok {
		t.Fatal("entry vanished")
	}
	if !got.Failed() || got.ErrorMessage != "interrupted" {
		t.Errorf("pending entry not adopted: %+v", got)
	}
	if got.AudioFile == "" {
		t.Error("the recording reference must survive, otherwise the attempt is not retryable")
	}
	if other, _ := NewHistory(10, 10).Get("ok-1"); other.Failed() {
		t.Error("a finished entry must not be adopted")
	}
}

func TestNewID_UniqueUnderConcurrency(t *testing.T) {
	// Two attempts minted in the same clock tick must not share an id: the id
	// names the recording, so a collision would overwrite one attempt's audio.
	const n = 2000
	ids := make(chan string, n)
	var wg sync.WaitGroup
	for i := 0; i < n; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			ids <- NewID()
		}()
	}
	wg.Wait()
	close(ids)

	seen := make(map[string]bool, n)
	for id := range ids {
		if seen[id] {
			t.Fatalf("NewID produced a duplicate: %q", id)
		}
		seen[id] = true
	}
	if len(seen) != n {
		t.Errorf("got %d unique ids, want %d", len(seen), n)
	}
}

func TestEntries_OrderedByTimestampNotInsertion(t *testing.T) {
	setHome(t)

	h := NewHistory(10, 10)

	// A slow transcription is stored after a faster one that was spoken later.
	early := sampleEntry("spoken first")
	early.ID = "early"
	early.Timestamp = time.Date(2026, 4, 16, 12, 0, 0, 0, time.UTC)
	late := sampleEntry("spoken second")
	late.ID = "late"
	late.Timestamp = time.Date(2026, 4, 16, 12, 0, 30, 0, time.UTC)

	if err := h.Add(late); err != nil { // stored first
		t.Fatalf("Add(late): %v", err)
	}
	if err := h.Add(early); err != nil {
		t.Fatalf("Add(early): %v", err)
	}

	got := h.Entries()
	if got[0].ID != "late" || got[1].ID != "early" {
		t.Errorf("order = [%s, %s], want [late, early]", got[0].ID, got[1].ID)
	}
}

func TestPruneAudio_KeepsNewestByTimestamp(t *testing.T) {
	setHome(t)

	h := NewHistory(10, 1)

	oldPath := writeAudio(t, h, "old")
	newPath := writeAudio(t, h, "new")

	newer := entryWithAudio("new", "newer")
	newer.Timestamp = time.Date(2026, 4, 16, 12, 0, 30, 0, time.UTC)
	older := entryWithAudio("old", "older")
	older.Timestamp = time.Date(2026, 4, 16, 12, 0, 0, 0, time.UTC)

	// Insert the newer one first, so insertion order disagrees with time order.
	if err := h.Add(newer); err != nil {
		t.Fatalf("Add(newer): %v", err)
	}
	if err := h.Add(older); err != nil {
		t.Fatalf("Add(older): %v", err)
	}

	if _, err := os.Stat(newPath); err != nil {
		t.Errorf("the newest recording by timestamp must be kept: %v", err)
	}
	if _, err := os.Stat(oldPath); err == nil {
		t.Error("the older recording should have been pruned")
	}
}

func TestStoreAudio_MovesFileOwnerOnly(t *testing.T) {
	setHome(t)

	h := NewHistory(10, 10)
	tmp := filepath.Join(t.TempDir(), "incoming.wav")
	if err := os.WriteFile(tmp, []byte("RIFFdummy"), 0o644); err != nil {
		t.Fatalf("write tmp: %v", err)
	}

	name, path, err := h.StoreAudio("entry-1", tmp)
	if err != nil {
		t.Fatalf("StoreAudio: %v", err)
	}
	if name != audioFileName("entry-1") {
		t.Errorf("name = %q, want %q", name, audioFileName("entry-1"))
	}
	if _, err := os.Stat(tmp); err == nil {
		t.Error("the temp file should be gone after storing")
	}

	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat stored: %v", err)
	}
	dirInfo, err := os.Stat(filepath.Dir(path))
	if err != nil {
		t.Fatalf("stat dir: %v", err)
	}
	// Windows does not represent Unix permission bits — os.Stat reports 0666/0777
	// there regardless of the mode passed to OpenFile/MkdirAll — so the owner-only
	// guarantee only holds, and is only checkable, on Unix.
	if runtime.GOOS != "windows" {
		if perm := info.Mode().Perm(); perm != 0o600 {
			t.Errorf("recording mode = %o, want 600 — recordings are the user's voice", perm)
		}
		if perm := dirInfo.Mode().Perm(); perm != 0o700 {
			t.Errorf("recordings dir mode = %o, want 700", perm)
		}
	}
}

func TestStoredAudioAndHasAudio(t *testing.T) {
	setHome(t)

	h := NewHistory(10, 10)
	writeAudio(t, h, "there")

	present := h.StoredAudio()
	if !HasAudio(entryWithAudio("there", "x"), present) {
		t.Error("HasAudio should find a stored recording")
	}
	if HasAudio(entryWithAudio("gone", "x"), present) {
		t.Error("HasAudio must not claim a missing recording")
	}
	if HasAudio(sampleEntry("no reference"), present) {
		t.Error("an entry without a reference has no audio")
	}
	if HasAudio(Entry{AudioFile: "../../escape.wav"}, present) {
		t.Error("an escaping reference must never count as audio")
	}
}

// TestSegments_PersistAcrossReload pins the VOX-13 storage contract: segments
// written with an entry survive the JSONL round trip, and entries without the
// field — everything written before it existed — load as segment-less instead
// of failing.
func TestSegments_PersistAcrossReload(t *testing.T) {
	home := setHome(t)

	h := NewHistory(10, 10)
	e := sampleEntry("mit segmenten")
	e.Segments = []Segment{
		{Start: 0.0, End: 1.8, Text: "Guten Morgen."},
		{Start: 2.6, End: 4.2, Text: " Bis später."},
	}
	if err := h.Add(e); err != nil {
		t.Fatalf("Add: %v", err)
	}

	// A legacy line without the segments field, as older versions wrote it.
	legacy := `{"id":"legacy-1","timestamp":"2026-04-15T12:00:00Z","language":"de","raw_text":"alt","cleaned_text":"alt","app_context":"test","duration_seconds":1,"backend":"whisper"}` + "\n"
	path := filepath.Join(home, ".config", "vox", "history.jsonl")
	f, err := os.OpenFile(path, os.O_APPEND|os.O_WRONLY, 0o600)
	if err != nil {
		t.Fatalf("open history file: %v", err)
	}
	if _, err := f.WriteString(legacy); err != nil {
		t.Fatalf("append legacy line: %v", err)
	}
	f.Close()

	reloaded := NewHistory(10, 10)
	entries := reloaded.Entries()
	if len(entries) != 2 {
		t.Fatalf("got %d entries after reload, want 2", len(entries))
	}

	var withSegments, legacyEntry *Entry
	for i := range entries {
		if entries[i].RawText == "mit segmenten" {
			withSegments = &entries[i]
		}
		if entries[i].RawText == "alt" {
			legacyEntry = &entries[i]
		}
	}
	if withSegments == nil || legacyEntry == nil {
		t.Fatalf("entries after reload: %+v", entries)
	}
	if len(withSegments.Segments) != 2 ||
		withSegments.Segments[0] != (Segment{Start: 0.0, End: 1.8, Text: "Guten Morgen."}) ||
		withSegments.Segments[1] != (Segment{Start: 2.6, End: 4.2, Text: " Bis später."}) {
		t.Errorf("segments after reload = %v", withSegments.Segments)
	}
	if legacyEntry.Segments != nil {
		t.Errorf("legacy entry segments = %v, want nil", legacyEntry.Segments)
	}
}
