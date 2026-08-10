package history

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"sync"
	"time"
)

// Entry status values. An empty status means "ok" so that history files
// written before failures were recorded keep their meaning.
const (
	StatusOK     = "ok"
	StatusFailed = "failed"

	// StatusPending marks an attempt whose recording is stored but whose
	// transcription has not finished. It exists so the recording is referenced
	// by an entry from the moment it hits the disk: transcription can take
	// minutes, and a crash in that window would otherwise leave a recording
	// that no entry points at — invisible in the history and deleted as an
	// orphan on a later start.
	StatusPending = "pending"
)

// ErrNotFound reports that no entry with the requested id exists.
var ErrNotFound = errors.New("history entry not found")

// Entry represents a single dictation attempt — successful or not.
type Entry struct {
	ID          string    `json:"id"`
	Timestamp   time.Time `json:"timestamp"`
	Language    string    `json:"language"`
	RawText     string    `json:"raw_text"`
	CleanedText string    `json:"cleaned_text"`
	AppContext  string    `json:"app_context"`
	DurationSec float64   `json:"duration_seconds"`
	Backend     string    `json:"backend"`

	// WindowTitle is kept because the cleanup tone is derived from the app name
	// *and* the window title. Without it a retry would pick a different tone
	// than the original attempt.
	WindowTitle string `json:"window_title,omitempty"`

	// Failure bookkeeping. Empty Status means the attempt succeeded.
	Status       string `json:"status,omitempty"`
	FailedStep   string `json:"failed_step,omitempty"`
	ErrorMessage string `json:"error_message,omitempty"`

	// SuspectedHallucination marks a transcript that matched a known Whisper-
	// hallucination pattern. The text was delivered and stored anyway — the
	// patterns are fuzzy and real dictation must never be lost to a false
	// alarm — so this is a review hint, not a failure.
	SuspectedHallucination bool `json:"suspected_hallucination,omitempty"`

	// AudioFile is the recording's basename inside the recordings directory.
	// The file may already be gone — audio is kept only for the newest
	// entries — so presence of the field does not imply presence of the file.
	AudioFile string `json:"audio_file,omitempty"`
}

// Failed reports whether the attempt did not produce inserted text.
func (e Entry) Failed() bool { return e.Status == StatusFailed }

// Pending reports whether the attempt was still being processed.
func (e Entry) Pending() bool { return e.Status == StatusPending }

// HasText reports whether the entry holds a transcription worth keeping.
func (e Entry) HasText() bool { return e.RawText != "" || e.CleanedText != "" }

// History manages dictation history stored as JSONL, plus the retention of
// the recorded audio that belongs to it.
//
// Text and audio are retained independently: text is small (~1 KB per entry)
// and stays for maxSize entries, audio is large (~1.9 MB per minute) and
// stays for the newest audioKeep entries only.
type History struct {
	mu        sync.Mutex
	entries   []Entry
	path      string
	audioDir  string
	maxSize   int
	audioKeep int

	// loadErr is set when the history file could only be read partially. The
	// in-memory set is then incomplete and must never be written back over the
	// file, which still holds everything.
	loadErr error

	// inUse counts recordings a retry is currently reading, keyed by basename.
	// Pruning skips them, so a retry cannot lose its own source mid-flight.
	inUse map[string]int
}

// NewHistory creates a History that keeps up to maxSize entries and retains
// audio for the newest audioKeep of them. It loads existing entries from
// ~/.config/vox/history.jsonl.
func NewHistory(maxSize, audioKeep int) *History {
	h := &History{maxSize: maxSize, audioKeep: audioKeep, inUse: map[string]int{}}

	home, err := os.UserHomeDir()
	if err != nil {
		h.loadErr = err
		return h
	}
	h.path = filepath.Join(home, ".config", "vox", "history.jsonl")
	h.audioDir = filepath.Join(home, ".config", "vox", "recordings")

	h.load()
	return h
}

// safeAudioName returns name if it is a plain file name for a recording inside
// the recordings directory, and "" otherwise.
//
// The value comes from a file on disk, so it is untrusted input: without this
// check a crafted or hand-edited "audio_file" such as "../../.ssh/id_rsa" would
// resolve outside the directory and be read, uploaded to the STT backend, or
// deleted by pruning. A symlink placed inside the directory itself still
// escapes, but that needs write access to the directory, which is the same
// precondition as editing the history file in the first place.
func safeAudioName(name string) string {
	if name == "" || name != filepath.Base(name) || name == "." || name == ".." {
		return ""
	}
	if filepath.Ext(name) != ".wav" {
		return ""
	}
	return name
}

func (h *History) load() {
	f, err := os.Open(h.path)
	if err != nil {
		if !os.IsNotExist(err) {
			h.loadErr = err
		}
		return
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	// Entries hold full transcripts, which can exceed the default 64 KB line cap.
	scanner.Buffer(make([]byte, 0, 64*1024), 4*1024*1024)
	line := 0
	for scanner.Scan() {
		line++
		var e Entry
		if err := json.Unmarshal(scanner.Bytes(), &e); err != nil {
			continue
		}
		// Entries written before ids existed get one derived from their
		// timestamp, which the UI needs to address them at all.
		if e.ID == "" {
			if e.Timestamp.IsZero() {
				e.ID = fmt.Sprintf("legacy-%d", line)
			} else {
				e.ID = idFromTime(e.Timestamp)
			}
		}
		// Normalise here so every later use — resolving, pruning, orphan
		// matching — works on the same trusted value.
		e.AudioFile = safeAudioName(e.AudioFile)
		h.entries = append(h.entries, e)
	}
	if err := scanner.Err(); err != nil {
		// The tail of the file was not read. Record it so writeAll refuses to
		// rewrite the file with an incomplete set — the file still has the data.
		h.loadErr = fmt.Errorf("history file read incompletely after %d entries: %w", len(h.entries), err)
		return
	}

	// Keep only the most recent maxSize entries. Their recordings go with them:
	// nothing references those files afterwards.
	if len(h.entries) > h.maxSize {
		dropped := h.entries[:len(h.entries)-h.maxSize]
		h.entries = h.entries[len(h.entries)-h.maxSize:]
		h.removeAudioOf(dropped)
	}
}

// LoadError reports why the history file could only be read partially, if that
// happened. While it is non-nil the file is never rewritten.
func (h *History) LoadError() error {
	h.mu.Lock()
	defer h.mu.Unlock()
	return h.loadErr
}

// idFromTime derives a stable id for entries written before ids existed.
func idFromTime(t time.Time) string {
	return strconv.FormatInt(t.UnixNano(), 10)
}

var (
	idMu     sync.Mutex
	idLastNS int64
)

// NewID returns a process-unique id for a new entry, exported so the caller can
// name the audio file before the entry itself exists. It is monotonic: two
// entries minted within the same clock tick — plausible on a coarse-resolution
// clock — would otherwise share an id and, through audioFileName, overwrite each
// other's recording.
func NewID() string {
	idMu.Lock()
	ns := time.Now().UnixNano()
	if ns <= idLastNS {
		ns = idLastNS + 1
	}
	idLastNS = ns
	idMu.Unlock()
	return strconv.FormatInt(ns, 10)
}

// audioDirEnsured returns the directory holding recordings, creating it on
// demand as owner-only: the files in it are recordings of the user's voice.
func (h *History) audioDirEnsured() (string, error) {
	if h.audioDir == "" {
		return "", fmt.Errorf("no home directory — cannot store recordings")
	}
	if err := os.MkdirAll(h.audioDir, 0o700); err != nil {
		return "", err
	}
	return h.audioDir, nil
}

// audioFileName is the canonical recording name for an entry id.
func audioFileName(id string) string { return id + ".wav" }

// StoreAudio moves a finished recording into the recordings directory. It
// returns the basename to store on the entry and the new path to read it from.
//
// The whole lifecycle of that directory lives in this package — naming,
// resolving, retention — so putting a file into it belongs here too.
func (h *History) StoreAudio(entryID, tmpPath string) (name string, path string, err error) {
	dir, err := h.audioDirEnsured()
	if err != nil {
		return "", "", err
	}

	name = audioFileName(entryID)
	dst := filepath.Join(dir, name)

	if err := os.Rename(tmpPath, dst); err == nil {
		// The mode is inherited from the temp file, which is world-readable.
		if err := os.Chmod(dst, 0o600); err != nil {
			return "", "", err
		}
		return name, dst, nil
	}

	// Rename fails across filesystems, and the temp directory frequently sits
	// on a different one than the home directory.
	if err := copyFile(tmpPath, dst); err != nil {
		return "", "", err
	}
	_ = os.Remove(tmpPath)
	return name, dst, nil
}

// AdoptPending turns entries left in the pending state into failed ones. Called
// at startup: the process that was transcribing them is gone, so they can only
// be leftovers from a crash or a kill. Their recording is kept, so they stay
// retryable. It returns how many entries were adopted.
func (h *History) AdoptPending(reason string) int {
	h.mu.Lock()
	defer h.mu.Unlock()

	n := 0
	for i := range h.entries {
		if !h.entries[i].Pending() {
			continue
		}
		h.entries[i].Status = StatusFailed
		h.entries[i].ErrorMessage = reason
		n++
	}
	if n > 0 {
		_ = h.writeAll()
	}
	return n
}

func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()

	out, err := os.OpenFile(dst, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0o600)
	if err != nil {
		return err
	}
	if _, err := io.Copy(out, in); err != nil {
		out.Close()
		os.Remove(dst)
		return err
	}
	if err := out.Sync(); err != nil {
		out.Close()
		os.Remove(dst)
		return err
	}
	return out.Close()
}

// StoredAudio returns the set of recording basenames currently on disk. The UI
// uses it to answer "does this entry still have audio" for a whole history in
// one directory scan instead of one stat per entry.
func (h *History) StoredAudio() map[string]struct{} {
	present := map[string]struct{}{}
	if h.audioDir == "" {
		return present
	}
	items, err := os.ReadDir(h.audioDir)
	if err != nil {
		return present
	}
	for _, it := range items {
		if !it.IsDir() && filepath.Ext(it.Name()) == ".wav" {
			present[it.Name()] = struct{}{}
		}
	}
	return present
}

// HasAudio reports whether the entry's recording is in the given set, as
// returned by StoredAudio.
func HasAudio(e Entry, present map[string]struct{}) bool {
	name := safeAudioName(e.AudioFile)
	if name == "" {
		return false
	}
	_, ok := present[name]
	return ok
}

// AudioPath resolves an entry's recording path. It returns an empty string
// when the entry never had audio, the reference is not a plain recording name,
// or the file has since been pruned.
func (h *History) AudioPath(e Entry) string {
	name := safeAudioName(e.AudioFile)
	if name == "" || h.audioDir == "" {
		return ""
	}
	p := filepath.Join(h.audioDir, name)
	if _, err := os.Stat(p); err != nil {
		return ""
	}
	return p
}

// HoldAudio marks an entry's recording as in use and returns a release
// function. Pruning skips held recordings, so a long-running retry cannot have
// its source deleted by a dictation that finishes in the meantime.
func (h *History) HoldAudio(e Entry) func() {
	name := safeAudioName(e.AudioFile)
	if name == "" {
		return func() {}
	}

	h.mu.Lock()
	h.inUse[name]++
	h.mu.Unlock()

	var once sync.Once
	return func() {
		once.Do(func() {
			h.mu.Lock()
			defer h.mu.Unlock()
			if h.inUse[name] <= 1 {
				delete(h.inUse, name)
				return
			}
			h.inUse[name]--
		})
	}
}

// Path returns the JSONL file path.
func (h *History) Path() string { return h.path }

// Add appends an entry, assigning an id when the caller left it empty, and
// applies audio retention.
func (h *History) Add(entry Entry) error {
	h.mu.Lock()
	defer h.mu.Unlock()

	if entry.ID == "" {
		entry.ID = NewID()
	}
	if entry.Status == "" {
		entry.Status = StatusOK
	}
	entry.AudioFile = safeAudioName(entry.AudioFile)

	h.entries = append(h.entries, entry)

	var err error
	if len(h.entries) > h.maxSize {
		// Rotating out an entry also drops the only reference to its recording,
		// so delete it here — pruneAudio below would no longer see it.
		dropped := h.entries[:len(h.entries)-h.maxSize]
		h.entries = h.entries[len(h.entries)-h.maxSize:]
		h.removeAudioOf(dropped)
		err = h.writeAll()
	} else {
		err = h.appendOne(entry)
	}

	h.pruneAudio()
	return err
}

// Update applies mutate to the entry with the given id and rewrites the file.
//
// It returns ErrNotFound when no such entry exists. Any other error comes from
// the rewrite: the in-memory entry is then already updated while the file still
// holds the previous state, so the caller must not report the change as
// persisted.
func (h *History) Update(id string, mutate func(*Entry)) (Entry, error) {
	h.mu.Lock()
	defer h.mu.Unlock()

	for i := range h.entries {
		if h.entries[i].ID != id {
			continue
		}
		mutate(&h.entries[i])
		h.entries[i].AudioFile = safeAudioName(h.entries[i].AudioFile)
		return h.entries[i], h.writeAll()
	}
	return Entry{}, ErrNotFound
}

// Get returns the entry with the given id.
func (h *History) Get(id string) (Entry, bool) {
	h.mu.Lock()
	defer h.mu.Unlock()

	for _, e := range h.entries {
		if e.ID == id {
			return e, true
		}
	}
	return Entry{}, false
}

func (h *History) appendOne(entry Entry) error {
	if h.path == "" {
		return nil
	}

	if err := os.MkdirAll(filepath.Dir(h.path), 0o700); err != nil {
		return err
	}

	f, err := os.OpenFile(h.path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o600)
	if err != nil {
		return err
	}
	defer f.Close()

	data, err := json.Marshal(entry)
	if err != nil {
		return err
	}
	if _, err := f.Write(append(data, '\n')); err != nil {
		return err
	}
	// Flush to disk: this is the durable marker that the recording exists, and a
	// crash before it reaches disk turns a recoverable attempt into an orphan.
	return f.Sync()
}

// writeAll replaces the history file with the current entries.
//
// It writes a temporary file and renames it into place, so an interrupted
// rewrite leaves the previous file intact. Rewriting truncates in place
// otherwise, and this path runs on every retry — losing the history to a crash
// while saving a recovered dictation would defeat the purpose.
func (h *History) writeAll() error {
	if h.path == "" {
		return nil
	}
	if h.loadErr != nil {
		return fmt.Errorf("refusing to rewrite the history file: %w", h.loadErr)
	}

	if err := os.MkdirAll(filepath.Dir(h.path), 0o700); err != nil {
		return err
	}

	tmp := h.path + ".tmp"
	f, err := os.OpenFile(tmp, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0o600)
	if err != nil {
		return err
	}

	writeErr := func() error {
		for _, e := range h.entries {
			data, err := json.Marshal(e)
			if err != nil {
				continue
			}
			if _, err := f.Write(append(data, '\n')); err != nil {
				return err
			}
		}
		return f.Sync()
	}()

	if closeErr := f.Close(); writeErr == nil {
		writeErr = closeErr
	}
	if writeErr != nil {
		os.Remove(tmp)
		return writeErr
	}

	if err := os.Rename(tmp, h.path); err != nil {
		os.Remove(tmp)
		return err
	}
	// fsync the directory so the rename itself survives a crash: the temp file's
	// contents are synced above, but on some filesystems the directory entry that
	// makes the new file the history can still be lost without this. Best-effort —
	// a directory handle does not support Sync on every platform.
	if dir, err := os.Open(filepath.Dir(h.path)); err == nil {
		_ = dir.Sync()
		_ = dir.Close()
	}
	return nil
}

// removeAudioOf deletes the recordings of the given entries. Callers must hold
// h.mu.
func (h *History) removeAudioOf(entries []Entry) {
	if h.audioDir == "" {
		return
	}
	for _, e := range entries {
		name := safeAudioName(e.AudioFile)
		if name == "" {
			continue
		}
		if _, held := h.inUse[name]; held {
			continue
		}
		_ = os.Remove(filepath.Join(h.audioDir, name))
	}
}

// pruneAudio deletes recordings belonging to entries outside the retention
// window. Callers must hold h.mu.
//
// It only ever deletes files a known entry points at; files it does not know
// about are left alone. A recording whose transcription is still in flight does
// have an entry now — it is written as pending before transcribing — so it can
// fall inside this prune; callers guard it with HoldAudio for the duration, and
// held recordings are skipped below. Unreferenced leftovers from a crash are
// handled at startup instead (see CleanOrphans).
func (h *History) pruneAudio() {
	if h.audioDir == "" || h.audioKeep < 0 {
		return
	}
	// A partial load leaves the newest entries out of the in-memory set, so a
	// prune driven by it could delete a recording whose entry simply was not
	// read. Refuse, like CleanOrphans and writeAll do — retention waits for a
	// clean load rather than risk deleting data the file still holds.
	if h.loadErr != nil {
		return
	}

	type ref struct {
		name string
		ts   time.Time
	}
	// Walk backwards so the newest insertion comes first, matching Entries().
	refs := make([]ref, 0, len(h.entries))
	for i := len(h.entries) - 1; i >= 0; i-- {
		if name := safeAudioName(h.entries[i].AudioFile); name != "" {
			refs = append(refs, ref{name: name, ts: h.entries[i].Timestamp})
		}
	}
	if len(refs) <= h.audioKeep {
		return
	}

	// Newest first by timestamp, not by insertion order: transcriptions run
	// concurrently, so entries are not appended in the order they were spoken.
	// Insertion order breaks ties, as in Entries().
	sort.SliceStable(refs, func(i, j int) bool { return refs[i].ts.After(refs[j].ts) })

	// Entries keep their audio reference after the file is gone, so most
	// candidates here no longer exist. One directory read beats a few hundred
	// unlink calls that are known to fail.
	present := h.StoredAudio()
	for _, r := range refs[h.audioKeep:] {
		if _, ok := present[r.name]; !ok {
			continue
		}
		if _, held := h.inUse[r.name]; held {
			continue
		}
		_ = os.Remove(filepath.Join(h.audioDir, r.name))
	}
}

// orphanGrace is how long an unreferenced recording is left alone. A crash
// between saving the audio and writing its entry leaves such a file behind;
// keeping it for one startup cycle means a recording is never deleted just
// because the app restarted while it was being processed.
const orphanGrace = time.Hour

// CleanOrphans removes recordings in the audio directory that no entry refers
// to and that are older than orphanGrace. Call it at startup only: at that
// point no transcription is in flight, so an unreferenced file is a leftover
// from a crash or a kill rather than work in progress.
func (h *History) CleanOrphans() {
	h.mu.Lock()
	defer h.mu.Unlock()

	if h.audioDir == "" {
		return
	}
	// A partial load leaves the newest entries out of the in-memory set, so their
	// recordings would be misclassified as orphans and deleted. Refuse then,
	// mirroring writeAll's guard: the file on disk still holds everything, and
	// deleting its audio is exactly the loss that guard exists to prevent.
	if h.loadErr != nil {
		return
	}
	// Applying the retention window must not depend on the directory scan below
	// succeeding.
	defer h.pruneAudio()

	referenced := make(map[string]struct{}, len(h.entries))
	for _, e := range h.entries {
		if name := safeAudioName(e.AudioFile); name != "" {
			referenced[name] = struct{}{}
		}
	}

	items, err := os.ReadDir(h.audioDir)
	if err != nil {
		return
	}

	cutoff := time.Now().Add(-orphanGrace)
	for _, it := range items {
		if it.IsDir() || filepath.Ext(it.Name()) != ".wav" {
			continue
		}
		if _, ok := referenced[it.Name()]; ok {
			continue
		}
		info, err := it.Info()
		if err != nil || info.ModTime().After(cutoff) {
			continue
		}
		_ = os.Remove(filepath.Join(h.audioDir, it.Name()))
	}
}

// AudioUsage reports how many recordings are stored and how much space they
// occupy, so the UI can show the cost instead of hiding it.
//
// It returns an error rather than zeroes when the directory cannot be read: a
// confident "0 stored, 0 B" would be a lie in exactly the case where the user
// most needs to know something is wrong.
func (h *History) AudioUsage() (files int, bytes int64, err error) {
	if h.audioDir == "" {
		return 0, 0, nil
	}
	items, err := os.ReadDir(h.audioDir)
	if err != nil {
		if os.IsNotExist(err) {
			// Nothing recorded yet — genuinely zero.
			return 0, 0, nil
		}
		return 0, 0, err
	}
	for _, it := range items {
		if it.IsDir() || filepath.Ext(it.Name()) != ".wav" {
			continue
		}
		info, err := it.Info()
		if err != nil {
			continue
		}
		files++
		bytes += info.Size()
	}
	return files, bytes, nil
}

// AudioKeep returns the number of entries whose audio is retained.
func (h *History) AudioKeep() int { return h.audioKeep }

// MaxSize returns the number of entries whose text is retained.
func (h *History) MaxSize() int { return h.maxSize }

// Entries returns a copy of all entries, newest first.
//
// Ordered by timestamp rather than by insertion: transcriptions run
// concurrently, so a slow attempt can be stored after a faster one that was
// spoken later. Insertion order is only the tie-breaker — it decides for
// entries with equal or missing timestamps, which is why the slice is reversed
// before the stable sort.
func (h *History) Entries() []Entry {
	h.mu.Lock()
	defer h.mu.Unlock()

	result := make([]Entry, len(h.entries))
	for i, e := range h.entries {
		result[len(h.entries)-1-i] = e
	}
	sort.SliceStable(result, func(i, j int) bool {
		return result[i].Timestamp.After(result[j].Timestamp)
	})
	return result
}
