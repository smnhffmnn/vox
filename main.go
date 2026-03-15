package main

import (
	"bufio"
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"strings"
	"sync"
	"sync/atomic"
	"syscall"
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
	"github.com/smnhffmnn/vox/internal/stt"
	"github.com/smnhffmnn/vox/internal/tray"
	"github.com/smnhffmnn/vox/internal/ui"
	"github.com/smnhffmnn/vox/internal/windowctx"
)

const Version = "0.2.0"

// recordingGen tracks the current recording generation for tray state race prevention.
var recordingGen atomic.Uint64

// pipelineConfig holds all dependencies needed for the record→transcribe→cleanup→inject pipeline.
type pipelineConfig struct {
	cfg           *config.Config
	apiKey        string
	lang          string
	output        string
	raw           bool
	dictionary    []string
	snippets      []config.Snippet
	customPrompts map[string]string
	notifications bool
	audioFeedback bool
	tray          tray.Tray
	history       *history.History
	uiServer      *ui.Server
}

func main() {
	// Check for daemon subcommand before flag parsing
	if len(os.Args) > 1 && os.Args[1] == "daemon" {
		// Strip "daemon" from args so flags work after it
		os.Args = append(os.Args[:1], os.Args[2:]...)
		runDaemon()
		return
	}

	runCLI()
}

// resolveAPIKey returns the OpenAI API key from env or keychain.
// Returns empty string if not found (caller decides if that's an error).
func resolveAPIKey() string {
	if key := os.Getenv("OPENAI_API_KEY"); key != "" {
		return key
	}
	if key, err := keychain.Get("vox", "openai-api-key"); err == nil && key != "" {
		return key
	}
	return ""
}

// requireAPIKey returns the API key or fatals if the backend needs one and it's missing.
func requireAPIKey(cfg *config.Config) string {
	key := resolveAPIKey()
	needsKey := (cfg.STTBackend == "" || cfg.STTBackend == "openai") ||
		(cfg.LLMBackend == "" || cfg.LLMBackend == "openai")
	if key == "" && needsKey {
		fatal("OPENAI_API_KEY ist nicht gesetzt (weder als ENV noch im Keychain)")
	}
	return key
}

func runCLI() {
	// Load config file (defaults if missing)
	cfg, err := config.Load()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Config laden fehlgeschlagen: %v\n", err)
		cfg = config.DefaultConfig()
	}

	// CLI flags (override config values)
	lang := flag.String("lang", cfg.Language, "Sprache für Transkription (z.B. de, en)")
	output := flag.String("output", cfg.Output, "Ausgabemethode: stdout, clipboard, wtype, ydotool")
	noCleanup := flag.Bool("raw", cfg.Raw, "LLM-Cleanup überspringen")
	flag.Parse()

	apiKey := requireAPIKey(cfg)

	// Load dictionary (non-fatal)
	dictionary, err := config.LoadDictionary()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Dictionary laden fehlgeschlagen: %v\n", err)
	}

	// Load snippets (non-fatal)
	snippets, err := config.LoadSnippets()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Snippets laden fehlgeschlagen: %v\n", err)
	}

	// Detect window context (non-fatal)
	var ctx *windowctx.Context
	if wctx, err := windowctx.GetContext(); err == nil {
		ctx = &wctx
	}

	// Start recording
	rec, err := audio.Start()
	if err != nil {
		fatal("Aufnahme: %v", err)
	}

	fmt.Fprintln(os.Stderr, "Recording... (Enter zum Stoppen)")
	start := time.Now()

	// Show elapsed time in background
	done := make(chan struct{})
	go func() {
		ticker := time.NewTicker(100 * time.Millisecond)
		defer ticker.Stop()
		for {
			select {
			case <-done:
				return
			case <-ticker.C:
				fmt.Fprintf(os.Stderr, "\r  %.1fs", time.Since(start).Seconds())
			}
		}
	}()

	// Wait for Enter
	bufio.NewReader(os.Stdin).ReadBytes('\n')
	close(done)

	// Stop recording
	audioFile, duration, err := rec.Stop()
	if err != nil {
		fatal("Aufnahme stoppen: %v", err)
	}
	defer os.Remove(audioFile)
	fmt.Fprintf(os.Stderr, "\r  %.1fs aufgenommen\n\n", duration.Seconds())

	// Transcribe and inject
	pcfg := &pipelineConfig{
		cfg:           cfg,
		apiKey:        apiKey,
		lang:          *lang,
		output:        *output,
		raw:           *noCleanup,
		dictionary:    dictionary,
		snippets:      snippets,
		customPrompts: config.LoadCustomPrompts(),
	}

	tr, err := transcribeAndCleanup(pcfg, audioFile, ctx)
	if err != nil {
		fatal("%v", err)
	}

	method := inject.ParseMethod(pcfg.output)
	if err := inject.Inject(method, tr.cleaned); err != nil {
		fatal("Ausgabe: %v", err)
	}

	if method == inject.Clipboard {
		fmt.Fprintln(os.Stderr, "In Zwischenablage kopiert.")
	}
}

func runDaemon() {
	cfg, err := config.Load()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Config laden fehlgeschlagen: %v\n", err)
		cfg = config.DefaultConfig()
	}

	// CLI flags
	lang := flag.String("lang", cfg.Language, "Sprache für Transkription (z.B. de, en)")
	output := flag.String("output", cfg.Output, "Ausgabemethode: stdout, clipboard, wtype, ydotool")
	noCleanup := flag.Bool("raw", cfg.Raw, "LLM-Cleanup überspringen")
	flag.Parse()

	apiKey := requireAPIKey(cfg)

	dictionary, err := config.LoadDictionary()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Dictionary laden fehlgeschlagen: %v\n", err)
	}

	snippets, err := config.LoadSnippets()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Snippets laden fehlgeschlagen: %v\n", err)
	}

	t := tray.New()
	hist := history.NewHistory(1000)

	// Start UI server
	uiPort := cfg.UIPort
	if uiPort == 0 {
		uiPort = 7890
	}
	uiServer := ui.NewServer(cfg, hist, uiPort, Version)
	uiServer.Start()
	t.SetSettingsPort(uiPort)

	pcfg := &pipelineConfig{
		cfg:           cfg,
		apiKey:        apiKey,
		lang:          *lang,
		output:        *output,
		raw:           *noCleanup,
		dictionary:    dictionary,
		snippets:      snippets,
		customPrompts: config.LoadCustomPrompts(),
		notifications: cfg.Notifications,
		audioFeedback: cfg.AudioFeedback,
		tray:          t,
		history:       hist,
		uiServer:      uiServer,
	}

	// Set up hotkey listener
	key := hotkey.ParseKey(cfg.Hotkey)
	listener := hotkey.New(key)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	var (
		recording   *audio.Recording
		recordingMu sync.Mutex
		isRecording bool

		// Hands-free state
		handsfreeActive bool
		handsfreeTimer  *time.Timer
		handsfreeDone   chan struct{}

		// Double-tap detection
		lastPressTime    time.Time
		lastReleaseTime  time.Time
		doubletapTimer   *time.Timer
		doubletapPending bool
	)

	var toggleState bool

	// Config accessor helpers — read fresh values under RLock for hot-reload support.
	isToggleMode := func() bool {
		cfg.RLock()
		defer cfg.RUnlock()
		return cfg.Mode == "toggle"
	}
	getDoubletapWindow := func() time.Duration {
		cfg.RLock()
		defer cfg.RUnlock()
		return time.Duration(cfg.DoubletapWindow) * time.Millisecond
	}
	getHandsfreeTimeout := func() time.Duration {
		cfg.RLock()
		defer cfg.RUnlock()
		return time.Duration(cfg.HandsfreeTimeout) * time.Second
	}

	setUIState := func(state string) {
		if uiServer != nil {
			uiServer.SetState(state)
		}
	}

	// startRec starts a new audio recording. Must be called with recordingMu held.
	startRec := func() {
		if pcfg.audioFeedback {
			feedback.PlayStart()
		}
		rec, err := audio.Start()
		if err != nil {
			fmt.Fprintf(os.Stderr, "vox: Aufnahme starten: %v\n", err)
			t.SetState(tray.StateIdle)
			t.SetStatus("Error: recording failed")
			setUIState("idle")
			return
		}
		recording = rec
		isRecording = true
		t.SetState(tray.StateRecording)
		t.SetStatus("Recording...")
		setUIState("recording")
		fmt.Fprintln(os.Stderr, "Recording...")
	}

	// stopAndProcess stops the current recording and processes it. Must be called with recordingMu held.
	stopAndProcess := func() {
		if pcfg.audioFeedback {
			feedback.PlayStop()
		}
		t.SetState(tray.StateProcessing)
		t.SetStatus("Processing...")
		setUIState("processing")
		gen := recordingGen.Add(1)
		go handleStopAndProcess(ctx, recording, pcfg, gen)
		recording = nil
		isRecording = false
	}

	// stopAndDiscard stops the current recording and discards it. Must be called with recordingMu held.
	stopAndDiscard := func() {
		if recording != nil {
			r := recording
			recording = nil
			isRecording = false
			go func() {
				if f, _, err := r.Stop(); err == nil {
					os.Remove(f)
				}
			}()
		}
		t.SetState(tray.StateIdle)
		t.SetStatus("Idle")
		setUIState("idle")
	}

	// startHandsfree enters hands-free continuous recording mode. Must be called with recordingMu held.
	startHandsfree := func() {
		toggleState = false

		if !isRecording {
			rec, err := audio.Start()
			if err != nil {
				fmt.Fprintf(os.Stderr, "vox: Aufnahme starten: %v\n", err)
				t.SetState(tray.StateIdle)
				t.SetStatus("Error: recording failed")
				setUIState("idle")
				return
			}
			recording = rec
			isRecording = true
		}

		handsfreeActive = true

		if pcfg.audioFeedback {
			feedback.PlayHandsfreeStart()
		}

		t.SetState(tray.StateRecording)
		setUIState("recording")
		fmt.Fprintln(os.Stderr, "Recording (Hands-Free)...")

		handsfreeDone = make(chan struct{})

		hfTimeout := getHandsfreeTimeout()
		if hfTimeout > 0 {
			deadline := time.Now().Add(hfTimeout)
			t.SetStatus(fmt.Sprintf("Recording (Hands-Free) — %d:%02d remaining",
				int(hfTimeout.Minutes()), int(hfTimeout.Seconds())%60))

			go func(done chan struct{}, dl time.Time) {
				ticker := time.NewTicker(time.Second)
				defer ticker.Stop()
				for {
					select {
					case <-done:
						return
					case <-ticker.C:
						r := time.Until(dl)
						if r < 0 {
							r = 0
						}
						t.SetStatus(fmt.Sprintf("Recording (Hands-Free) — %d:%02d remaining",
							int(r.Minutes()), int(r.Seconds())%60))
					}
				}
			}(handsfreeDone, deadline)

			handsfreeTimer = time.AfterFunc(hfTimeout, func() {
				recordingMu.Lock()
				defer recordingMu.Unlock()

				if !handsfreeActive {
					return
				}

				handsfreeActive = false
				if handsfreeDone != nil {
					close(handsfreeDone)
					handsfreeDone = nil
				}

				fmt.Fprintln(os.Stderr, "Hands-Free timeout reached, stopping...")

				if isRecording && recording != nil {
					stopAndProcess()
				}

				if pcfg.notifications {
					notify.Send("vox", fmt.Sprintf("Hands-Free recording stopped after %d:%02d",
						int(hfTimeout.Minutes()), int(hfTimeout.Seconds())%60))
				}
			})
		} else {
			t.SetStatus("Recording (Hands-Free)")
		}
	}

	// stopHandsfree exits hands-free mode and processes the recording. Must be called with recordingMu held.
	stopHandsfree := func() {
		handsfreeActive = false
		toggleState = false

		// Clear doubletap state to prevent deferred toggle from firing
		doubletapPending = false
		if doubletapTimer != nil {
			doubletapTimer.Stop()
			doubletapTimer = nil
		}
		lastReleaseTime = time.Time{}

		if handsfreeTimer != nil {
			handsfreeTimer.Stop()
			handsfreeTimer = nil
		}

		if handsfreeDone != nil {
			close(handsfreeDone)
			handsfreeDone = nil
		}

		if isRecording && recording != nil {
			stopAndProcess()
		}
	}

	onPress := func() {
		recordingMu.Lock()
		defer recordingMu.Unlock()

		now := time.Now()
		dtWindow := getDoubletapWindow()

		if isToggleMode() {
			// === TOGGLE MODE ===
			if handsfreeActive {
				// During hands-free: check for double-tap exit
				if doubletapPending {
					doubletapPending = false
					if doubletapTimer != nil {
						doubletapTimer.Stop()
					}
					stopHandsfree()
				}
				return
			}

			// Check for double-tap to enter hands-free
			if doubletapPending {
				doubletapPending = false
				if doubletapTimer != nil {
					doubletapTimer.Stop()
				}
				startHandsfree()
				return
			}

			// First press; action deferred to onRelease timer
			return
		}

		// === HOLD MODE ===
		if handsfreeActive {
			// During hands-free: check for double-tap exit
			if !lastReleaseTime.IsZero() && now.Sub(lastReleaseTime) < dtWindow {
				stopHandsfree()
				return
			}
			lastPressTime = now
			return
		}

		// Check for double-tap to enter hands-free
		if !lastReleaseTime.IsZero() && now.Sub(lastReleaseTime) < dtWindow && !isRecording {
			startHandsfree()
			return
		}

		if isRecording {
			return
		}

		lastPressTime = now
		startRec()
	}

	onRelease := func() {
		recordingMu.Lock()
		defer recordingMu.Unlock()

		now := time.Now()
		dtWindow := getDoubletapWindow()

		if isToggleMode() {
			// === TOGGLE MODE ===
			doubletapPending = true
			if doubletapTimer != nil {
				doubletapTimer.Stop()
			}

			if handsfreeActive {
				// During hands-free: timer for exit detection
				doubletapTimer = time.AfterFunc(dtWindow, func() {
					recordingMu.Lock()
					defer recordingMu.Unlock()
					if !doubletapPending {
						return
					}
					doubletapPending = false
					// Single tap during hands-free: ignore
				})
				return
			}

			// Deferred toggle action
			capturedToggleState := toggleState
			doubletapTimer = time.AfterFunc(dtWindow, func() {
				recordingMu.Lock()
				defer recordingMu.Unlock()
				if !doubletapPending {
					return
				}
				doubletapPending = false

				if capturedToggleState {
					// Was recording → stop
					toggleState = false
					if recording != nil {
						stopAndProcess()
					}
				} else {
					// Was idle → start
					toggleState = true
					startRec()
				}
			})
			return
		}

		// === HOLD MODE ===
		if handsfreeActive {
			lastReleaseTime = now
			return
		}

		if !isRecording || recording == nil {
			return
		}

		pressDuration := now.Sub(lastPressTime)
		if pressDuration < 300*time.Millisecond {
			// Short tap: discard recording, remember for double-tap detection
			stopAndDiscard()
			lastReleaseTime = now
			return
		}

		// Normal hold release: process
		stopAndProcess()
	}

	fmt.Fprintf(os.Stderr, "vox daemon gestartet (hotkey: %s, mode: %s, doubletap: %dms)\n", cfg.Hotkey, cfg.Mode, cfg.DoubletapWindow)

	// Handle SIGINT/SIGTERM for graceful shutdown
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	// Start hotkey listener in background
	go func() {
		if err := listener.Listen(onPress, onRelease); err != nil {
			fmt.Fprintf(os.Stderr, "vox: Hotkey listener: %v\n", err)
		}
	}()

	// Signal → cancel context
	go func() {
		select {
		case <-sigCh:
			cancel()
		case <-ctx.Done():
		}
	}()

	// Shutdown coordinator: clean up on context cancel
	go func() {
		<-ctx.Done()

		recordingMu.Lock()

		// Cancel hands-free timers
		if handsfreeTimer != nil {
			handsfreeTimer.Stop()
			handsfreeTimer = nil
		}
		if handsfreeDone != nil {
			close(handsfreeDone)
			handsfreeDone = nil
		}
		handsfreeActive = false

		// Cancel doubletap timer
		if doubletapTimer != nil {
			doubletapTimer.Stop()
		}
		doubletapPending = false

		// Stop active recording and clean up temp file
		rec := recording
		recording = nil
		isRecording = false
		recordingMu.Unlock()

		if rec != nil {
			if audioFile, _, err := rec.Stop(); err == nil {
				os.Remove(audioFile)
			}
		}

		listener.Close()
		t.Quit()
	}()

	// Run tray (blocks on main thread — required for macOS)
	t.Run(func() {
		// Tray is ready
	}, func() {
		// Tray quit callback
		cancel()
	})

	fmt.Fprintln(os.Stderr, "\nvox daemon beendet.")
}

// handleStopAndProcess stops a recording, transcribes, cleans up, and injects.
// gen is the recording generation at the time this was started — used to avoid
// resetting tray state if a new recording has started since.
func handleStopAndProcess(ctx context.Context, rec *audio.Recording, pcfg *pipelineConfig, gen uint64) {
	audioFile, duration, err := rec.Stop()
	if err != nil {
		fmt.Fprintf(os.Stderr, "vox: Aufnahme stoppen: %v\n", err)
		if pcfg.tray != nil && recordingGen.Load() == gen {
			pcfg.tray.SetState(tray.StateIdle)
			pcfg.tray.SetStatus("Error: recording failed")
			if pcfg.uiServer != nil {
				pcfg.uiServer.SetState("idle")
			}
		}
		return
	}
	defer os.Remove(audioFile)
	fmt.Fprintf(os.Stderr, "  %.1fs aufgenommen\n", duration.Seconds())

	// Abort if shutdown was requested
	select {
	case <-ctx.Done():
		return
	default:
	}

	// Detect window context
	var wctx *windowctx.Context
	if w, err := windowctx.GetContext(); err == nil {
		wctx = &w
	}

	tr, err := transcribeAndCleanup(pcfg, audioFile, wctx)
	if err != nil {
		fmt.Fprintf(os.Stderr, "vox: %v\n", err)
		if pcfg.tray != nil && recordingGen.Load() == gen {
			pcfg.tray.SetState(tray.StateIdle)
			pcfg.tray.SetStatus("Error")
			if pcfg.uiServer != nil {
				pcfg.uiServer.SetState("idle")
			}
		}
		return
	}

	// Record to history
	if pcfg.history != nil {
		appCtx := ""
		if wctx != nil {
			appCtx = wctx.AppName
		}
		pcfg.cfg.RLock()
		sttBackend := pcfg.cfg.STTBackend
		pcfg.cfg.RUnlock()
		pcfg.history.Add(history.Entry{
			Timestamp:   time.Now(),
			Language:    pcfg.lang,
			RawText:     tr.raw,
			CleanedText: tr.cleaned,
			AppContext:  appCtx,
			DurationSec: duration.Seconds(),
			Backend:     sttBackend,
		})
	}

	method := inject.ParseMethod(pcfg.output)
	if err := inject.Inject(method, tr.cleaned); err != nil {
		fmt.Fprintf(os.Stderr, "vox: Ausgabe: %v\n", err)
	}

	// Notification
	if pcfg.notifications {
		notify.Send("vox", tr.cleaned)
	}

	// Reset tray only if no new recording has started
	if pcfg.tray != nil && recordingGen.Load() == gen {
		pcfg.tray.SetState(tray.StateIdle)
		pcfg.tray.SetStatus("Idle")
		if pcfg.uiServer != nil {
			pcfg.uiServer.SetState("idle")
		}
	}
}

// transcriptionResult holds both raw and cleaned text from the pipeline.
type transcriptionResult struct {
	raw     string
	cleaned string
}

// transcribeAndCleanup runs the STT → cleanup → snippet-match pipeline.
func transcribeAndCleanup(pcfg *pipelineConfig, audioFile string, ctx *windowctx.Context) (transcriptionResult, error) {
	pcfg.cfg.RLock()
	sttBackend := pcfg.cfg.STTBackend
	sttURL := pcfg.cfg.STTURL
	llmBackend := pcfg.cfg.LLMBackend
	llmURL := pcfg.cfg.LLMURL
	llmModel := pcfg.cfg.LLMModel
	pcfg.cfg.RUnlock()

	whisperPrompt := strings.Join(pcfg.dictionary, ", ")
	transcriber := stt.NewTranscriber(sttBackend, pcfg.apiKey, sttURL)
	raw, err := transcriber.Transcribe(audioFile, pcfg.lang, whisperPrompt)
	if err != nil {
		return transcriptionResult{}, fmt.Errorf("Transkription: %w", err)
	}
	fmt.Fprintf(os.Stderr, "> %s\n", raw)

	result := raw

	if !pcfg.raw {
		cleaner := cleanup.NewCleanerFromConfig(llmBackend, pcfg.apiKey, llmURL, llmModel)
		cleaned, err := cleanupWithPrompts(cleaner, raw, pcfg.lang, ctx, pcfg.dictionary, pcfg.customPrompts)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Cleanup fehlgeschlagen, verwende Rohtext: %v\n", err)
		} else {
			result = cleaned
			fmt.Fprintf(os.Stderr, "> %s\n", result)
		}
	}

	if len(pcfg.snippets) > 0 {
		if expanded, ok := config.MatchSnippet(result, pcfg.snippets); ok {
			result = expanded
		}
	}

	return transcriptionResult{raw: raw, cleaned: result}, nil
}

// cleanupWithPrompts calls CleanupWithCustomPrompts if the cleaner supports it, otherwise Cleanup.
func cleanupWithPrompts(c cleanup.CleanerInterface, text, lang string, ctx *windowctx.Context, dict []string, prompts map[string]string) (string, error) {
	if cp, ok := c.(*cleanup.Cleaner); ok && len(prompts) > 0 {
		return cp.CleanupWithCustomPrompts(text, lang, ctx, dict, prompts)
	}
	return c.Cleanup(text, lang, ctx, dict)
}

func fatal(format string, args ...any) {
	fmt.Fprintf(os.Stderr, "vox: "+format+"\n", args...)
	os.Exit(1)
}
