# Vox

Cross-platform speech-to-text dictation tool. Press hotkey → speak → text appears at cursor.

## Features

- **Hold-to-talk / Toggle / Hands-free** recording modes with configurable hotkey
- **Context-aware cleanup** — adapts tone based on active app (chat=casual, email=formal, IDE=technical)
- **STT backends** — OpenAI Whisper (cloud) or local Whisper server
- **LLM cleanup** — OpenAI, Ollama (local), or none (raw text)
- **Dictionary** — custom word list improves recognition accuracy
- **Snippets** — voice triggers expand to predefined text
- **History** — searchable transcription log
- **System color scheme** — inherits OS dark/light mode
- **Native desktop app** — Wails v3 + Svelte 5, single binary

## Platforms

| Platform | Status |
|----------|--------|
| macOS (arm64, amd64) | Tested |
| Linux (Fedora, Wayland/X11) | Supported |
| Windows (amd64) | Supported |

## Installation

### Homebrew (macOS)

```bash
brew tap smnhffmnn/tap

# Desktop app (recommended) — installs to /Applications
brew install --cask vox

# CLI binary only — installs to PATH
brew install vox
```

### GitHub Releases

Download the latest release from [GitHub Releases](https://github.com/smnhffmnn/vox/releases).

> **macOS Gatekeeper:** The app is not code-signed. If macOS blocks it, run:
> ```bash
> xattr -cr /Applications/vox.app
> ```

### Build from source

**Requirements:**

- Go 1.25+
- Node.js 18+
- [Wails v3 CLI](https://v3.wails.io): `go install github.com/wailsapp/wails/v3/cmd/wails3@latest`

**Platform-specific:**

- **macOS:** Xcode Command Line Tools, Accessibility permission (for hotkey), Microphone permission
- **Linux (Fedora):** `sudo dnf install gcc pkg-config webkit2gtk4.1-devel gtk3-devel` and `sudo usermod -aG input $USER` for hotkey support
- **Windows:** WebView2 runtime (pre-installed on Windows 11)

```bash
git clone https://github.com/smnhffmnn/vox.git
cd vox
make build
```

The binary is at `build/bin/vox`. On macOS, a `.app` bundle is created at `build/bin/vox.app`.

## Quick Start

1. Launch Vox — it appears as a system tray icon
2. Set your OpenAI API key in **Backends** (or configure a local Whisper server)
3. Press the hotkey (default: Right Option/Alt) and speak
4. Release — your text appears at the cursor

**Recording modes:**

| Mode | How it works |
|------|-------------|
| Hold-to-talk | Hold hotkey → speak → release |
| Toggle | Press once to start, press again to stop |
| Hands-free | Double-tap hotkey → speaks until timeout or double-tap again |

## Configuration

All settings are manageable through the built-in UI. Config files are stored in `~/.config/vox/` (Linux/macOS) or `%APPDATA%\vox\` (Windows):

| File | Purpose |
|------|---------|
| `config.yaml` | General settings (language, hotkey, recording mode, backends) |
| `dictionary.txt` | Custom words/phrases for improved recognition (one per line) |
| `snippets.yaml` | Voice trigger → text expansion mappings |
| `history.jsonl` | Transcription log (max 1000 entries) |
| `prompts/*.txt` | Custom LLM prompts per app category (chat, email, ide, docs, browser) |

### STT Backends

| Backend | Config | Notes |
|---------|--------|-------|
| OpenAI Whisper | API key required | Cloud-based, high accuracy |
| Local Whisper | `stt_url` → your server | [whisper.cpp](https://github.com/ggerganov/whisper.cpp) HTTP server, fully offline |

### LLM Backends

| Backend | Config | Notes |
|---------|--------|-------|
| OpenAI | API key required | GPT-4o-mini for cleanup |
| Ollama | `llm_url` → your Ollama instance | Fully offline, any model |
| None | — | Raw transcription, no cleanup |

## Architecture

```
main.go              Wails v3 app bootstrap + system tray
app.go               App struct with all frontend bindings + recording pipeline
icons.go             Tray icon generation
internal/
├── audio/            Audio recording (malgo/miniaudio — native on all platforms)
├── cleanup/          LLM text cleanup with context-aware tone
├── config/           YAML config, dictionary, snippets, custom prompts
├── feedback/         Audio feedback (start/stop/handsfree sounds)
├── history/          JSONL transcription history
├── hotkey/           Global hotkey (NSEvent/evdev/SetWindowsHookEx)
├── inject/           Text output (clipboard/keystroke injection)
├── keychain/         OS keychain (Keychain/secret-tool/file-based)
├── notify/           Desktop notifications
├── permissions/      Permission checks (Accessibility, Microphone)
├── stt/              Speech-to-text (OpenAI Whisper, local)
└── windowctx/        Active window detection for context-aware cleanup
frontend/
├── src/
│   ├── App.svelte    Root layout with sidebar navigation
│   ├── app.css       Global styles with system color scheme
│   └── lib/
│       ├── api.ts    Typed Wails binding wrappers
│       ├── stores.ts Svelte stores
│       └── components/
│           ├── StatusView.svelte     State, permissions, uptime
│           ├── SettingsView.svelte   Language, hotkey, mode config
│           ├── BackendsView.svelte   STT/LLM config, API key management
│           ├── DictionaryView.svelte Word list editor
│           ├── SnippetsView.svelte   Trigger/text editor
│           ├── HistoryView.svelte    Transcription log
│           └── AboutView.svelte      Version, credits
```

## Tech Stack

- **Backend:** Go + Wails v3
- **Frontend:** Svelte 5 + TypeScript + Vite
- **Audio:** malgo (miniaudio) — CoreAudio / WASAPI / PulseAudio
- **STT:** OpenAI Whisper API or local Whisper-compatible server
- **LLM:** OpenAI GPT-4o-mini / Ollama / none

## License

MIT — see [LICENSE](LICENSE)
