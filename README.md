# vox

Cross-platform speech-to-text tool for Linux and macOS. Hotkey drücken, sprechen, Text erscheint. Context-aware cleanup passt Ton und Formatierung an die aktive App an.

## How it works

1. **Recording** — `pw-record` (Linux/PipeWire) or `sox` (macOS) captures 16kHz mono WAV
2. **Transcription** — OpenAI Whisper API with optional dictionary hints
3. **Cleanup** — GPT-4o-mini cleans punctuation, filler words, technical terms — adapts tone to focused app (chat vs. email vs. IDE)
4. **Snippet matching** — Trigger phrases expand to predefined text
5. **Injection** — Text output via stdout, clipboard, or keystroke simulation

## Prerequisites

### Both platforms
- Go 1.25+
- `OPENAI_API_KEY` environment variable

### Linux
- PipeWire (`pw-record`)
- Optional: `wl-clipboard` (clipboard), `wtype` (Wayland keystroke), `ydotool` (universal keystroke)
- Window context: `xdotool`/`xprop` (X11), `swaymsg` (sway), `hyprctl` (Hyprland)

### macOS
- sox (`brew install sox`)
- `pbcopy` (built-in, clipboard)
- `osascript` (built-in, keystroke injection + window context)

## Installation

```bash
go build -o vox .
```

## Usage

```bash
# Default: German, stdout
./vox

# English, clipboard
./vox -lang en -output clipboard

# Type directly into focused window (Linux/Wayland)
./vox -output wtype

# Type directly into focused window (macOS)
./vox -output wtype   # maps to osascript keystroke on macOS

# Skip LLM cleanup (raw transcription)
./vox -raw
```

Press Enter to stop recording.

## Flags

| Flag | Default | Description |
|------|---------|-------------|
| `-lang` | `de` | Transcription language (`de`, `en`, ...) |
| `-output` | `stdout` | Output method: `stdout`, `clipboard`, `wtype`, `ydotool` |
| `-raw` | `false` | Skip LLM cleanup |

Defaults can be set in the config file — CLI flags always take priority.

## Configuration

All config files live in `~/.config/vox/`.

### config.yaml

```yaml
language: de
output: clipboard
raw: false
```

### dictionary.txt

One word/phrase per line. Improves Whisper recognition and tells the LLM to use exact spellings.

```
Kubernetes
GitHub
MYFIT24
TypeScript
```

Lines starting with `#` are comments.

### snippets.yaml

Map voice triggers to text expansions. After cleanup, if the result exactly matches a trigger (case-insensitive), the snippet text is injected instead.

```yaml
- trigger: "kalenderlink"
  text: "https://cal.com/simon/30min"
- trigger: "signatur"
  text: "Mit freundlichen Grüßen\nSimon Hoffmann"
```

## Context-aware cleanup

vox detects the focused application and adapts the LLM cleanup tone:

| Category | Apps | Tone |
|----------|------|------|
| Chat | Slack, Teams, Discord, Telegram, Signal, Messages | Casual, short sentences, no trailing period |
| Email | Mail, Gmail, Outlook, Thunderbird | Formal, correct punctuation, complete sentences |
| IDE | VS Code, Cursor, IntelliJ, PhpStorm, Terminal, iTerm | Technical, preserves camelCase/snake_case |
| Docs | Pages, Docs, Word, Notes, Notion, Obsidian | Neutral, clean punctuation |
| Browser | Firefox, Chrome, Safari, Arc, Brave | Neutral |

## Architecture

```
main.go                              CLI, flow orchestration
internal/
  audio/
    recorder.go                      Shared types (Recording, Stop, File)
    recorder_linux.go                pw-record (PipeWire)
    recorder_darwin.go               sox
  stt/
    stt.go                           Transcriber interface
    openai.go                        OpenAI Whisper API
  cleanup/
    cleanup.go                       Context-aware LLM text cleanup
  inject/
    injector.go                      Method types, ParseMethod, Inject dispatch
    injector_linux.go                wl-copy, wtype, ydotool
    injector_darwin.go               pbcopy, osascript keystroke
  windowctx/
    windowctx.go                     Context struct
    windowctx_darwin.go              osascript (app name, window title, bundle ID)
    windowctx_linux.go               Auto-detect: sway/Hyprland/GNOME/X11
  config/
    config.go                        Config file parsing
    dictionary.go                    Dictionary loading
    snippets.go                      Snippet loading and matching
```

Single binary, no external Go dependencies. API calls via `net/http`.

## Cost

- Whisper API: ~$0.006/minute
- GPT-4o-mini cleanup: negligible
