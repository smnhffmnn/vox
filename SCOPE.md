# vox — Feature Scope & Research

## Vision

Systemweites Open-Source Diktiertool für Linux + macOS (wie WisprFlow). Single Go binary, offline-fähig.

## Feature Scope (priorisiert)

### P0 — Core (MVP, plattformübergreifend)

- [x] Audio-Aufnahme (pw-record auf Linux)
- [x] Audio-Aufnahme auf macOS (sox)
- [x] STT via OpenAI Whisper API
- [x] LLM-Cleanup (Interpunktion, Füllwörter, Fachbegriffe)
- [x] Text-Output: stdout
- [x] Text-Output macOS: pbcopy (clipboard), osascript (keystroke injection)
- [x] Text-Output Linux: wl-copy, wtype, ydotool
- [x] Build-Tags für plattformspezifischen Code (`_linux.go` / `_darwin.go`)
- [x] Systemweiter Hotkey (Hold-to-Talk + Toggle-Modus)
- [x] Daemon-Modus (Hintergrundprozess, reagiert auf Hotkey)

### P1 — Intelligenz

- [x] Kontext-Awareness: fokussierte App erkennen
  - macOS: `osascript` für App-Name/Window-Title (keine Perms nötig)
  - macOS: CGo + Accessibility API für Selected Text, Feldinhalt (optional, mit Perm)
  - Linux X11: `_NET_ACTIVE_WINDOW` + `_NET_WM_NAME` via xdotool/xprop
  - Linux Wayland/GNOME: D-Bus + GNOME Shell Extension
  - Linux Wayland/Sway: `swaymsg -t get_tree`
  - Linux Wayland/Hyprland: `hyprctl activewindow -j`
  - Linux Wayland/KDE: KWin D-Bus Scripting
- [x] Per-App Tone Profiles (casual für Chat, formal für E-Mail, code-aware für IDEs)
- [ ] Konfigurierbarer LLM-Prompt pro Kontext/Modus
- [x] Custom Dictionary/Glossar (Wortliste die ASR und LLM-Cleanup beeinflusst)
- [x] Snippet Library (Sprach-Trigger → Text-Expansion)
- [ ] Offline-Modus: Whisper.cpp HTTP-Server + Ollama für lokales LLM

### P2 — Power Features

- [ ] Auto-Learning aus Korrekturen (Dictionary wächst automatisch)
- [ ] Multi-Language mit Auto-Detection
- [ ] Hands-Free Continuous Mode (konfigurierbares Timeout, bis 6 Min)
- [ ] Command Mode ("lösch den letzten Satz", "mach das als Aufzählung")
- [ ] Konfigurierbares STT-Backend (OpenAI / Groq / Deepgram / lokal)
- [ ] Konfigurierbares LLM-Backend (OpenAI / Anthropic / Ollama / lokal)

### P3 — Polish

- [x] System-Tray-Icon mit Status (idle/recording/processing)
- [x] Desktop-Notification bei fertigem Text
- [x] Audio-Feedback (kurzer Ton bei Start/Stop)
- [x] Config-Datei (~/.config/vox/config.yaml)
- [ ] History/Log der letzten Transkriptionen
- [ ] Latenz-Optimierung (Streaming ASR)
- [ ] Usage-Statistiken

## Architektur

```
main.go                         CLI Entry Point
internal/
  audio/
    recorder.go                 Shared types (Recording, Stop, File)
    recorder_linux.go           pw-record (PipeWire)
    recorder_darwin.go          sox
  stt/
    stt.go                      Transcriber Interface
    openai.go                   OpenAI Whisper API
    local.go                    whisper.cpp HTTP Backend (P1)
  cleanup/
    cleanup.go                  Context-aware LLM Textbereinigung
  inject/
    injector.go                 Method types, ParseMethod, Inject dispatch
    injector_linux.go           wl-copy, wtype, ydotool
    injector_darwin.go          pbcopy, osascript
  windowctx/
    windowctx.go                Context struct
    windowctx_darwin.go         osascript + optional CGo Accessibility
    windowctx_linux.go          Auto-detect compositor, dispatch
  config/
    config.go                   Config file parsing
    dictionary.go               Dictionary loading
    snippets.go                 Snippet loading and matching
  hotkey/
    hotkey.go                   Interface + Key types
    hotkey_linux.go             evdev (/dev/input/event*)
    hotkey_darwin.go            CGo + NSEvent globalMonitor
    hotkey_darwin.c             Objective-C implementation
  tray/
    tray.go                     Tray interface + State types
    tray_enabled.go             fyne.io/systray (build tag: tray)
    tray_disabled.go            No-op stubs (build tag: !tray)
    icondata.go                 Programmatic PNG icon generation
  notify/
    notify_darwin.go            osascript display notification
    notify_linux.go             notify-send
  feedback/
    feedback_darwin.go          afplay with system sounds
    feedback_linux.go           paplay/pw-play with freedesktop sounds
```

## Kontext-Awareness Detail

### Context struct (plattformübergreifend)

```go
type Context struct {
    AppName      string // "Firefox", "Code", "Terminal"
    AppID        string // Bundle ID (macOS) / app_id (Wayland) / WM_CLASS (X11)
    WindowTitle  string // "CLAUDE.md - vox - Visual Studio Code"
    SelectedText string // Aktuell markierter Text (macOS mit Accessibility, Linux via Primary Selection)
}
```

### Kontext → LLM-Prompt Mapping

| App-Kategorie | Erkannt via | Formatierung |
|---|---|---|
| Chat (Slack, Teams, iMessage) | AppName/AppID | Casual, wenig Interpunktion, kein Punkt am Ende |
| E-Mail (Gmail, Outlook) | Window-Title enthält "Mail"/"Gmail" | Formal, korrekte Interpunktion, Anrede |
| IDE (VS Code, Cursor, Terminal) | AppName | Code-aware, camelCase/snake_case, technische Terme |
| Docs (Google Docs, Notes) | AppName | Neutral, saubere Interpunktion |
| Browser (allgemein) | Window-Title für Kontext | Adaptiv basierend auf Seitentitel |

## Differenzierung vs. WisprFlow

| | WisprFlow | vox |
|---|---|---|
| Preis | $12/mo (Pro) | Kostenlos (Open Source) |
| Offline | Nein (cloud-only) | Ja (Whisper.cpp + Ollama) |
| Linux | Nein | Ja (Primärplattform) |
| Privacy | Cloud-Verarbeitung | Lokal möglich |
| Erweiterbar | Nein | Config + Plugins |
| Backend | Proprietär | Wählbar (OpenAI/Anthropic/Groq/lokal) |

## Zielplattformen

- Linux: Fedora (Wayland/PipeWire, GNOME) — Primärplattform
- macOS: Darwin (CoreAudio, Accessibility API)

## Kosten (bei Cloud-Nutzung)

- Whisper API: ~$0.006/Minute
- GPT-4o-mini Cleanup: vernachlässigbar
- Lokal: $0 (eigene Hardware)
