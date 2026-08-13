package main

import (
	"encoding/base64"
	"errors"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/smnhffmnn/vox/internal/apierr"
	"github.com/smnhffmnn/vox/internal/audio"
	"github.com/smnhffmnn/vox/internal/cleanup"
	"github.com/smnhffmnn/vox/internal/config"
	"github.com/smnhffmnn/vox/internal/feedback"
	"github.com/smnhffmnn/vox/internal/history"
	"github.com/smnhffmnn/vox/internal/hotkey"
	"github.com/smnhffmnn/vox/internal/inject"
	"github.com/smnhffmnn/vox/internal/keychain"
	"github.com/smnhffmnn/vox/internal/logbuf"
	"github.com/smnhffmnn/vox/internal/notify"
	"github.com/smnhffmnn/vox/internal/openpath"
	"github.com/smnhffmnn/vox/internal/permissions"
	"github.com/smnhffmnn/vox/internal/stt"
	"github.com/smnhffmnn/vox/internal/windowctx"
)

const insufficientCreditsMessage = "OpenAI-Guthaben aufgebraucht — API-Key oder Plan prüfen"

// errNoSpeech marks a transcription that came back empty: there is nothing to
// insert or to clean up. The recording is kept like any other failure's — an
// empty result can be the backend's fault, so the attempt stays retryable.
var errNoSpeech = errors.New("no speech detected")

var version = "dev"

// UIBridge abstracts desktop UI operations so the core logic
// compiles without Wails in headless builds.
type UIBridge interface {
	SetTrayIcon(icon []byte)
	SetTrayLabel(label string)
	ShowOverlay(x, y int)
	HideOverlay()
	EmitEvent(name string, data any)
	ShowWindow()
}

// App is the main application struct.
type App struct {
	cfg  *config.Config
	hist *history.History
	ui   UIBridge

	// State
	state        string
	stateMu      sync.RWMutex
	started      time.Time
	recordingGen atomic.Uint64

	// Recording state (all access under recordingMu)
	recording   *audio.Recording
	recordingMu sync.Mutex
	isRecording bool
	toggleState bool

	// Hands-free state (under recordingMu)
	handsfreeActive bool
	handsfreeTimer  *time.Timer
	handsfreeDone   chan struct{}

	// Double-tap detection (under recordingMu)
	lastPressTime    time.Time
	lastReleaseTime  time.Time
	doubletapTimer   *time.Timer
	doubletapPending bool

	// Hotkey
	listener hotkey.Listener

	// Dynamic data (protected by dataMu)
	dataMu        sync.RWMutex
	dictionary    []string
	snippets      []config.Snippet
	customPrompts map[string]string
}

// NewApp creates a new App with config pre-loaded.
func NewApp() *App {
	cfg, err := config.Load()
	if err != nil {
		cfg = config.DefaultConfig()
	}
	return &App{
		state:   "idle",
		started: time.Now(),
		cfg:     cfg,
		hist:    history.NewHistory(cfg.HistorySize, cfg.AudioKeep),
	}
}

// Start initializes the app (hotkey listener, dynamic data).
// Called by the desktop lifecycle (ServiceStartup) or headless entry.
func (a *App) Start() {
	// Push warnings and errors to the UI as they happen, so a cause is visible
	// without opening a terminal — the Diagnostics view is built to show exactly
	// these, and a warning-level degrade (cleanup fell back to raw text, a
	// recovery ran) would otherwise only appear after a manual refresh. Info
	// stays out of the live stream as noise; it is still in the buffer for
	// GetLogs. The frontend decides what raises the error badge.
	logbuf.SetSink(func(rec logbuf.Record) {
		if a.ui == nil || (rec.Level != logbuf.LevelError && rec.Level != logbuf.LevelWarn) {
			return
		}
		a.ui.EmitEvent("log-error", map[string]string{
			"time":    rec.Time.Format(time.RFC3339),
			"level":   rec.Level,
			"step":    rec.Step,
			"message": rec.Message,
		})
	})

	if err := a.hist.LoadError(); err != nil {
		logbuf.Errorf(logbuf.StepHistory, "history read incompletely — it will not be rewritten, so nothing is lost: %v", err)
	}

	// Any attempt still marked pending belongs to a process that is gone.
	if n := a.hist.AdoptPending("interrupted before the transcription finished"); n > 0 {
		logbuf.Warnf(logbuf.StepHistory, "%d interrupted attempt(s) recovered — the recordings are kept and can be transcribed again", n)
	}

	// Safe to do here: nothing is being transcribed yet, so any recording
	// without a history entry is a leftover from a crash.
	a.hist.CleanOrphans()

	a.reloadDynamicData()
	a.startHotkeyListener()
	logbuf.Infof(logbuf.StepApp, "service started")
}

// Shutdown cleans up resources.
func (a *App) Shutdown() {
	// Drop the sink before the UI goes away, so nothing emits into a torn-down
	// Wails app.
	logbuf.SetSink(nil)
	if a.listener != nil {
		a.listener.Close()
	}
}

// --- State Management ---

func (a *App) setState(state string) {
	a.stateMu.Lock()
	a.state = state
	a.stateMu.Unlock()

	if a.ui == nil {
		return
	}

	switch state {
	case "recording":
		a.ui.SetTrayIcon(trayIconRecording)
		a.ui.SetTrayLabel("Recording...")
	case "processing":
		a.ui.SetTrayIcon(trayIconProcessing)
		a.ui.SetTrayLabel("Processing...")
	default:
		a.ui.SetTrayIcon(trayIconIdle)
		a.ui.SetTrayLabel("Idle")
	}

	if a.getShowOverlay() && (state == "recording" || state == "processing") {
		s := hotkey.GetMainScreenInfo()
		overlayWidth := 240
		x := (s.Width - overlayWidth) / 2
		y := s.MenuBarHeight + 8
		a.ui.ShowOverlay(x, y)
	} else {
		a.ui.HideOverlay()
	}

	payload := map[string]any{"state": state}
	if state == "recording" {
		payload["started_at"] = time.Now().UnixMilli()
	}
	a.ui.EmitEvent("state-changed", payload)
}

func (a *App) getState() string {
	a.stateMu.RLock()
	defer a.stateMu.RUnlock()
	return a.state
}

// --- Frontend Bindings ---

// ConfigResponse holds the config for the frontend.
type ConfigResponse struct {
	Language         string `json:"language"`
	Output           string `json:"output"`
	Raw              bool   `json:"raw"`
	Hotkey           string `json:"hotkey"`
	Mode             string `json:"mode"`
	HandsfreeTimeout int    `json:"handsfree_timeout"`
	DoubletapWindow  int    `json:"doubletap_window"`
	Notifications    bool   `json:"notifications"`
	AudioFeedback    bool   `json:"audio_feedback"`
	ShowOverlay      bool   `json:"show_overlay"`
	STTBackend       string `json:"stt_backend"`
	STTURL           string `json:"stt_url"`
	LLMBackend       string `json:"llm_backend"`
	LLMURL           string `json:"llm_url"`
	LLMModel         string `json:"llm_model"`
}

// GetConfig returns the current configuration.
func (a *App) GetConfig() ConfigResponse {
	a.cfg.RLock()
	defer a.cfg.RUnlock()
	return ConfigResponse{
		Language:         a.cfg.Language,
		Output:           a.cfg.Output,
		Raw:              a.cfg.Raw,
		Hotkey:           a.cfg.Hotkey,
		Mode:             a.cfg.Mode,
		HandsfreeTimeout: a.cfg.HandsfreeTimeout,
		DoubletapWindow:  a.cfg.DoubletapWindow,
		Notifications:    a.cfg.Notifications,
		AudioFeedback:    a.cfg.AudioFeedback,
		ShowOverlay:      a.cfg.ShowOverlay,
		STTBackend:       a.cfg.STTBackend,
		STTURL:           a.cfg.STTURL,
		LLMBackend:       a.cfg.LLMBackend,
		LLMURL:           a.cfg.LLMURL,
		LLMModel:         a.cfg.LLMModel,
	}
}

// SaveConfig updates and persists the configuration.
func (a *App) SaveConfig(update ConfigResponse) error {
	a.cfg.Lock()
	oldHotkey := a.cfg.Hotkey
	a.cfg.Language = update.Language
	a.cfg.Output = update.Output
	a.cfg.Raw = update.Raw
	a.cfg.Hotkey = update.Hotkey
	a.cfg.Mode = update.Mode
	a.cfg.HandsfreeTimeout = update.HandsfreeTimeout
	a.cfg.DoubletapWindow = update.DoubletapWindow
	a.cfg.Notifications = update.Notifications
	a.cfg.AudioFeedback = update.AudioFeedback
	a.cfg.ShowOverlay = update.ShowOverlay
	a.cfg.STTBackend = update.STTBackend
	a.cfg.STTURL = update.STTURL
	a.cfg.LLMBackend = update.LLMBackend
	a.cfg.LLMURL = update.LLMURL
	a.cfg.LLMModel = update.LLMModel
	err := a.cfg.Save()
	a.cfg.Unlock()

	if update.Hotkey != oldHotkey {
		a.restartHotkeyListener()
	}
	if !update.ShowOverlay && a.ui != nil {
		a.ui.HideOverlay()
	}
	return err
}

// StatusResponse holds status info for the frontend.
type StatusResponse struct {
	State   string `json:"state"`
	Uptime  string `json:"uptime"`
	Version string `json:"version"`
	HasKey  bool   `json:"has_key"`
}

// GetStatus returns the current daemon status.
func (a *App) GetStatus() StatusResponse {
	return StatusResponse{
		State:   a.getState(),
		Uptime:  time.Since(a.started).Truncate(time.Second).String(),
		Version: version,
		HasKey:  keychain.HasKey("vox", "openai-api-key"),
	}
}

// GetDictionary returns the current dictionary words.
func (a *App) GetDictionary() []string {
	words, _ := config.LoadDictionary()
	if words == nil {
		return []string{}
	}
	sort.Slice(words, func(i, j int) bool {
		return strings.ToLower(words[i]) < strings.ToLower(words[j])
	})
	return words
}

// SaveDictionary saves the dictionary and reloads.
func (a *App) SaveDictionary(words []string) error {
	dir, err := config.ConfigDir()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	sort.Slice(words, func(i, j int) bool {
		return strings.ToLower(words[i]) < strings.ToLower(words[j])
	})
	content := strings.Join(words, "\n") + "\n"
	if err := os.WriteFile(filepath.Join(dir, "dictionary.txt"), []byte(content), 0o644); err != nil {
		return err
	}
	a.reloadDynamicData()
	return nil
}

// GetSnippets returns the current snippets.
func (a *App) GetSnippets() []config.Snippet {
	snippets, _ := config.LoadSnippets()
	if snippets == nil {
		return []config.Snippet{}
	}
	return snippets
}

// SaveSnippets saves snippets and reloads.
func (a *App) SaveSnippets(snippets []config.Snippet) error {
	dir, err := config.ConfigDir()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	var b strings.Builder
	for _, s := range snippets {
		trigger := escapeYAMLValue(s.Trigger)
		text := strings.ReplaceAll(s.Text, "\n", `\n`)
		text = escapeYAMLValue(text)
		b.WriteString(fmt.Sprintf("- trigger: \"%s\"\n  text: \"%s\"\n", trigger, text))
	}
	if err := os.WriteFile(filepath.Join(dir, "snippets.yaml"), []byte(b.String()), 0o644); err != nil {
		return err
	}
	a.reloadDynamicData()
	return nil
}

// HistoryEntry is a frontend-friendly history entry.
type HistoryEntry struct {
	ID          string  `json:"id"`
	Timestamp   string  `json:"timestamp"`
	Language    string  `json:"language"`
	RawText     string  `json:"raw_text"`
	CleanedText string  `json:"cleaned_text"`
	AppContext  string  `json:"app_context"`
	DurationSec float64 `json:"duration_seconds"`
	Backend     string  `json:"backend"`

	Status       string `json:"status"`
	FailedStep   string `json:"failed_step"`
	ErrorMessage string `json:"error_message"`

	// SuspectedHallucination flags a transcript that matched a known Whisper-
	// hallucination pattern. The text was delivered and stored regardless; the
	// flag tells the user to double-check it.
	SuspectedHallucination bool `json:"suspected_hallucination"`

	// HasAudio reports whether the recording is still on disk. Audio is kept
	// for the newest entries only, so this is false for most of the history.
	HasAudio bool `json:"has_audio"`
}

// GetHistory returns the dictation history, newest first.
func (a *App) GetHistory() []HistoryEntry {
	entries := a.hist.Entries()
	// One directory read for the whole list instead of a stat per entry.
	present := a.hist.StoredAudio()

	result := make([]HistoryEntry, len(entries))
	for i, e := range entries {
		result[i] = toHistoryEntry(e, present)
	}
	return result
}

// toHistoryEntry converts a stored entry for the frontend. present is the set of
// recordings on disk, as returned by history.StoredAudio.
func toHistoryEntry(e history.Entry, present map[string]struct{}) HistoryEntry {
	// An empty status comes from an entry written before failures were recorded.
	status := e.Status
	if status == "" {
		status = history.StatusOK
	}
	return HistoryEntry{
		ID:                     e.ID,
		Timestamp:              e.Timestamp.Format(time.RFC3339),
		Language:               e.Language,
		RawText:                e.RawText,
		CleanedText:            e.CleanedText,
		AppContext:             e.AppContext,
		DurationSec:            e.DurationSec,
		Backend:                e.Backend,
		Status:                 status,
		FailedStep:             e.FailedStep,
		ErrorMessage:           e.ErrorMessage,
		SuspectedHallucination: e.SuspectedHallucination,
		HasAudio:               history.HasAudio(e, present),
	}
}

// HistoryInfo describes what the history keeps and what that costs on disk.
type HistoryInfo struct {
	Path       string `json:"path"`
	TextKept   int    `json:"text_kept"`
	AudioKept  int    `json:"audio_kept"`
	AudioFiles int    `json:"audio_files"`
	AudioBytes int64  `json:"audio_bytes"`
	// UsageError is set when the recordings directory could not be read, so the
	// UI can say "unknown" instead of showing a confident zero.
	UsageError string `json:"usage_error,omitempty"`
}

// GetHistoryInfo returns the retention settings and current disk usage.
func (a *App) GetHistoryInfo() HistoryInfo {
	info := HistoryInfo{
		Path:      a.hist.Path(),
		TextKept:  a.hist.MaxSize(),
		AudioKept: a.hist.AudioKeep(),
	}
	files, bytes, err := a.hist.AudioUsage()
	if err != nil {
		logbuf.Warnf(logbuf.StepHistory, "reading the recordings directory: %v", err)
		info.UsageError = err.Error()
		return info
	}
	info.AudioFiles = files
	info.AudioBytes = bytes
	return info
}

// RetryResult reports the outcome of a second transcription attempt.
//
// OK covers the transcription only. Delivery to the clipboard is reported
// separately, because a recovered text that could not be copied must not look
// like a success — the user would try to paste nothing.
type RetryResult struct {
	OK            bool         `json:"ok"`
	Delivered     bool         `json:"delivered"`
	Error         string       `json:"error,omitempty"`
	DeliveryError string       `json:"delivery_error,omitempty"`
	Persisted     bool         `json:"persisted"`
	Entry         HistoryEntry `json:"entry"`
}

// RetryEntry transcribes a stored recording again and updates its history entry.
// When toClipboard is true the recovered text is also copied.
//
// Delivery is deliberately limited to the clipboard: a retry is triggered from
// the vox window, so that window holds the keyboard focus and typing the text
// at "the cursor" would type it into vox itself.
//
// This is the recovery path for a failed attempt — the spoken words are read
// back from the kept recording instead of being spoken again.
func (a *App) RetryEntry(id string, toClipboard bool) RetryResult {
	entry, ok := a.hist.Get(id)
	if !ok {
		return RetryResult{Error: "history entry not found"}
	}

	// Hold the recording for the duration: a dictation finishing meanwhile
	// could otherwise prune exactly this file mid-transcription.
	release := a.hist.HoldAudio(entry)
	defer release()

	audioPath := a.hist.AudioPath(entry)
	if audioPath == "" {
		return RetryResult{Error: fmt.Sprintf("recording no longer available — audio is kept for the newest %d entries", a.hist.AudioKeep())}
	}

	// Reuse the context the recording was made in, so the cleanup tone matches
	// the original situation rather than the vox window. The tone is derived
	// from the app name and the window title, so both are restored.
	var wctx *windowctx.Context
	if entry.AppContext != "" || entry.WindowTitle != "" {
		wctx = &windowctx.Context{AppName: entry.AppContext, WindowTitle: entry.WindowTitle}
	}

	present := a.hist.StoredAudio()

	tr, err := a.transcribeAndCleanup(audioPath, wctx)
	if err != nil {
		step := stepOf(err, logbuf.StepSTT)
		logbuf.Errorf(step, "retry failed: %s", redactURLCredentials(err.Error()))
		if errors.Is(err, apierr.ErrInsufficientCredits) {
			a.notifyInsufficientCredits()
		}

		// Do not demote an entry that already holds a usable transcription:
		// re-transcribing a good entry and hitting a network blip must not turn
		// it into a failure.
		result := RetryResult{Error: redactURLCredentials(err.Error()), Entry: toHistoryEntry(entry, present)}
		if entry.HasText() {
			return result
		}

		entry.Status = history.StatusFailed
		entry.FailedStep = step
		entry.ErrorMessage = storableError(err)
		if _, uerr := a.hist.Update(id, func(e *history.Entry) {
			e.Status = entry.Status
			e.FailedStep = entry.FailedStep
			e.ErrorMessage = entry.ErrorMessage
		}); uerr != nil {
			logbuf.Errorf(logbuf.StepHistory, "could not record the retry failure on entry %s: %v", id, uerr)
		}
		result.Entry = toHistoryEntry(entry, present)
		return result
	}
	if tr.cleanupCreditErr {
		a.notifyInsufficientCredits()
	}

	entry.RawText = tr.raw
	entry.CleanedText = tr.cleaned
	entry.Status = history.StatusOK
	entry.FailedStep = ""
	entry.ErrorMessage = ""
	// A retry re-runs the pattern check, so it can set the mark as well as
	// clear one left by the original attempt.
	entry.SuspectedHallucination = tr.suspectedHallucination

	res := RetryResult{OK: true, Persisted: true, Entry: toHistoryEntry(entry, present)}

	if _, uerr := a.hist.Update(id, func(e *history.Entry) {
		e.RawText = entry.RawText
		e.CleanedText = entry.CleanedText
		e.Status = entry.Status
		e.FailedStep = ""
		e.ErrorMessage = ""
		e.SuspectedHallucination = entry.SuspectedHallucination
	}); uerr != nil {
		if errors.Is(uerr, history.ErrNotFound) {
			return RetryResult{Error: "history entry disappeared during the retry"}
		}
		// The text is recovered but only in memory — say so rather than
		// reporting a clean success.
		logbuf.Errorf(logbuf.StepHistory, "could not save the retry result for entry %s: %v", id, uerr)
		res.Persisted = false
		res.Error = fmt.Sprintf("recovered, but saving to the history failed: %v", uerr)
	}
	logbuf.Infof(logbuf.StepSTT, "retry succeeded for entry %s", id)

	if toClipboard {
		if err := inject.Inject(inject.Clipboard, tr.cleaned); err != nil {
			logbuf.Errorf(logbuf.StepInsert, "clipboard after retry: %v", err)
			res.DeliveryError = err.Error()
			return res
		}
		res.Delivered = true
	}

	return res
}

// maxInlineAudioBytes bounds what GetEntryAudio will hand to the webview. At
// ~1.9 MB per minute this is roughly half an hour of dictation.
const maxInlineAudioBytes = 64 << 20

// AudioData carries a recording to the frontend as base64, so it can be played
// or downloaded from the history view.
type AudioData struct {
	Base64   string `json:"base64"`
	MIMEType string `json:"mime_type"`
	Filename string `json:"filename"`
	Error    string `json:"error,omitempty"`
}

// GetEntryAudio returns the stored recording for an entry.
func (a *App) GetEntryAudio(id string) AudioData {
	entry, ok := a.hist.Get(id)
	if !ok {
		return AudioData{Error: "history entry not found"}
	}
	path := a.hist.AudioPath(entry)
	if path == "" {
		return AudioData{Error: "recording no longer available"}
	}

	// The recording crosses the bridge base64-encoded and becomes a data: URL,
	// so it exists several times over in memory. Refuse the outliers instead of
	// letting the webview choke, and point at the way that always works.
	if info, err := os.Stat(path); err == nil && info.Size() > maxInlineAudioBytes {
		return AudioData{Error: fmt.Sprintf(
			"recording is %d MB — too large to load in the app; use Reveal to open it in the file manager",
			info.Size()/(1024*1024))}
	}

	data, err := os.ReadFile(path)
	if err != nil {
		logbuf.Errorf(logbuf.StepHistory, "reading recording: %v", err)
		return AudioData{Error: err.Error()}
	}
	return AudioData{
		Base64:   base64.StdEncoding.EncodeToString(data),
		MIMEType: "audio/wav",
		Filename: fmt.Sprintf("vox-%s.wav", entry.Timestamp.Format("2006-01-02-150405")),
	}
}

// RevealEntryAudio shows a stored recording in the file manager.
func (a *App) RevealEntryAudio(id string) error {
	entry, ok := a.hist.Get(id)
	if !ok {
		return fmt.Errorf("history entry not found")
	}
	path := a.hist.AudioPath(entry)
	if path == "" {
		return fmt.Errorf("recording no longer available")
	}
	return openpath.Reveal(path)
}

// RevealHistoryFile shows history.jsonl in the file manager. Revealing rather
// than opening it: .jsonl usually has no registered handler, so "open" would
// either fail or launch something unexpected.
func (a *App) RevealHistoryFile() error {
	path := a.hist.Path()
	if path == "" {
		return fmt.Errorf("no history file")
	}
	return openpath.Reveal(path)
}

// CopyToClipboard puts text on the system clipboard.
func (a *App) CopyToClipboard(text string) error {
	return inject.Inject(inject.Clipboard, text)
}

// LogRecord is a frontend-friendly diagnostic record.
type LogRecord struct {
	Time    string `json:"time"`
	Level   string `json:"level"`
	Step    string `json:"step"`
	Message string `json:"message"`
}

// GetLogs returns the diagnostic log, newest first.
func (a *App) GetLogs() []LogRecord {
	records := logbuf.Records()
	out := make([]LogRecord, len(records))
	for i, r := range records {
		out[i] = LogRecord{
			Time:    r.Time.Format(time.RFC3339),
			Level:   r.Level,
			Step:    r.Step,
			Message: r.Message,
		}
	}
	return out
}

// ClearLogs empties the diagnostic log.
func (a *App) ClearLogs() {
	logbuf.Reset()
}

// TestResult holds the result of a backend test.
type TestResult struct {
	OK      bool   `json:"ok"`
	Status  int    `json:"status"`
	Error   string `json:"error,omitempty"`
	Message string `json:"message,omitempty"`
}

// TestSTT tests the STT backend connectivity.
func (a *App) TestSTT() TestResult {
	a.cfg.RLock()
	backend := a.cfg.STTBackend
	sttURL := a.cfg.STTURL
	a.cfg.RUnlock()

	url := "https://api.openai.com/v1/models"
	if backend == "local" {
		u := sttURL
		if u == "" {
			u = "http://localhost:8080"
		}
		url = u + "/v1/models"
	}

	client := &http.Client{Timeout: 5 * time.Second}
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return TestResult{OK: false, Error: redactURLCredentials(err.Error())}
	}
	if backend != "local" {
		if key := a.resolveAPIKey(); key != "" {
			req.Header.Set("Authorization", "Bearer "+key)
		}
	}
	resp, err := client.Do(req)
	if err != nil {
		return TestResult{OK: false, Error: redactURLCredentials(err.Error())}
	}
	resp.Body.Close()
	return TestResult{OK: resp.StatusCode == 200, Status: resp.StatusCode}
}

// TestLLM tests the LLM backend connectivity.
func (a *App) TestLLM() TestResult {
	a.cfg.RLock()
	llmBackend := a.cfg.LLMBackend
	llmURL := a.cfg.LLMURL
	a.cfg.RUnlock()

	if llmBackend == "none" {
		return TestResult{OK: true, Message: "LLM disabled"}
	}

	baseURL := "https://api.openai.com/v1"
	if llmBackend == "ollama" {
		baseURL = llmURL
		if baseURL == "" {
			baseURL = "http://localhost:11434/v1"
		}
	}

	url := baseURL + "/models"
	client := &http.Client{Timeout: 5 * time.Second}
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return TestResult{OK: false, Error: redactURLCredentials(err.Error())}
	}
	if llmBackend != "ollama" {
		if key := a.resolveAPIKey(); key != "" {
			req.Header.Set("Authorization", "Bearer "+key)
		}
	}
	resp, err := client.Do(req)
	if err != nil {
		return TestResult{OK: false, Error: redactURLCredentials(err.Error())}
	}
	resp.Body.Close()
	return TestResult{OK: resp.StatusCode == 200, Status: resp.StatusCode}
}

// SetAPIKey stores an API key in the OS keychain.
func (a *App) SetAPIKey(key, value string) error {
	return keychain.Set("vox", key, value)
}

// DeleteAPIKey removes an API key from the OS keychain.
func (a *App) DeleteAPIKey(key string) error {
	return keychain.Delete("vox", key)
}

// HasAPIKey checks if an API key exists.
func (a *App) HasAPIKey(key string) bool {
	return keychain.HasKey("vox", key)
}

// GetVersion returns the app version.
func (a *App) GetVersion() string {
	return version
}

// GetPermissions returns the current system permission status.
func (a *App) GetPermissions() permissions.Status {
	return permissions.Check()
}

// OpenAccessibilitySettings opens the OS accessibility settings panel.
func (a *App) OpenAccessibilitySettings() {
	permissions.OpenAccessibilitySettings()
}

// OpenMicrophoneSettings opens the OS microphone privacy settings panel.
func (a *App) OpenMicrophoneSettings() {
	permissions.OpenMicrophoneSettings()
}

// ShowWindow brings the settings window to front.
func (a *App) ShowWindow() {
	if a.ui != nil {
		a.ui.ShowWindow()
	}
}

// --- Internal Methods ---

func (a *App) resolveAPIKey() string {
	if key := os.Getenv("OPENAI_API_KEY"); key != "" {
		return key
	}
	if key, err := keychain.Get("vox", "openai-api-key"); err == nil && key != "" {
		return key
	}
	return ""
}

func (a *App) reloadDynamicData() {
	dict, _ := config.LoadDictionary()
	snips, _ := config.LoadSnippets()
	prompts := config.LoadCustomPrompts()
	a.dataMu.Lock()
	a.dictionary = dict
	a.snippets = snips
	a.customPrompts = prompts
	a.dataMu.Unlock()
}

// --- Hotkey ---

func (a *App) startHotkeyListener() {
	a.cfg.RLock()
	key := hotkey.ParseKey(a.cfg.Hotkey)
	a.cfg.RUnlock()

	a.listener = hotkey.New(key)
	go func() {
		if err := a.listener.Listen(a.onPress, a.onRelease); err != nil {
			logbuf.Errorf(logbuf.StepApp, "hotkey listener: %v", err)
		}
	}()
}

func (a *App) restartHotkeyListener() {
	if a.listener != nil {
		a.listener.Close()
	}
	a.startHotkeyListener()
}

func (a *App) isToggleMode() bool {
	a.cfg.RLock()
	defer a.cfg.RUnlock()
	return a.cfg.Mode == "toggle"
}

func (a *App) getDoubletapWindow() time.Duration {
	a.cfg.RLock()
	defer a.cfg.RUnlock()
	return time.Duration(a.cfg.DoubletapWindow) * time.Millisecond
}

func (a *App) getHandsfreeTimeout() time.Duration {
	a.cfg.RLock()
	defer a.cfg.RUnlock()
	return time.Duration(a.cfg.HandsfreeTimeout) * time.Second
}

func (a *App) getNotifications() bool {
	a.cfg.RLock()
	defer a.cfg.RUnlock()
	return a.cfg.Notifications
}

func (a *App) getAudioFeedback() bool {
	a.cfg.RLock()
	defer a.cfg.RUnlock()
	return a.cfg.AudioFeedback
}

func (a *App) getShowOverlay() bool {
	a.cfg.RLock()
	defer a.cfg.RUnlock()
	return a.cfg.ShowOverlay
}

// --- Recording Pipeline ---

func (a *App) startRec() {
	if a.getAudioFeedback() {
		feedback.PlayStart()
	}
	rec, err := audio.Start()
	if err != nil {
		logbuf.Errorf(logbuf.StepRecording, "recording start: %v", err)
		a.setState("idle")
		return
	}
	a.recording = rec
	a.isRecording = true
	// A new capture supersedes any still-processing previous one: bump the
	// generation so a late settleState from that transcription cannot flip the
	// state back to idle while this recording is live.
	a.recordingGen.Add(1)
	a.setState("recording")

	canConsume := hotkey.StartEscapeMonitor(func() {
		a.recordingMu.Lock()
		defer a.recordingMu.Unlock()
		if a.isRecording {
			a.stopAndDiscard()
		}
	})
	if !canConsume {
		logbuf.Warnf(logbuf.StepRecording, "ESC monitor running in degraded mode (listen-only) — grant Accessibility permission to prevent ESC from leaking to the active app")
	}
}

// stopAndProcess must be called with recordingMu held.
func (a *App) stopAndProcess() {
	hotkey.StopEscapeMonitor()
	if a.getAudioFeedback() {
		feedback.PlayStop()
	}
	a.setState("processing")
	gen := a.recordingGen.Add(1)
	rec := a.recording
	a.recording = nil
	a.isRecording = false
	go a.handleStopAndProcess(rec, gen)
}

// stopAndDiscard must be called with recordingMu held.
func (a *App) stopAndDiscard() {
	hotkey.StopEscapeMonitor()
	// A distinct sound, not silence: on a fullscreen space the overlay is
	// invisible (VOX-2), so this can be the only confirmation that the
	// recording was discarded rather than processed.
	if a.getAudioFeedback() {
		feedback.PlayCancel()
	}
	if a.recording != nil {
		r := a.recording
		a.recording = nil
		a.isRecording = false
		go func() {
			// Discarding on purpose, so the file goes either way — Stop returns
			// no path on failure, but may already have written the file.
			if f, _, err := r.Stop(); err == nil {
				os.Remove(f)
			} else if leftover := r.File(); leftover != "" {
				os.Remove(leftover)
			}
		}()
	}
	a.setState("idle")
}

func (a *App) startHandsfree() {
	a.toggleState = false
	if !a.isRecording {
		rec, err := audio.Start()
		if err != nil {
			logbuf.Errorf(logbuf.StepRecording, "recording start: %v", err)
			a.setState("idle")
			return
		}
		a.recording = rec
		a.isRecording = true
		// See startRec: a fresh capture supersedes a still-processing one.
		a.recordingGen.Add(1)
	}
	a.handsfreeActive = true
	if a.getAudioFeedback() {
		feedback.PlayHandsfreeStart()
	}
	a.setState("recording")

	canConsume := hotkey.StartEscapeMonitor(func() {
		a.recordingMu.Lock()
		defer a.recordingMu.Unlock()
		if a.handsfreeActive {
			a.stopHandsfree()
		} else if a.isRecording {
			a.stopAndDiscard()
		}
	})
	if !canConsume {
		logbuf.Warnf(logbuf.StepRecording, "ESC monitor running in degraded mode (listen-only) — grant Accessibility permission to prevent ESC from leaking to the active app")
	}
	a.handsfreeDone = make(chan struct{})
	hfTimeout := a.getHandsfreeTimeout()
	if hfTimeout > 0 {
		a.handsfreeTimer = time.AfterFunc(hfTimeout, func() {
			var shouldNotify bool
			a.recordingMu.Lock()
			if !a.handsfreeActive {
				a.recordingMu.Unlock()
				return
			}
			a.handsfreeActive = false
			if a.handsfreeDone != nil {
				close(a.handsfreeDone)
				a.handsfreeDone = nil
			}
			if a.isRecording && a.recording != nil {
				a.stopAndProcess()
			}
			shouldNotify = a.getNotifications()
			a.recordingMu.Unlock()
			if shouldNotify {
				notify.Send("vox", fmt.Sprintf("Hands-Free stopped after %d:%02d",
					int(hfTimeout.Minutes()), int(hfTimeout.Seconds())%60))
			}
		})
	}
}

func (a *App) stopHandsfree() {
	hotkey.StopEscapeMonitor()
	a.handsfreeActive = false
	a.toggleState = false
	a.doubletapPending = false
	if a.doubletapTimer != nil {
		a.doubletapTimer.Stop()
		a.doubletapTimer = nil
	}
	a.lastReleaseTime = time.Time{}
	if a.handsfreeTimer != nil {
		a.handsfreeTimer.Stop()
		a.handsfreeTimer = nil
	}
	if a.handsfreeDone != nil {
		close(a.handsfreeDone)
		a.handsfreeDone = nil
	}
	if a.isRecording && a.recording != nil {
		a.stopAndProcess()
	}
}

func (a *App) onPress() {
	a.recordingMu.Lock()
	defer a.recordingMu.Unlock()

	now := time.Now()
	dtWindow := a.getDoubletapWindow()

	if a.isToggleMode() {
		if a.handsfreeActive {
			if a.doubletapPending {
				a.doubletapPending = false
				if a.doubletapTimer != nil {
					a.doubletapTimer.Stop()
					a.doubletapTimer = nil
				}
				a.stopHandsfree()
			}
			return
		}
		if a.doubletapPending {
			a.doubletapPending = false
			if a.doubletapTimer != nil {
				a.doubletapTimer.Stop()
			}
			a.startHandsfree()
			return
		}
		return
	}

	// Hold mode
	if a.handsfreeActive {
		if !a.lastReleaseTime.IsZero() && now.Sub(a.lastReleaseTime) < dtWindow {
			a.stopHandsfree()
			return
		}
		a.lastPressTime = now
		return
	}
	if !a.lastReleaseTime.IsZero() && now.Sub(a.lastReleaseTime) < dtWindow && !a.isRecording {
		a.startHandsfree()
		return
	}
	if a.isRecording {
		return
	}
	a.lastPressTime = now
	a.startRec()
}

func (a *App) onRelease() {
	a.recordingMu.Lock()
	defer a.recordingMu.Unlock()

	now := time.Now()
	dtWindow := a.getDoubletapWindow()

	if a.isToggleMode() {
		a.doubletapPending = true
		if a.doubletapTimer != nil {
			a.doubletapTimer.Stop()
		}
		if a.handsfreeActive {
			a.doubletapTimer = time.AfterFunc(dtWindow, func() {
				a.recordingMu.Lock()
				defer a.recordingMu.Unlock()
				if !a.doubletapPending {
					return
				}
				a.doubletapPending = false
			})
			return
		}
		capturedToggleState := a.toggleState
		a.doubletapTimer = time.AfterFunc(dtWindow, func() {
			a.recordingMu.Lock()
			defer a.recordingMu.Unlock()
			if !a.doubletapPending {
				return
			}
			a.doubletapPending = false
			if capturedToggleState {
				a.toggleState = false
				if a.recording != nil {
					a.stopAndProcess()
				}
			} else if !a.isRecording {
				a.toggleState = true
				a.startRec()
			}
		})
		return
	}

	// Hold mode
	if a.handsfreeActive {
		a.lastReleaseTime = now
		return
	}
	if !a.isRecording || a.recording == nil {
		return
	}
	pressDuration := now.Sub(a.lastPressTime)
	if pressDuration < 300*time.Millisecond {
		a.stopAndDiscard()
		a.lastReleaseTime = now
		return
	}
	a.stopAndProcess()
}

// handleStopAndProcess runs everything after the recording stopped: keep the
// audio, transcribe, clean up, deliver the text.
//
// The recording is moved out of the temp directory before the first fallible
// step, and every outcome — success or failure — is written to the history
// with a reference to it. That is what makes a second attempt possible without
// speaking again.
func (a *App) handleStopAndProcess(rec *audio.Recording, gen uint64) {
	tmpAudio, duration, err := rec.Stop()
	if err != nil {
		logbuf.Errorf(logbuf.StepRecording, "recording stop: %v", err)
		// Stop returns no path on failure, but writeWAV may already have created
		// the file — clean it up via the recording's own path.
		if leftover := rec.File(); leftover != "" {
			os.Remove(leftover)
		}
		// There is no audio to keep here, but the attempt is still recorded so
		// the failure is visible instead of silent. The id is assigned up front
		// so the event below can address the stored entry.
		failed := history.Entry{
			ID:           history.NewID(),
			Timestamp:    time.Now(),
			Status:       history.StatusFailed,
			FailedStep:   logbuf.StepRecording,
			ErrorMessage: storableError(err),
		}
		a.addEntry(failed)
		a.emitAttemptFailed(failed)
		a.settleState(gen)
		return
	}

	var wctx *windowctx.Context
	if w, err := windowctx.GetContext(); err == nil {
		wctx = &w
	}
	appCtx, winTitle := "", ""
	if wctx != nil {
		appCtx, winTitle = wctx.AppName, wctx.WindowTitle
	}

	a.cfg.RLock()
	sttBackend := a.cfg.STTBackend
	lang := a.cfg.Language
	output := a.cfg.Output
	a.cfg.RUnlock()

	entryID := history.NewID()
	audioPath, audioName := tmpAudio, ""

	if a.hist.AudioKeep() == 0 {
		// The user asked for no recordings on disk, so the file stays in the
		// temp directory and is removed on the way out.
		defer os.Remove(tmpAudio)
	} else if name, stored, err := a.hist.StoreAudio(entryID, tmpAudio); err != nil {
		// Raised to error level on purpose: this is the one case where a retry
		// will not be possible, so the user should see it. The recording stays in
		// the temp dir for this transcription and is removed on the way out —
		// leaving it would leak an unreferenced file that no entry points at and
		// that CleanOrphans never scans.
		logbuf.Errorf(logbuf.StepRecording, "cannot keep the recording: %v — this attempt cannot be retried", err)
		defer os.Remove(tmpAudio)
	} else {
		audioName, audioPath = name, stored
	}

	entry := history.Entry{
		ID:          entryID,
		Timestamp:   time.Now(),
		Language:    lang,
		AppContext:  appCtx,
		WindowTitle: winTitle,
		DurationSec: duration.Seconds(),
		Backend:     sttBackend,
		AudioFile:   audioName,
		Status:      history.StatusPending,
	}

	// Pin the recording before the entry references it and for the whole read.
	// Once addEntry writes the pending entry, a concurrent dictation's pruneAudio
	// could evict this file — including in the gap before the hold is taken — so
	// hold first: addEntry's own prune and any concurrent one then skip it. No-op
	// when nothing was stored (audio_keep: 0, or StoreAudio failed). Released
	// right after the read — from there the file is subject to normal retention.
	release := a.hist.HoldAudio(entry)

	// Store the attempt before transcribing. Transcription can take minutes, and
	// until an entry references the recording it is invisible in the history and
	// would be swept up as an orphan after a crash.
	a.addEntry(entry)

	tr, err := a.transcribeAndCleanup(audioPath, wctx)
	release()
	if err != nil {
		step := stepOf(err, logbuf.StepSTT)
		logbuf.Errorf(step, "%s", redactURLCredentials(err.Error()))
		if errors.Is(err, apierr.ErrInsufficientCredits) {
			a.notifyInsufficientCredits()
		}
		entry.Status = history.StatusFailed
		entry.FailedStep = step
		entry.ErrorMessage = storableError(err)
		a.updateEntry(&entry)
		a.emitAttemptFailed(entry)
		a.settleState(gen)
		return
	}
	if tr.cleanupCreditErr {
		a.notifyInsufficientCredits()
	}

	entry.RawText = tr.raw
	entry.CleanedText = tr.cleaned
	entry.Status = history.StatusOK
	entry.SuspectedHallucination = tr.suspectedHallucination

	// Save before delivering: once the text is on disk it survives a failure or
	// a crash in the injection step below.
	a.updateEntry(&entry)

	method := inject.ParseMethod(output)
	if injErr := inject.Inject(method, tr.cleaned); injErr != nil {
		logbuf.Errorf(logbuf.StepInsert, "output: %v", injErr)
		// The text itself is intact and already stored — only the delivery
		// failed, so the entry is marked recoverable rather than lost. Stop here:
		// the success notification and the "transcription" event below would tell
		// the user the dictation landed at the cursor when it did not. The failure
		// banner already carries the text for recovery.
		entry.Status = history.StatusFailed
		entry.FailedStep = logbuf.StepInsert
		entry.ErrorMessage = storableError(injErr)
		a.updateEntry(&entry)
		a.emitAttemptFailed(entry)
		a.settleState(gen)
		return
	}

	if a.getNotifications() {
		notify.Send("vox", tr.cleaned)
	}

	if a.ui != nil {
		a.ui.EmitEvent("transcription", map[string]any{
			"raw":       tr.raw,
			"cleaned":   tr.cleaned,
			"suspected": tr.suspectedHallucination,
		})
	}

	a.settleState(gen)
}

func (a *App) addEntry(e history.Entry) {
	if err := a.hist.Add(e); err != nil {
		logbuf.Errorf(logbuf.StepHistory, "writing history: %v", err)
	}
}

// updateEntry persists the current state of e. A failure is logged rather than
// returned: every caller is on a path where the in-memory entry is already the
// truth and the pipeline must carry on.
func (a *App) updateEntry(e *history.Entry) {
	if _, err := a.hist.Update(e.ID, func(stored *history.Entry) {
		*stored = *e
	}); err != nil {
		logbuf.Errorf(logbuf.StepHistory, "saving entry %s: %v", e.ID, err)
	}
}

// maxStoredErrorLen bounds what goes into the history file. Backend errors can
// carry a whole HTTP response body, and the entry is meant to say what went
// wrong, not to archive the response.
const maxStoredErrorLen = 300

// storableError prepares an error for the history file: URL credentials are
// stripped (a self-hosted backend URL may carry one), and the message is
// truncated on a rune boundary so a multibyte character is never cut in half.
func storableError(err error) string {
	if err == nil {
		return ""
	}
	msg := redactURLCredentials(err.Error())
	if r := []rune(msg); len(r) > maxStoredErrorLen {
		msg = string(r[:maxStoredErrorLen]) + "…"
	}
	return msg
}

var (
	urlWithQuery    = regexp.MustCompile(`(https?://[^\s?"]+)\?[^\s"]*`)
	urlWithUserinfo = regexp.MustCompile(`(https?://)[^/\s"@]+@`)
)

// redactURLCredentials removes credentials a backend URL may carry — a
// query-string token and HTTP userinfo (user:pass@), the two forms a self-hosted
// STT/LLM URL usually uses — so an error string can go into the history file, the
// diagnostics log, or a UI banner without leaking them. Applied at every boundary
// that persists or surfaces a backend error. A credential in a URL *path* segment
// is out of scope: redacting whole paths would strip the useful part of most
// error messages, and that placement is rare.
func redactURLCredentials(s string) string {
	s = urlWithQuery.ReplaceAllString(s, "$1?…")
	s = urlWithUserinfo.ReplaceAllString(s, "$1…@")
	return s
}

// settleState returns to idle unless a newer recording has taken over.
func (a *App) settleState(gen uint64) {
	if a.recordingGen.Load() == gen {
		a.setState("idle")
	}
}

// emitAttemptFailed tells the UI that an attempt failed and whether the audio
// is still there to retry from.
func (a *App) emitAttemptFailed(e history.Entry) {
	if a.ui == nil {
		return
	}
	a.ui.EmitEvent("attempt-failed", map[string]any{
		"entry_id":  e.ID,
		"step":      e.FailedStep,
		"message":   e.ErrorMessage,
		"text":      e.CleanedText,
		"can_retry": a.hist.AudioPath(e) != "",
	})
}

// pipelineError carries the pipeline step a failure happened in, so the UI and
// the history entry can name it instead of showing a bare message.
type pipelineError struct {
	step string
	err  error
}

func (e *pipelineError) Error() string { return e.err.Error() }
func (e *pipelineError) Unwrap() error { return e.err }

func pipelineErrf(step string, format string, args ...any) error {
	return &pipelineError{step: step, err: fmt.Errorf(format, args...)}
}

// stepOf extracts the pipeline step from an error, falling back to fallback.
func stepOf(err error, fallback string) string {
	var pe *pipelineError
	if errors.As(err, &pe) {
		return pe.step
	}
	return fallback
}

type transcriptionResult struct {
	raw     string
	cleaned string
	// cleanupCreditErr is true when the cleanup step failed with an
	// insufficient-credits error. The pipeline degrades to the raw text in
	// that case, but the application layer still needs to surface the
	// billing problem to the user.
	cleanupCreditErr bool
	// suspectedHallucination is true when the transcript matched a known
	// Whisper-hallucination pattern. The text is delivered and stored anyway —
	// a false alarm on real dictation must never lose it — and the entry is
	// marked so the user knows to double-check what landed.
	suspectedHallucination bool
}

func (a *App) notifyInsufficientCredits() {
	if a.ui != nil {
		a.ui.EmitEvent("api-error", map[string]string{
			"kind":    "insufficient_credits",
			"message": insufficientCreditsMessage,
		})
	}
	if !a.getNotifications() {
		return
	}
	if err := notify.Send("vox", insufficientCreditsMessage); err != nil {
		logbuf.Warnf(logbuf.StepApp, "notify: %v", err)
	}
}

func (a *App) transcribeAndCleanup(audioFile string, ctx *windowctx.Context) (transcriptionResult, error) {
	a.cfg.RLock()
	sttBackend := a.cfg.STTBackend
	sttURL := a.cfg.STTURL
	sttModel := a.cfg.STTModel
	llmBackend := a.cfg.LLMBackend
	llmURL := a.cfg.LLMURL
	llmModel := a.cfg.LLMModel
	lang := a.cfg.Language
	raw := a.cfg.Raw
	a.cfg.RUnlock()

	apiKey := a.resolveAPIKey()
	sttNeedsKey := sttBackend == "" || sttBackend == "openai"
	llmNeedsKey := llmBackend == "" || llmBackend == "openai"
	if apiKey == "" && (sttNeedsKey || llmNeedsKey) {
		return transcriptionResult{}, pipelineErrf(logbuf.StepApp, "no API key set — configure in Settings")
	}

	a.dataMu.RLock()
	dictionary := a.dictionary
	snippets := a.snippets
	customPrompts := a.customPrompts
	a.dataMu.RUnlock()

	whisperPrompt := buildWhisperPrompt(dictionary, lang)
	transcriber := stt.NewTranscriber(sttBackend, apiKey, sttURL, sttModel)
	rawText, err := transcriber.Transcribe(audioFile, lang, whisperPrompt)
	if err != nil {
		return transcriptionResult{}, pipelineErrf(logbuf.StepSTT, "transcription: %w", err)
	}

	// Empty text is the one outcome that stays a failure: there is nothing to
	// insert. Anything else goes through — a transcript that merely *looks* like
	// a known hallucination is delivered and marked, never dropped: the patterns
	// are fuzzy enough to match real dictation, and spoken words cannot be
	// reconstructed while a hallucination is spotted at a glance.
	if strings.TrimSpace(rawText) == "" {
		return transcriptionResult{}, pipelineErrf(logbuf.StepSTT, "%w", errNoSpeech)
	}
	suspected := isHallucination(rawText)
	if suspected {
		logbuf.Warnf(logbuf.StepSTT, "transcript matches a hallucination pattern (%d chars) — delivered and marked for review", len(rawText))
	}

	result := rawText
	var cleanupCreditErr bool
	if !raw {
		cleaner := cleanup.NewCleanerFromConfig(llmBackend, apiKey, llmURL, llmModel)
		cleaned, err := cleanupWithPrompts(cleaner, rawText, lang, ctx, dictionary, customPrompts)
		if err != nil {
			logbuf.Warnf(logbuf.StepCleanup, "cleanup failed, using raw text: %s", redactURLCredentials(err.Error()))
			if errors.Is(err, apierr.ErrInsufficientCredits) {
				cleanupCreditErr = true
			}
		} else {
			result = cleaned
		}
	}

	if len(snippets) > 0 {
		if expanded, ok := config.MatchSnippet(result, snippets); ok {
			result = expanded
		}
	}

	return transcriptionResult{
		raw:                    rawText,
		cleaned:                result,
		cleanupCreditErr:       cleanupCreditErr,
		suspectedHallucination: suspected,
	}, nil
}

func cleanupWithPrompts(c cleanup.CleanerInterface, text, lang string, ctx *windowctx.Context, dict []string, prompts map[string]string) (string, error) {
	if cp, ok := c.(*cleanup.Cleaner); ok && len(prompts) > 0 {
		return cp.CleanupWithCustomPrompts(text, lang, ctx, dict, prompts)
	}
	return c.Cleanup(text, lang, ctx, dict)
}

var whisperHallucinations = []string{
	"untertitel",
	"amara",
	"subtitles by",
	"vielen dank f",
	"thanks for watching",
	"thank you for watching",
	"bis zum n",
	"www.mooji",
	"copyright watchmojo",
	"please subscribe",
	"bitte abonnieren",
	// Additional YouTube-outro and broadcaster markers. "abonniert den" catches
	// "Abonniert den Kanal" — but stripNonLetters merges across punctuation, so
	// "Zeitung abonniert, den Rest…" matches too. Tolerable: a match only marks.
	"abonniert den",
	// SWR outros usually appear as "Untertitel: SWR YYYY". "untertitel" alone
	// would catch it already, but in the rare case Whisper drops the prefix
	// the "swr YYYY" number pattern is specific enough to be safe.
	"swr 2019",
	"swr 2020",
	"swr 2021",
	"swr 2022",
	"swr 2023",
	"swr 2024",
	"swr 2025",
	"swr 2026",
}

// whisperHallucinationRegexps catches patterns that require structure around
// them (word boundaries, line anchors) — things plain substring matching
// cannot express. Applied against the lowercased, trimmed text BEFORE the
// punctuation-stripping pass.
var whisperHallucinationRegexps = []*regexp.Regexp{
	// Outro URLs at the end of the transcript: "... www.mein-blog.de".
	// Anchored at end of line so legitimate mid-sentence URLs pass through.
	// Restricted to "www.*" to avoid catching bare domains mentioned in
	// dictation (e.g. "die Domain foo.de gehört uns").
	regexp.MustCompile(`\bwww\.[a-z0-9.\-]+\.(de|com|org|net)\b\s*$`),
}

// isHallucination reports whether the text matches a known Whisper-hallucination
// pattern. It is deliberately fuzzy — substring matches over punctuation- and
// umlaut-stripped text hit real dictation too ("Vielen Dank für die Info…") —
// which is acceptable only because a match marks the transcript for review
// instead of dropping it. Empty text is not this function's concern; the
// pipeline treats it as errNoSpeech before ever asking.
func isHallucination(text string) bool {
	normalized := strings.TrimSpace(strings.ToLower(text))
	for _, re := range whisperHallucinationRegexps {
		if re.MatchString(normalized) {
			return true
		}
	}
	stripped := stripNonLetters(normalized)
	for _, h := range whisperHallucinations {
		hStripped := stripNonLetters(h)
		if strings.Contains(stripped, hStripped) {
			return true
		}
	}
	return false
}

// buildWhisperPrompt turns the dictionary into the Whisper prompt. The prompt
// is not an instruction field but decoder context that the model continues,
// so its shape matters more than its wording:
//
//   - Entries that are a prefix of another entry are dropped. "SEPA,
//     SEPA-Lastschrift" is a self-repetition seed: over a silent first window
//     the model continued the pattern and filled the transcript with over a
//     hundred "SEPA," repetitions, eating the first ~36s of real speech
//     (VOX-12). The longer entry biases the vocabulary for both.
//   - The terms are wrapped in a sentence closed with a period, so the most
//     natural continuation is running text rather than another list item.
func buildWhisperPrompt(dictionary []string, lang string) string {
	terms := dropPrefixEntries(dictionary)
	if len(terms) == 0 {
		return ""
	}
	intro := "Technical terms: "
	if lang == "de" {
		intro = "Fachbegriffe: "
	}
	return intro + strings.Join(terms, ", ") + "."
}

// dropPrefixEntries returns the dictionary without blanks, without
// case-insensitive duplicates, and without entries that are a case-insensitive
// prefix of another entry. Order is preserved.
func dropPrefixEntries(entries []string) []string {
	trimmed := make([]string, len(entries))
	lowered := make([]string, len(entries))
	for i, e := range entries {
		trimmed[i] = strings.TrimSpace(e)
		lowered[i] = strings.ToLower(trimmed[i])
	}
	var kept []string
	for i := range entries {
		if lowered[i] == "" {
			continue
		}
		drop := false
		for j := range entries {
			if i == j || lowered[j] == "" {
				continue
			}
			if len(lowered[j]) > len(lowered[i]) && strings.HasPrefix(lowered[j], lowered[i]) {
				drop = true // a longer entry starts with this one
				break
			}
			if j < i && lowered[j] == lowered[i] {
				drop = true // duplicate, the first occurrence is kept
				break
			}
		}
		if !drop {
			kept = append(kept, trimmed[i])
		}
	}
	return kept
}

func stripNonLetters(s string) string {
	var b strings.Builder
	for _, r := range s {
		if r >= 'a' && r <= 'z' || r >= '0' && r <= '9' || r == ' ' {
			b.WriteRune(r)
		}
	}
	return b.String()
}

func escapeYAMLValue(s string) string {
	s = strings.ReplaceAll(s, `\`, `\\`)
	s = strings.ReplaceAll(s, `"`, `\"`)
	return s
}
