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
	"github.com/smnhffmnn/vox/internal/hotkey"
	"github.com/smnhffmnn/vox/internal/inject"
	"github.com/smnhffmnn/vox/internal/notify"
	"github.com/smnhffmnn/vox/internal/stt"
	"github.com/smnhffmnn/vox/internal/tray"
	"github.com/smnhffmnn/vox/internal/windowctx"
)

// recordingGen tracks the current recording generation for tray state race prevention.
var recordingGen atomic.Uint64

// pipelineConfig holds all dependencies needed for the record→transcribe→cleanup→inject pipeline.
type pipelineConfig struct {
	apiKey        string
	lang          string
	output        string
	raw           bool
	dictionary    []string
	snippets      []config.Snippet
	notifications bool
	audioFeedback bool
	tray          tray.Tray
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

	apiKey := os.Getenv("OPENAI_API_KEY")
	if apiKey == "" {
		fatal("OPENAI_API_KEY ist nicht gesetzt")
	}

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
		apiKey:     apiKey,
		lang:       *lang,
		output:     *output,
		raw:        *noCleanup,
		dictionary: dictionary,
		snippets:   snippets,
	}

	result, err := transcribeAndCleanup(pcfg, audioFile, ctx)
	if err != nil {
		fatal("%v", err)
	}

	method := inject.ParseMethod(pcfg.output)
	if err := inject.Inject(method, result); err != nil {
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

	apiKey := os.Getenv("OPENAI_API_KEY")
	if apiKey == "" {
		fatal("OPENAI_API_KEY ist nicht gesetzt")
	}

	dictionary, err := config.LoadDictionary()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Dictionary laden fehlgeschlagen: %v\n", err)
	}

	snippets, err := config.LoadSnippets()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Snippets laden fehlgeschlagen: %v\n", err)
	}

	t := tray.New()

	pcfg := &pipelineConfig{
		apiKey:        apiKey,
		lang:          *lang,
		output:        *output,
		raw:           *noCleanup,
		dictionary:    dictionary,
		snippets:      snippets,
		notifications: cfg.Notifications,
		audioFeedback: cfg.AudioFeedback,
		tray:          t,
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
	)

	toggleMode := cfg.Mode == "toggle"
	var toggleState bool

	onPress := func() {
		recordingMu.Lock()
		defer recordingMu.Unlock()

		if toggleMode {
			if toggleState {
				toggleState = false
				if recording != nil {
					if pcfg.audioFeedback {
						feedback.PlayStop()
					}
					t.SetState(tray.StateProcessing)
					t.SetStatus("Processing...")
					gen := recordingGen.Add(1)
					go handleStopAndProcess(ctx, recording, pcfg, gen)
					recording = nil
					isRecording = false
				}
				return
			}
			toggleState = true
		}

		if isRecording {
			return
		}

		if pcfg.audioFeedback {
			feedback.PlayStart()
		}

		rec, err := audio.Start()
		if err != nil {
			fmt.Fprintf(os.Stderr, "vox: Aufnahme starten: %v\n", err)
			t.SetState(tray.StateIdle)
			t.SetStatus("Error: recording failed")
			return
		}
		recording = rec
		isRecording = true
		t.SetState(tray.StateRecording)
		t.SetStatus("Recording...")
		fmt.Fprintln(os.Stderr, "Recording...")
	}

	onRelease := func() {
		if toggleMode {
			return
		}

		recordingMu.Lock()
		defer recordingMu.Unlock()

		if !isRecording || recording == nil {
			return
		}

		if pcfg.audioFeedback {
			feedback.PlayStop()
		}
		t.SetState(tray.StateProcessing)
		t.SetStatus("Processing...")
		gen := recordingGen.Add(1)
		go handleStopAndProcess(ctx, recording, pcfg, gen)
		recording = nil
		isRecording = false
	}

	fmt.Fprintf(os.Stderr, "vox daemon gestartet (hotkey: %s, mode: %s)\n", cfg.Hotkey, cfg.Mode)

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

		// Stop active recording and clean up temp file
		recordingMu.Lock()
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

	result, err := transcribeAndCleanup(pcfg, audioFile, wctx)
	if err != nil {
		fmt.Fprintf(os.Stderr, "vox: %v\n", err)
		if pcfg.tray != nil && recordingGen.Load() == gen {
			pcfg.tray.SetState(tray.StateIdle)
			pcfg.tray.SetStatus("Error")
		}
		return
	}

	method := inject.ParseMethod(pcfg.output)
	if err := inject.Inject(method, result); err != nil {
		fmt.Fprintf(os.Stderr, "vox: Ausgabe: %v\n", err)
	}

	// Notification
	if pcfg.notifications {
		notify.Send("vox", result)
	}

	// Reset tray only if no new recording has started
	if pcfg.tray != nil && recordingGen.Load() == gen {
		pcfg.tray.SetState(tray.StateIdle)
		pcfg.tray.SetStatus("Idle")
	}
}

// transcribeAndCleanup runs the STT → cleanup → snippet-match pipeline.
func transcribeAndCleanup(pcfg *pipelineConfig, audioFile string, ctx *windowctx.Context) (string, error) {
	whisperPrompt := strings.Join(pcfg.dictionary, ", ")
	transcriber := stt.NewOpenAI(pcfg.apiKey)
	raw, err := transcriber.Transcribe(audioFile, pcfg.lang, whisperPrompt)
	if err != nil {
		return "", fmt.Errorf("Transkription: %w", err)
	}
	fmt.Fprintf(os.Stderr, "> %s\n", raw)

	result := raw

	if !pcfg.raw {
		cleaner := cleanup.NewCleaner(pcfg.apiKey)
		cleaned, err := cleaner.Cleanup(raw, pcfg.lang, ctx, pcfg.dictionary)
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

	return result, nil
}

func fatal(format string, args ...any) {
	fmt.Fprintf(os.Stderr, "vox: "+format+"\n", args...)
	os.Exit(1)
}
