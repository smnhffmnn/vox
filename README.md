# vox

Cross-platform speech-to-text tool for Linux and macOS. Hotkey drücken, sprechen, Text erscheint. Context-aware cleanup passt Ton und Formatierung an die aktive App an.

## How it works

1. **Recording** — `pw-record` (Linux/PipeWire) or `sox` (macOS) captures 16kHz mono WAV
2. **Transcription** — OpenAI Whisper API or local Whisper server with optional dictionary hints
3. **Cleanup** — LLM cleans punctuation, filler words, technical terms — adapts tone to focused app (chat vs. email vs. IDE)
4. **Snippet matching** — Trigger phrases expand to predefined text
5. **Injection** — Text output via stdout, clipboard, or keystroke simulation

## Prerequisites

### Both platforms
- Go 1.25+
- `OPENAI_API_KEY` environment variable or key stored in OS keychain

### Linux
- PipeWire (`pw-record`)
- Optional: `wl-clipboard` (clipboard), `wtype` (Wayland keystroke), `ydotool` (universal keystroke)
- Window context: `xdotool`/`xprop` (X11), `swaymsg` (sway), `hyprctl` (Hyprland)
- Notifications: `notify-send`
- Audio feedback: `paplay` or `pw-play`
- Keychain: `secret-tool` (libsecret)

### macOS
- sox (`brew install sox`)
- `pbcopy` (built-in, clipboard)
- `osascript` (built-in, keystroke injection + window context + notifications)
- Accessibility permission required for global hotkey monitoring
- Keychain: built-in macOS Keychain (via `security` command)

## Installation

```bash
# Standard build (no tray icon)
go build -o vox .

# With system tray icon (requires CGo)
go build -tags tray -o vox .
```

## Usage

### CLI mode (one-shot)

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

### Daemon mode (background, hotkey-triggered)

```bash
# Start daemon with default settings
./vox daemon

# Daemon with custom output method
./vox daemon -output clipboard

# Daemon with English transcription
./vox daemon -lang en
```

The daemon runs in the background and listens for a global hotkey. When pressed, it starts recording; when released (hold mode) or pressed again (toggle mode), it stops, transcribes, cleans up, and injects the text.

In daemon mode, the web UI starts automatically on `http://localhost:7890` (configurable via `ui_port`).

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

# Daemon settings
hotkey: right_option    # right_option, right_alt, f13-f20
mode: hold              # hold (hold-to-talk) or toggle (press to start/stop)
notifications: true     # Desktop notification after transcription
audio_feedback: true    # Sound on recording start/stop

# Backend
stt_backend: openai     # openai | local
stt_url: ""             # URL for local Whisper server (default: http://localhost:8080)
llm_backend: openai     # openai | ollama | none
llm_url: ""             # Base URL for Ollama (default: http://localhost:11434/v1)
llm_model: ""           # Model override (default: gpt-4o-mini for openai, llama3.2 for ollama)

# UI
ui_port: 7890           # Web UI port
```

#### Hotkey options

| Key | macOS | Linux |
|-----|-------|-------|
| `right_option` / `right_alt` | Right Option (⌥) | Right Alt |
| `f13` - `f20` | F13-F20 | F13-F20 |

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

### Custom prompts

Override the default LLM system prompt per app category by creating text files in `~/.config/vox/prompts/`:

```
~/.config/vox/prompts/chat.txt     # Custom prompt for chat apps
~/.config/vox/prompts/email.txt    # Custom prompt for email apps
~/.config/vox/prompts/ide.txt      # Custom prompt for IDEs/terminals
~/.config/vox/prompts/docs.txt     # Custom prompt for document editors
~/.config/vox/prompts/browser.txt  # Custom prompt for browsers
~/.config/vox/prompts/default.txt  # Fallback prompt
```

If a file exists for a category, its content replaces the default system prompt entirely.

## Settings UI

In daemon mode, vox runs an embedded web UI for configuration:

- **URL:** `http://localhost:7890` (default port, configurable via `ui_port`)
- **Access:** Tray icon → "Settings" menu item, or open the URL directly
- **Sections:** General settings, API & Backend config, Dictionary editor, Snippet editor, History viewer, Status

The UI is a single embedded HTML file — no external dependencies, no build step.

## Backend Configuration

### OpenAI (default)

Requires `OPENAI_API_KEY` environment variable or key stored in keychain.

### Local Whisper (STT)

Run a Whisper-compatible HTTP server locally and set `stt_backend: local` in config:

```yaml
stt_backend: local
stt_url: "http://localhost:8080"
```

### Ollama (LLM cleanup)

Run Ollama locally and set `llm_backend: ollama`:

```yaml
llm_backend: ollama
llm_url: "http://localhost:11434/v1"
llm_model: "llama3.2"
```

### No cleanup

Skip LLM cleanup entirely (raw transcription output):

```yaml
llm_backend: none
```

## API Key Management

API keys are stored in the OS keychain (macOS Keychain / Linux libsecret), never in config files.

**Priority:** Environment variable `OPENAI_API_KEY` → OS Keychain → error

**Set via CLI (macOS):**
```bash
security add-generic-password -U -s vox -a openai-api-key -w "sk-..."
```

**Set via CLI (Linux):**
```bash
echo -n "sk-..." | secret-tool store --label="vox: openai-api-key" service vox key openai-api-key
```

**Set via UI:** Open Settings → API & Backend → paste key → Save Key

## History

Transcription history is stored in `~/.config/vox/history.jsonl` (max 1000 entries, auto-rotated). View history in the web UI Settings → History tab, or read the JSONL file directly.

## Context-aware cleanup

vox detects the focused application and adapts the LLM cleanup tone:

| Category | Apps | Tone |
|----------|------|------|
| Chat | Slack, Teams, Discord, Telegram, Signal, Messages | Casual, short sentences, no trailing period |
| Email | Mail, Gmail, Outlook, Thunderbird | Formal, correct punctuation, complete sentences |
| IDE | VS Code, Cursor, IntelliJ, PhpStorm, Terminal, iTerm | Technical, preserves camelCase/snake_case |
| Docs | Pages, Docs, Word, Notes, Notion, Obsidian | Neutral, clean punctuation |
| Browser | Firefox, Chrome, Safari, Arc, Brave | Neutral |

## System tray

When built with `-tags tray`, the daemon shows a system tray icon with three states:

- **Gray circle** — Idle
- **Red circle** — Recording
- **Orange circle** — Processing (transcribing/cleaning)

The tray menu shows status, a "Settings" link (opens web UI), and a "Quit" option. Without the tray build tag, the daemon runs headless.

## Architecture

```
main.go                              CLI + daemon, flow orchestration
internal/
  audio/
    recorder.go                      Shared types (Recording, Stop, File)
    recorder_linux.go                pw-record (PipeWire)
    recorder_darwin.go               sox
  stt/
    stt.go                           Transcriber interface + factory
    openai.go                        OpenAI Whisper API
    local.go                         Local Whisper HTTP backend
  cleanup/
    cleanup.go                       Context-aware LLM text cleanup + factory
  inject/
    injector.go                      Method types, ParseMethod, Inject dispatch
    injector_linux.go                wl-copy, wtype, ydotool
    injector_darwin.go               pbcopy, osascript keystroke
  windowctx/
    windowctx.go                     Context struct
    windowctx_darwin.go              osascript (app name, window title, bundle ID)
    windowctx_linux.go               Auto-detect: sway/Hyprland/GNOME/X11
  config/
    config.go                        Config file parsing + Save
    dictionary.go                    Dictionary loading
    snippets.go                      Snippet loading and matching
    prompts.go                       Custom prompt loading per category
  keychain/
    keychain.go                      Keychain interface
    keychain_darwin.go               macOS security command
    keychain_linux.go                Linux secret-tool
  history/
    history.go                       JSONL transcription history
  ui/
    server.go                        Web UI HTTP server + REST API
    static/index.html                Embedded settings SPA
  hotkey/
    hotkey.go                        Interface + Key types
    hotkey_linux.go                  evdev (/dev/input/event*)
    hotkey_darwin.go                 CGo + NSEvent globalMonitor
    hotkey_darwin.c                  Objective-C implementation
  tray/
    tray.go                          Tray interface
    tray_enabled.go                  fyne.io/systray (build tag: tray)
    tray_disabled.go                 No-op stubs (build tag: !tray)
    icondata.go                      Programmatic PNG icon generation
  notify/
    notify_darwin.go                 osascript display notification
    notify_linux.go                  notify-send
  feedback/
    feedback_darwin.go               afplay with system sounds
    feedback_linux.go                paplay/pw-play
```

## Build tags

| Tag | Effect |
|-----|--------|
| `tray` | Enable system tray icon (requires fyne.io/systray, CGo on macOS) |
| (none) | Headless build, no tray, no CGo dependency for tray |

Note: The hotkey system always requires CGo on macOS (NSEvent monitoring). On Linux, no CGo is needed.

## Cost

- Whisper API: ~$0.006/minute
- GPT-4o-mini cleanup: negligible
- Local (Whisper.cpp + Ollama): $0
