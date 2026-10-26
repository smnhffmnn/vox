//go:build nogui

package main

import (
	"fmt"
	"os"
)

func printUsage() {
	fmt.Fprintf(os.Stderr, `vox — Speech-to-Text (headless build)

This build has no desktop app — no tray, no hotkey, no settings window —
and needs no display: it transcribes audio files from the command line.

Usage:
  vox transcribe -f <file>              Transcribe an audio file
  vox transcribe -f <file> -l de        Transcribe with language hint
  vox transcribe -f <file> --json       Output as JSON
  vox transcribe -h                     All transcribe flags
  vox --version                         Print version
  vox --help                            Show this help

API key: -api-key, else OPENAI_API_KEY, else the OS keychain (secret-tool on Linux)
Config:  ~/.config/vox/config.yaml and dictionary.txt
More info: https://github.com/smnhffmnn/vox
`)
}

func runDesktop() {
	fmt.Fprintln(os.Stderr, "vox: this is a headless build — desktop GUI is not available")
	fmt.Fprintln(os.Stderr, "      use 'vox transcribe -f <file>' for transcription")
	os.Exit(1)
}

// headlessUI implements UIBridge as no-ops.
type headlessUI struct{}

func (h *headlessUI) SetTrayIcon(icon []byte)         {}
func (h *headlessUI) SetTrayLabel(label string)       {}
func (h *headlessUI) ShowOverlay(x, y int)            {}
func (h *headlessUI) HideOverlay()                    {}
func (h *headlessUI) EmitEvent(name string, data any) {}
func (h *headlessUI) ShowWindow()                     {}
