package history

import (
	"bufio"
	"encoding/json"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// Entry represents a single transcription event.
type Entry struct {
	Timestamp   time.Time `json:"timestamp"`
	Language    string    `json:"language"`
	RawText     string    `json:"raw_text"`
	CleanedText string    `json:"cleaned_text"`
	AppContext  string    `json:"app_context"`
	DurationSec float64  `json:"duration_seconds"`
	Backend     string    `json:"backend"`
}

// History manages transcription history stored as JSONL.
type History struct {
	mu      sync.Mutex
	entries []Entry
	path    string
	maxSize int
}

// NewHistory creates a History that stores up to maxSize entries.
// It loads existing entries from ~/.config/vox/history.jsonl.
func NewHistory(maxSize int) *History {
	h := &History{maxSize: maxSize}

	home, err := os.UserHomeDir()
	if err != nil {
		return h
	}
	h.path = filepath.Join(home, ".config", "vox", "history.jsonl")

	h.load()
	return h
}

func (h *History) load() {
	f, err := os.Open(h.path)
	if err != nil {
		return
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		var e Entry
		if err := json.Unmarshal(scanner.Bytes(), &e); err == nil {
			h.entries = append(h.entries, e)
		}
	}

	// Keep only the most recent maxSize entries
	if len(h.entries) > h.maxSize {
		h.entries = h.entries[len(h.entries)-h.maxSize:]
	}
}

// Add appends an entry to the history.
func (h *History) Add(entry Entry) error {
	h.mu.Lock()
	defer h.mu.Unlock()

	h.entries = append(h.entries, entry)

	// Rotate if over max
	if len(h.entries) > h.maxSize {
		h.entries = h.entries[len(h.entries)-h.maxSize:]
		return h.writeAll()
	}

	return h.appendOne(entry)
}

func (h *History) appendOne(entry Entry) error {
	if h.path == "" {
		return nil
	}

	if err := os.MkdirAll(filepath.Dir(h.path), 0o755); err != nil {
		return err
	}

	f, err := os.OpenFile(h.path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return err
	}
	defer f.Close()

	data, err := json.Marshal(entry)
	if err != nil {
		return err
	}
	_, err = f.Write(append(data, '\n'))
	return err
}

func (h *History) writeAll() error {
	if h.path == "" {
		return nil
	}

	if err := os.MkdirAll(filepath.Dir(h.path), 0o755); err != nil {
		return err
	}

	f, err := os.Create(h.path)
	if err != nil {
		return err
	}
	defer f.Close()

	for _, e := range h.entries {
		data, err := json.Marshal(e)
		if err != nil {
			continue
		}
		if _, err := f.Write(append(data, '\n')); err != nil {
			return err
		}
	}
	return nil
}

// Entries returns a copy of all entries, newest first.
func (h *History) Entries() []Entry {
	h.mu.Lock()
	defer h.mu.Unlock()

	result := make([]Entry, len(h.entries))
	for i, e := range h.entries {
		result[len(h.entries)-1-i] = e
	}
	return result
}
