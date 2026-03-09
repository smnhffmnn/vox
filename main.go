package main

import (
	"bufio"
	"flag"
	"fmt"
	"os"
	"time"

	"github.com/smnhffmnn/vox/internal/audio"
	"github.com/smnhffmnn/vox/internal/cleanup"
	"github.com/smnhffmnn/vox/internal/inject"
	"github.com/smnhffmnn/vox/internal/stt"
)

func main() {
	lang := flag.String("lang", "de", "Sprache für Transkription (z.B. de, en)")
	output := flag.String("output", "stdout", "Ausgabemethode: stdout, clipboard, wtype, ydotool")
	noCleanup := flag.Bool("raw", false, "LLM-Cleanup überspringen")
	flag.Parse()

	apiKey := os.Getenv("OPENAI_API_KEY")
	if apiKey == "" {
		fatal("OPENAI_API_KEY ist nicht gesetzt")
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

	// Transcribe
	fmt.Fprintln(os.Stderr, "Transcribing...")
	transcriber := stt.NewOpenAI(apiKey)
	raw, err := transcriber.Transcribe(audioFile, *lang)
	if err != nil {
		fatal("Transkription: %v", err)
	}
	fmt.Fprintf(os.Stderr, "> %s\n\n", raw)

	result := raw

	// LLM cleanup
	if !*noCleanup {
		fmt.Fprintln(os.Stderr, "Cleaning up...")
		cleaner := cleanup.NewCleaner(apiKey)
		cleaned, err := cleaner.Cleanup(raw, *lang)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Cleanup fehlgeschlagen, verwende Rohtext: %v\n", err)
		} else {
			result = cleaned
			fmt.Fprintf(os.Stderr, "> %s\n\n", result)
		}
	}

	// Output
	method := inject.ParseMethod(*output)
	if err := inject.Inject(method, result); err != nil {
		fatal("Ausgabe: %v", err)
	}

	if method == inject.Clipboard {
		fmt.Fprintln(os.Stderr, "In Zwischenablage kopiert.")
	}
}

func fatal(format string, args ...any) {
	fmt.Fprintf(os.Stderr, "vox: "+format+"\n", args...)
	os.Exit(1)
}
