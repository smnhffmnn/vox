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

## Requirements

- Go 1.24+
- Node.js 18+
- [Wails v3 CLI](https://v3.wails.io): `go install github.com/wailsapp/wails/v3/cmd/wails3@latest`
- [Task](https://taskfile.dev): `go install github.com/go-task/task/v3/cmd/task@latest`

### Platform-specific

**macOS:** Xcode Command Line Tools, Accessibility permission (for hotkey), Microphone permission

**Linux (Fedora):**
```bash
sudo dnf install gcc pkg-config webkit2gtk4.1-devel gtk3-devel
# For hotkey support:
sudo usermod -aG input $USER
```

**Windows:** WebView2 runtime (pre-installed on Windows 11)

## Build

```bash
# Development (hot-reload)
wails3 dev

# Production build
wails3 task build

# Package as .app (macOS)
wails3 task darwin:package
```

## Configuration

Config files are stored in `~/.config/vox/` (Linux/macOS) or `%APPDATA%\vox\` (Windows):

| File | Purpose |
|------|---------|
| `config.yaml` | General settings (language, hotkey, backends) |
| `dictionary.txt` | Custom words/phrases (one per line) |
| `snippets.yaml` | Voice trigger → text expansion |
| `history.jsonl` | Transcription log (max 1000 entries) |
| `prompts/*.txt` | Custom LLM prompts per app category |

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
