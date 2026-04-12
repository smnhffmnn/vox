package main

import (
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/smnhffmnn/vox/internal/audio"
	"github.com/smnhffmnn/vox/internal/cleanup"
	"github.com/smnhffmnn/vox/internal/config"
	"github.com/smnhffmnn/vox/internal/feedback"
	"github.com/smnhffmnn/vox/internal/history"
	"github.com/smnhffmnn/vox/internal/hotkey"
	"github.com/smnhffmnn/vox/internal/inject"
	"github.com/smnhffmnn/vox/internal/keychain"
	"github.com/smnhffmnn/vox/internal/notify"
	"github.com/smnhffmnn/vox/internal/permissions"
	"github.com/smnhffmnn/vox/internal/stt"
	"github.com/smnhffmnn/vox/internal/windowctx"
)

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
		hist:    history.NewHistory(1000),
	}
}

// Start initializes the app (hotkey listener, dynamic data).
// Called by the desktop lifecycle (ServiceStartup) or headless entry.
func (a *App) Start() {
	a.reloadDynamicData()
	a.startHotkeyListener()
	fmt.Fprintln(os.Stderr, "vox: service started")
}

// Shutdown cleans up resources.
func (a *App) Shutdown() {
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
	Timestamp   string  `json:"timestamp"`
	Language    string  `json:"language"`
	RawText     string  `json:"raw_text"`
	CleanedText string  `json:"cleaned_text"`
	AppContext  string  `json:"app_context"`
	DurationSec float64 `json:"duration_seconds"`
	Backend     string  `json:"backend"`
}

// GetHistory returns transcription history.
func (a *App) GetHistory() []HistoryEntry {
	entries := a.hist.Entries()
	result := make([]HistoryEntry, len(entries))
	for i, e := range entries {
		result[i] = HistoryEntry{
			Timestamp:   e.Timestamp.Format(time.RFC3339),
			Language:    e.Language,
			RawText:     e.RawText,
			CleanedText: e.CleanedText,
			AppContext:  e.AppContext,
			DurationSec: e.DurationSec,
			Backend:     e.Backend,
		}
	}
	return result
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
		return TestResult{OK: false, Error: err.Error()}
	}
	if backend != "local" {
		if key := a.resolveAPIKey(); key != "" {
			req.Header.Set("Authorization", "Bearer "+key)
		}
	}
	resp, err := client.Do(req)
	if err != nil {
		return TestResult{OK: false, Error: err.Error()}
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
		return TestResult{OK: false, Error: err.Error()}
	}
	if llmBackend != "ollama" {
		if key := a.resolveAPIKey(); key != "" {
			req.Header.Set("Authorization", "Bearer "+key)
		}
	}
	resp, err := client.Do(req)
	if err != nil {
		return TestResult{OK: false, Error: err.Error()}
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
			fmt.Fprintf(os.Stderr, "vox: hotkey listener: %v\n", err)
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
		fmt.Fprintf(os.Stderr, "vox: recording start: %v\n", err)
		a.setState("idle")
		return
	}
	a.recording = rec
	a.isRecording = true
	a.setState("recording")

	hotkey.StartEscapeMonitor(func() {
		a.recordingMu.Lock()
		defer a.recordingMu.Unlock()
		if a.isRecording {
			a.stopAndDiscard()
		}
	})
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
	if a.recording != nil {
		r := a.recording
		a.recording = nil
		a.isRecording = false
		go func() {
			if f, _, err := r.Stop(); err == nil {
				os.Remove(f)
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
			fmt.Fprintf(os.Stderr, "vox: recording start: %v\n", err)
			a.setState("idle")
			return
		}
		a.recording = rec
		a.isRecording = true
	}
	a.handsfreeActive = true
	if a.getAudioFeedback() {
		feedback.PlayHandsfreeStart()
	}
	a.setState("recording")

	hotkey.StartEscapeMonitor(func() {
		a.recordingMu.Lock()
		defer a.recordingMu.Unlock()
		if a.handsfreeActive {
			a.stopHandsfree()
		} else if a.isRecording {
			a.stopAndDiscard()
		}
	})
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

func (a *App) handleStopAndProcess(rec *audio.Recording, gen uint64) {
	audioFile, duration, err := rec.Stop()
	if err != nil {
		fmt.Fprintf(os.Stderr, "vox: recording stop: %v\n", err)
		if a.recordingGen.Load() == gen {
			a.setState("idle")
		}
		return
	}
	defer os.Remove(audioFile)

	var wctx *windowctx.Context
	if w, err := windowctx.GetContext(); err == nil {
		wctx = &w
	}

	tr, err := a.transcribeAndCleanup(audioFile, wctx)
	if err != nil {
		fmt.Fprintf(os.Stderr, "vox: %v\n", err)
		if a.recordingGen.Load() == gen {
			a.setState("idle")
		}
		return
	}

	appCtx := ""
	if wctx != nil {
		appCtx = wctx.AppName
	}
	a.cfg.RLock()
	sttBackend := a.cfg.STTBackend
	lang := a.cfg.Language
	output := a.cfg.Output
	a.cfg.RUnlock()

	a.hist.Add(history.Entry{
		Timestamp:   time.Now(),
		Language:    lang,
		RawText:     tr.raw,
		CleanedText: tr.cleaned,
		AppContext:  appCtx,
		DurationSec: duration.Seconds(),
		Backend:     sttBackend,
	})

	method := inject.ParseMethod(output)
	if err := inject.Inject(method, tr.cleaned); err != nil {
		fmt.Fprintf(os.Stderr, "vox: output: %v\n", err)
	}

	if a.getNotifications() {
		notify.Send("vox", tr.cleaned)
	}

	if a.ui != nil {
		a.ui.EmitEvent("transcription", map[string]string{
			"raw":     tr.raw,
			"cleaned": tr.cleaned,
		})
	}

	if a.recordingGen.Load() == gen {
		a.setState("idle")
	}
}

type transcriptionResult struct {
	raw     string
	cleaned string
}

func (a *App) transcribeAndCleanup(audioFile string, ctx *windowctx.Context) (transcriptionResult, error) {
	a.cfg.RLock()
	sttBackend := a.cfg.STTBackend
	sttURL := a.cfg.STTURL
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
		return transcriptionResult{}, fmt.Errorf("no API key set — configure in Settings")
	}

	a.dataMu.RLock()
	dictionary := a.dictionary
	snippets := a.snippets
	customPrompts := a.customPrompts
	a.dataMu.RUnlock()

	whisperPrompt := strings.Join(dictionary, ", ")
	transcriber := stt.NewTranscriber(sttBackend, apiKey, sttURL)
	rawText, err := transcriber.Transcribe(audioFile, lang, whisperPrompt)
	if err != nil {
		return transcriptionResult{}, fmt.Errorf("transcription: %w", err)
	}

	if isHallucination(rawText) {
		return transcriptionResult{}, fmt.Errorf("no speech detected")
	}

	result := rawText
	if !raw {
		cleaner := cleanup.NewCleanerFromConfig(llmBackend, apiKey, llmURL, llmModel)
		cleaned, err := cleanupWithPrompts(cleaner, rawText, lang, ctx, dictionary, customPrompts)
		if err != nil {
			fmt.Fprintf(os.Stderr, "cleanup failed, using raw text: %v\n", err)
		} else {
			result = cleaned
		}
	}

	if len(snippets) > 0 {
		if expanded, ok := config.MatchSnippet(result, snippets); ok {
			result = expanded
		}
	}

	return transcriptionResult{raw: rawText, cleaned: result}, nil
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
}

func isHallucination(text string) bool {
	normalized := strings.TrimSpace(strings.ToLower(text))
	if normalized == "" {
		return true
	}
	stripped := stripNonLetters(normalized)
	for _, h := range whisperHallucinations {
		hStripped := stripNonLetters(h)
		if strings.Contains(stripped, hStripped) {
			fmt.Fprintf(os.Stderr, "vox: filtered hallucination: %q\n", text)
			return true
		}
	}
	return false
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
