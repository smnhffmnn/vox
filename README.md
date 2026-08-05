# Vox

Cross-platform speech-to-text dictation tool. Press hotkey → speak → text appears at cursor.

## Features

- **Hold-to-talk / Toggle / Hands-free** recording modes with configurable hotkey
- **Context-aware cleanup** — adapts tone based on active app (chat=casual, email=formal, IDE=technical)
- **STT backends** — OpenAI Whisper (cloud) or local Whisper server
- **LLM cleanup** — OpenAI, Ollama (local), or none (raw text)
- **Dictionary** — custom word list improves recognition accuracy
- **Snippets** — voice triggers expand to predefined text
- **History** — searchable log with stored recordings: play, re-transcribe, copy raw or polished text
- **No lost dictation** — the recording is kept before the first fallible step, so a failure after that point can be transcribed again instead of spoken again
- **Diagnostics** — in-app log naming the step and cause of a failure, no terminal needed
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

### CLI Transcribe

`vox transcribe` transcribes audio files with LLM cleanup enabled by default (same pipeline as the desktop app). Use `--raw` to get the unprocessed STT output.

```bash
vox transcribe -f recording.wav            # transcribed + cleaned
vox transcribe -f recording.wav --raw      # raw STT output only
vox transcribe -f recording.wav -json      # JSON output
```

## Configuration

Most settings are manageable through the built-in UI; the retention keys below are edited in `config.yaml` directly. Config files are stored in `~/.config/vox/` on all platforms, Windows included (`%USERPROFILE%\.config\vox\`) — only the file-based API-key fallback lives under `%APPDATA%\vox\`:

| File | Purpose |
|------|---------|
| `config.yaml` | General settings (language, hotkey, recording mode, backends, retention) |
| `dictionary.txt` | Custom words/phrases for improved recognition (one per line) |
| `snippets.yaml` | Voice trigger → text expansion mappings |
| `history.jsonl` | Dictation log, successful and failed (max `history_size` entries, default 1000) |
| `recordings/*.wav` | Kept recordings for the newest `audio_keep` entries (default 30) — the source for a retry |
| `prompts/*.txt` | Custom LLM prompts per app category (chat, email, ide, docs, browser) |

### Retention

Two keys in `config.yaml`, edited in the file only — they are not in the UI. Both are read at
startup, so a change takes effect on the next start.

| Key | Default | Meaning |
|-----|---------|---------|
| `history_size` | 1000 | Entries whose text is kept in `history.jsonl`. Zero, negative and non-numeric values are ignored and the default applies. |
| `audio_keep` | 30 | Newest entries whose recording is kept in `recordings/`. `0` keeps no recording at all — nothing is written to the config directory, and no attempt can be retried. Negative values are ignored. |

Text and audio are deliberately independent: an entry costs about 1 KB of text, but a minute of
audio is about 1.9 MB. There is no size cap on top of `audio_keep` — the History view reports how
much space the recordings actually use instead.

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
icons_*.go           Tray icon generation (desktop/headless)
internal/
├── apierr/           Typed API errors (insufficient credits)
├── audio/            Audio recording (malgo/miniaudio — native on all platforms)
├── cleanup/          LLM text cleanup with context-aware tone
├── config/           YAML config, dictionary, snippets, custom prompts
├── feedback/         Audio feedback (start/stop/handsfree sounds)
├── history/          JSONL dictation history + audio retention
├── hotkey/           Global hotkey (NSEvent/evdev/SetWindowsHookEx)
├── inject/           Text output (clipboard/keystroke injection)
├── keychain/         OS keychain (Keychain/secret-tool/file-based)
├── logbuf/           In-memory diagnostic log surfaced in the UI
├── notify/           Desktop notifications
├── openpath/         Reveal a file in the platform file manager
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
│           ├── HistoryView.svelte    Dictation log, playback, re-transcribe
│           ├── LogView.svelte        Diagnostics log (steps, causes, filters)
│           └── AboutView.svelte      Version, credits
```

## Tech Stack

- **Backend:** Go + Wails v3
- **Frontend:** Svelte 5 + TypeScript + Vite
- **Audio:** malgo (miniaudio) — CoreAudio / WASAPI / PulseAudio
- **STT:** OpenAI Whisper API or local Whisper-compatible server
- **LLM:** OpenAI GPT-4o-mini / Ollama / none

## Versioning

**Pre-1.0** (current):

| Bump | Meaning |
|------|---------|
| Patch (`0.x.Y`) | Bugfixes — no user-visible behavior changes |
| Minor (`0.X.y`) | New features, new tools, backwards-compatible changes |
| Major (`X.y.z`) | Breaking changes (config format, CLI interface, storage format) |

**Post-1.0:** Strict [Semantic Versioning](https://semver.org) (MAJOR.MINOR.PATCH).

## Manual Smoke Test

Automated tests cover pure helpers, config parsing, history (retention, untrusted audio references, interrupted attempts), the diagnostic buffer, cleanup prompt assembly, and STT HTTP boundaries. Everything below requires a running binary, real hardware, or OS integration and must be smoke-tested by hand before a release.

Run through this list on each target platform. Items marked _(platform)_ apply only to that OS.

### Start & Permissions

- [ ] Binary starts without errors, no stderr noise beyond the banner.
- [ ] Tray icon appears and switches between Idle / Recording / Processing states.
- [ ] First launch prompts for required permissions; Settings → Status reflects the current grant state.
- [ ] _(macOS)_ Accessibility and Microphone prompts appear; revoking and relaunching re-prompts.
- [ ] _(Linux)_ User is in the `input` group (or equivalent); hotkey grab succeeds on both Wayland and X11.
- [ ] _(Windows)_ WebView2 is detected; missing runtime produces an actionable error.

### Hotkey & Recording Modes

- [ ] Hold-to-talk: hold hotkey → status goes to Recording → release → cleaned text appears at the cursor.
- [ ] Toggle: press once to start, press again to stop; text is injected after stop.
- [ ] Hands-free: double-tap starts continuous mode; double-tap again (or timeout) stops it.
- [ ] Escape during a recording discards the take — no injection, tray returns to Idle.
- [ ] _(macOS)_ ESC during recording does **not** leak to the active app (e.g. in a Claude Code shell, the running process is not interrupted). Requires Accessibility permission granted.
- [ ] Very short hotkey tap (< 300ms) is discarded, not transcribed.
- [ ] Changing the hotkey in Settings takes effect without a restart.

### Audio

- [ ] Default microphone is used; switching the system default between sessions picks up the new device.
- [ ] Audio feedback sounds (start/stop/hands-free) play when enabled and stay silent when disabled.
- [ ] Output device does not switch to Bluetooth during recording (regression guard for `NoFixedSizedCallback` / `FormatUnknown`).
- [ ] Plugging/unplugging a USB/Bluetooth mic between recordings does not crash the app.

### Transcription & Output

- [ ] OpenAI backend: save API key → Test button turns green → real recording produces correct text.
- [ ] Local backend: point `stt_url` at a running Whisper server → Test button green → recording works offline.
- [ ] Backend switch in Settings is applied to the next recording without a restart.
- [ ] Text injection via clipboard restores the prior clipboard contents.
- [ ] Text injection via keystroke produces the exact string (no lost characters, no autocorrect clobbering).

### Context-aware Cleanup, Snippets & Dictionary

- [ ] IDE window (e.g. VS Code, JetBrains) in focus → cleanup uses the technical tone.
- [ ] Chat window (e.g. Slack, iMessage) in focus → cleanup uses the casual tone.
- [ ] Email client in focus → cleanup uses the formal tone.
- [ ] Snippets: speaking a configured trigger phrase expands to the mapped text at the cursor.
- [ ] Dictionary: a custom word added to `dictionary.txt` is spelled correctly in the transcription.
- [ ] Cleanup backend set to "none" outputs the raw STT result unchanged.

### Failure recovery

Force each failure and confirm the recording survives it. For the STT, cleanup and insert
failures, `~/.config/vox/recordings/` must still hold the WAV afterwards and the history must
show the attempt as failed. The last two items are the deliberate exceptions.

- [ ] Wrong API key → attempt fails at the STT step, banner names it, `Try again → clipboard` recovers the text after fixing the key.
- [ ] Airplane mode / no network → same, and the recording is still there.
- [ ] Silence only → "no speech detected" is recorded as a failed attempt, and the recording is kept rather than discarded.
- [ ] ESC during recording → recording is discarded on purpose; no history entry, no leftover WAV.
- [ ] Kill the app during transcription → the entry is in `history.jsonl` as `pending` and the WAV survives; the next start turns it into a failed attempt that can be transcribed again.
- [ ] Set `audio_keep: 0`, dictate → no file appears in `recordings/` at any point, and the history offers no retry.

### UI

- [ ] Overlay appears during Recording/Processing when enabled and stays hidden when disabled.
- [ ] Settings window opens from the tray menu and persists changes after restart.
- [ ] Dark / light mode follows the system setting and updates live when the OS theme changes.
- [ ] History view lists entries newest first, and the search box filters over raw text, polished text, app context and error message.
- [ ] History rows of the newest `audio_keep` entries offer Play, Re-transcribe, Download and Reveal; older rows offer only the copy actions.
- [ ] Re-transcribe on a stored recording replaces the entry's text without a new recording.
- [ ] `Reveal history.jsonl` opens the file manager — _(macOS/Windows)_ with the file selected, _(Linux)_ at the containing directory.
- [ ] Diagnostics view lists steps and causes, filters to warnings/errors, and copies the log to the clipboard.
- [ ] About view shows the correct version string.

### Platform-specific

- [ ] _(macOS)_ `.app` bundle launches after `xattr -cr /Applications/vox.app` without a Gatekeeper block.
- [ ] _(macOS)_ Keystroke injection works into native apps (Terminal, Notes) and Electron apps (VS Code, Slack).
- [ ] _(Linux)_ Hotkey fires under both Wayland and X11 sessions (Fedora ships both).
- [ ] _(Linux)_ Keystroke injection works under the active display server; clipboard fallback works otherwise.
- [ ] _(Windows)_ `SetWindowsHookEx`-based hotkey fires system-wide, including over elevated windows where possible.
- [ ] _(Windows)_ WebView2 renders the UI without blank panels on a fresh user profile.

## License

MIT — see [LICENSE](LICENSE)
