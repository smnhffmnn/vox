//go:build nogui

package main

import (
	"fmt"
	"os"
)

func printUsage() {
	fmt.Fprintf(os.Stderr, `vox — Speech-to-Text (headless)

Usage:
  vox transcribe -f <file>         Transcribe an audio file
  vox transcribe -f <file> -l en   Transcribe with language hint
  vox transcribe -f <file> --json  Output as JSON
  vox --version                    Print version
  vox --help                       Show this help

Config: ~/.config/vox/config.yaml
`)
}

func runDesktop() {
	fmt.Fprintln(os.Stderr, "vox: this is a headless build — desktop GUI is not available")
	fmt.Fprintln(os.Stderr, "      use 'vox transcribe -f <file>' for transcription")
	os.Exit(1)
}

// headlessUI implements UIBridge as no-ops.
type headlessUI struct{}

func (h *headlessUI) SetTrayIcon(icon []byte)       {}
func (h *headlessUI) SetTrayLabel(label string)      {}
func (h *headlessUI) ShowOverlay(x, y int)           {}
func (h *headlessUI) HideOverlay()                   {}
func (h *headlessUI) EmitEvent(name string, data any) {}
func (h *headlessUI) ShowWindow()                    {}
