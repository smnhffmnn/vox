package main

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"os"
	"strings"

	"github.com/smnhffmnn/vox/internal/config"
	"github.com/smnhffmnn/vox/internal/keychain"
	"github.com/smnhffmnn/vox/internal/stt"
)

// TranscribeResult is the JSON output format for the transcribe subcommand.
type TranscribeResult struct {
	Text     string `json:"text"`
	Language string `json:"language"`
	Backend  string `json:"backend"`
}

func runTranscribe(args []string) int {
	fs := flag.NewFlagSet("transcribe", flag.ContinueOnError)
	file := fs.String("f", "", "Audio file to transcribe (required)")
	lang := fs.String("l", "", "Language hint (e.g. de, en) — overrides config")
	asJSON := fs.Bool("json", false, "Output as JSON")
	backend := fs.String("backend", "", "STT backend: openai (default) or local — overrides config")
	sttURL := fs.String("stt-url", "", "URL for local Whisper server — overrides config")
	apiKey := fs.String("api-key", "", "OpenAI API key — overrides env/keychain")

	if err := fs.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return 0
		}
		return 1
	}

	if *file == "" {
		fmt.Fprintln(os.Stderr, "vox transcribe: -f <file> is required")
		fs.Usage()
		return 1
	}

	if _, err := os.Stat(*file); err != nil {
		fmt.Fprintf(os.Stderr, "vox transcribe: %v\n", err)
		return 1
	}

	// Load config for defaults
	cfg, _ := config.Load()

	// Resolve language
	language := cfg.Language
	if *lang != "" {
		language = *lang
	}

	// Resolve backend
	sttBackend := cfg.STTBackend
	if sttBackend == "" {
		sttBackend = "openai"
	}
	if *backend != "" {
		sttBackend = *backend
	}

	// Resolve STT URL
	sttServerURL := cfg.STTURL
	if *sttURL != "" {
		sttServerURL = *sttURL
	}

	// Resolve API key
	key := *apiKey
	if key == "" {
		key = os.Getenv("OPENAI_API_KEY")
	}
	if key == "" {
		if k, err := keychain.Get("vox", "openai-api-key"); err == nil && k != "" {
			key = k
		}
	}
	if key == "" && sttBackend != "local" {
		fmt.Fprintln(os.Stderr, "vox transcribe: no API key — set OPENAI_API_KEY, use -api-key, or configure via keychain")
		return 1
	}

	// Load dictionary for Whisper prompt
	dictionary, _ := config.LoadDictionary()
	whisperPrompt := strings.Join(dictionary, ", ")

	// Transcribe
	transcriber := stt.NewTranscriber(sttBackend, key, sttServerURL)
	text, err := transcriber.Transcribe(*file, language, whisperPrompt)
	if err != nil {
		fmt.Fprintf(os.Stderr, "vox transcribe: %v\n", err)
		return 1
	}

	if *asJSON {
		result := TranscribeResult{
			Text:     text,
			Language: language,
			Backend:  sttBackend,
		}
		enc := json.NewEncoder(os.Stdout)
		enc.SetEscapeHTML(false)
		if err := enc.Encode(result); err != nil {
			fmt.Fprintf(os.Stderr, "vox transcribe: JSON encode: %v\n", err)
			return 1
		}
	} else {
		fmt.Print(text)
	}

	return 0
}
