package main

import (
	"bufio"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"strings"
	"sync"
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
					go handleStopAndProcess(recording, pcfg)
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
		go handleStopAndProcess(recording, pcfg)
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

	// Run tray (blocks on main thread — required for macOS)
	// If no tray (notray build), this blocks until quit signal.
	t.Run(func() {
		// Tray is ready — set up signal handler to quit
		go func() {
			<-sigCh
			fmt.Fprintln(os.Stderr, "\nvox daemon beendet.")
			listener.Close()
			os.Exit(0)
		}()
	}, func() {
		// Tray quit callback
		fmt.Fprintln(os.Stderr, "vox daemon beendet.")
		listener.Close()
		os.Exit(0)
	})
}

// handleStopAndProcess stops a recording, transcribes, cleans up, and injects.
func handleStopAndProcess(rec *audio.Recording, pcfg *pipelineConfig) {
	audioFile, duration, err := rec.Stop()
	if err != nil {
		fmt.Fprintf(os.Stderr, "vox: Aufnahme stoppen: %v\n", err)
		if pcfg.tray != nil {
			pcfg.tray.SetState(tray.StateIdle)
			pcfg.tray.SetStatus("Error: recording failed")
		}
		return
	}
	defer os.Remove(audioFile)
	fmt.Fprintf(os.Stderr, "  %.1fs aufgenommen\n", duration.Seconds())

	// Detect window context
	var ctx *windowctx.Context
	if wctx, err := windowctx.GetContext(); err == nil {
		ctx = &wctx
	}

	result, err := transcribeAndCleanup(pcfg, audioFile, ctx)
	if err != nil {
		fmt.Fprintf(os.Stderr, "vox: %v\n", err)
		if pcfg.tray != nil {
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

	// Reset tray
	if pcfg.tray != nil {
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
