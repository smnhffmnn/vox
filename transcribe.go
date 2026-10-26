package main

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"os"

	"github.com/smnhffmnn/vox/internal/cleanup"
	"github.com/smnhffmnn/vox/internal/config"
	"github.com/smnhffmnn/vox/internal/keychain"
	"github.com/smnhffmnn/vox/internal/stt"
)

// TranscribeResult is the JSON output format for the transcribe subcommand.
type TranscribeResult struct {
	Text     string `json:"text"`
	Language string `json:"language"`
	Backend  string `json:"backend"`
	// Segments are the raw transcript's timestamped spans when the backend
	// provided them (whisper-1). They describe the STT output, not the
	// cleaned text — that is what makes them useful for diagnosing gaps and
	// hallucinations against the audio.
	Segments []stt.Segment `json:"segments,omitempty"`
}

func runTranscribe(args []string) int {
	fs := flag.NewFlagSet("transcribe", flag.ContinueOnError)
	file := fs.String("f", "", "Audio file to transcribe (required)")
	lang := fs.String("l", "", "Language hint (e.g. de, en) — overrides config")
	asJSON := fs.Bool("json", false, "Output as JSON")
	raw := fs.Bool("raw", false, "Skip LLM cleanup, return raw STT output")
	backend := fs.String("backend", "", "STT backend: openai (default) or local — overrides config")
	sttURL := fs.String("stt-url", "", "URL for local Whisper server — overrides config")
	apiKey := fs.String("api-key", "", "OpenAI API key — overrides OPENAI_API_KEY and the OS keychain")
	llmBackendFlag := fs.String("llm-backend", "", "LLM backend: openai, ollama, none — overrides config")
	llmURLFlag := fs.String("llm-url", "", "LLM base URL — overrides config")

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

	// Load config for defaults. A config.yaml that exists but cannot be read
	// is a setup error, not "no config": falling back to the defaults would
	// silently turn a local STT setup into an upload to OpenAI.
	cfg, err := config.Load()
	if err != nil {
		fmt.Fprintf(os.Stderr, "vox transcribe: config.yaml exists but cannot be read: %v\n", err)
		return 1
	}

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

	// Resolve the cleanup backend before the key check — it needs the key
	// as much as OpenAI STT does.
	llmBackend := cfg.LLMBackend
	if llmBackend == "" {
		llmBackend = "openai"
	}
	if *llmBackendFlag != "" {
		llmBackend = *llmBackendFlag
	}
	llmURL := cfg.LLMURL
	if *llmURLFlag != "" {
		llmURL = *llmURLFlag
	}

	// Resolve the API key only when OpenAI does the STT or the cleanup — a
	// local Whisper server and Ollama never see it, and looking it up would
	// shell out to the OS keychain for nothing. When it is needed, it is
	// required up front. Without the check, a local STT run would still post
	// the transcript to api.openai.com for cleanup — unauthenticated, failing
	// only after the text has left the machine. The desktop pipeline requires
	// the key up front too, but resolves it unconditionally and decides the
	// need with its own backend checks (transcribeAndCleanup in app.go).
	var key string
	sttNeedsKey := sttBackend != "local"
	llmNeedsKey := !*raw && llmBackend != "none" && llmBackend != "ollama"
	if sttNeedsKey || llmNeedsKey {
		var keyErr error
		key, keyErr = lookupAPIKey(*apiKey, os.Getenv, keychain.Get)
		if key == "" {
			fmt.Fprintf(os.Stderr, "vox transcribe: %v\n", keyErr)
			return 1
		}
	}

	// Load dictionary for Whisper prompt. Shaped like the app's prompt — the
	// raw comma join was a repetition-hallucination seed (VOX-12).
	dictionary, _ := config.LoadDictionary()
	whisperPrompt := buildWhisperPrompt(dictionary, language)

	// Transcribe
	transcriber := stt.NewTranscriber(sttBackend, key, sttServerURL, cfg.STTModel)
	res, err := transcriber.Transcribe(*file, language, whisperPrompt)
	if err != nil {
		fmt.Fprintf(os.Stderr, "vox transcribe: %v\n", err)
		return 1
	}
	text := res.Text

	// LLM cleanup (default: on, --raw disables)
	if !*raw {
		cleaner := cleanup.NewCleanerFromConfig(llmBackend, key, llmURL, cfg.LLMModel)
		cleaned, err := cleaner.Cleanup(text, language, nil, dictionary)
		if err != nil {
			fmt.Fprintf(os.Stderr, "vox transcribe: cleanup failed, using raw text: %v\n", err)
		} else {
			text = cleaned
		}
	}

	if *asJSON {
		result := TranscribeResult{
			Text:     text,
			Language: language,
			Backend:  sttBackend,
			Segments: res.Segments,
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

// lookupAPIKey resolves the OpenAI key: the flag wins, then OPENAI_API_KEY,
// then the OS keychain. getenv and keychainGet are parameters so a test can
// stage every combination without touching the real keychain — on a developer
// Mac that one holds a real key.
//
// When nothing yields a key, the returned error says how to provide one. On
// a headless server there is usually no keychain (no secret-tool), and the
// hint names only the flag and the environment instead of offering it. Where
// a keychain helper is installed, the hint offers the keychain as well, and
// whatever the keychain answered — no such key, no Secret Service on the bus,
// a helper that did not answer in time — is quoted after it, so the reason is
// not lost. The keychain error stays wrapped for errors.Is.
func lookupAPIKey(flagKey string, getenv func(string) string, keychainGet func(service, key string) (string, error)) (string, error) {
	if flagKey != "" {
		return flagKey, nil
	}
	if k := getenv("OPENAI_API_KEY"); k != "" {
		return k, nil
	}
	k, err := keychainGet("vox", "openai-api-key")
	if err == nil && k != "" {
		return k, nil
	}
	if errors.Is(err, keychain.ErrUnavailable) {
		return "", fmt.Errorf("no API key — set OPENAI_API_KEY or use -api-key (%w)", err)
	}
	if err != nil {
		return "", fmt.Errorf("no API key — set OPENAI_API_KEY, use -api-key, or store it in the OS keychain (%w)", err)
	}
	return "", errors.New("no API key — set OPENAI_API_KEY, use -api-key, or store it in the OS keychain")
}
