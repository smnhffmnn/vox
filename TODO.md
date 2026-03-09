# vox — Speech-to-Text Tool für Linux

## Vision
WisprFlow-ähnliche Erfahrung auf Linux: Sprechen → sauberer Text im aktiven Fenster.

## Architektur-Entscheidungen

### Sprache: Go
- Single binary, kein Dependency-Hell (Python wäre die Alternative gewesen)
- Schneller Startup, ideal für ein Tool das per Hotkey getriggert wird
- Trade-off: whisper.cpp CGo-Bindings sind fragil → daher API-basierter Ansatz fürs Erste

### STT: OpenAI Whisper API (statt lokal)
- whisper.cpp Go-Bindings sind "nearly unmaintained", CGo-Build komplex, Performance-Caveats (45x Overhead bei Callbacks)
- CPU-basiertes lokales Whisper (Ryzen 9 7950X) wäre schnell genug, aber API ist simpler für MVP
- Kosten minimal: $0.006/Minute
- **Spätere Option:** whisper.cpp als HTTP-Server (kein CGo nötig), vox spricht dann mit localhost statt OpenAI

### LLM-Cleanup: OpenAI GPT-4o-mini (statt Anthropic)
- Kostengründe — GPT-4o-mini ist extrem günstig für diesen Use-Case
- Cleanup macht den Qualitätsunterschied: Interpunktion, Fachbegriffe, Füllwörter raus
- System-Prompt auf DE/EN optimiert, erkennt Sprache automatisch

### Audio: pw-record (PipeWire Subprocess)
- Fedora nutzt PipeWire nativ, pw-record ist vorinstalliert
- Nimmt 16kHz Mono WAV auf (Whisper-optimales Format)
- Wird per SIGINT sauber gestoppt (finalisiert WAV-Header)

### Text-Injection: Pluggable (stdout/clipboard/wtype/ydotool)
- Wayland macht globale Text-Injection schwieriger als X11
- wtype: Wayland-nativ, simuliert Tastatureingaben
- ydotool: funktioniert auf Wayland+X11 via /dev/uinput
- clipboard (wl-copy): universeller Fallback
- stdout: zum Testen und für Pipe-Workflows

### GPU: Nicht genutzt (bewusste Entscheidung)
- AMD RX 6950 XT vorhanden, aber ROCm auf Fedora ist fragil
- CTranslate2 (faster-whisper) ist CUDA-optimiert, nicht ROCm
- Speedup bei kurzen Diktier-Chunks (5-30s) minimal vs. CPU
- Wird nur relevant wenn lokaler Whisper-Server kommt

## System-Specs (Zielmaschine)
- CPU: AMD Ryzen 9 7950X (16-Core, 32 Threads)
- RAM: 64 GB
- GPU: AMD RX 6950 XT (kein ROCm)
- OS: Fedora (Wayland, PipeWire)
- Go: 1.25

## Status: MVP fertig, untested

Grundgerüst steht. Alle Komponenten implementiert, kompiliert, aber noch nicht
end-to-end getestet (kein Mikrofon vorhanden).

## TODOs

### Prio 1 — Grundfunktion testen
- [ ] Mikrofon anschließen und end-to-end testen
- [ ] wl-clipboard installieren (`sudo dnf install wl-clipboard`)
- [ ] wtype installieren (`sudo dnf install wtype`)
- [ ] OPENAI_API_KEY setzen und API-Calls verifizieren

### Prio 2 — Globaler Hotkey (Push-to-Talk)
- [ ] Hotkey-Mechanismus evaluieren:
  - evdev (direkt /dev/input lesen, braucht input-Gruppe)
  - D-Bus GlobalShortcuts Portal (Wayland-nativ, XDG Portal)
  - Dedizierte Taste (z.B. Maus-Seitentaste, Fußpedal)
- [ ] Push-to-talk implementieren: Taste halten → aufnehmen, loslassen → transkribieren
- [ ] Daemon-Modus: vox läuft im Hintergrund, reagiert auf Hotkey

### Prio 3 — Lokales Whisper (Latenz + Offline)
- [ ] whisper.cpp HTTP-Server als Backend-Option
- [ ] vox managed den Server-Prozess (auto-start/stop)
- [ ] Modell-Download und -Management (tiny/base/small/medium)
- [ ] Backend per Config wählbar (openai vs. local)

### Prio 4 — UX Polish
- [ ] System-Tray-Icon mit Status (idle/recording/processing)
- [ ] Desktop-Notification bei fertigem Text
- [ ] Audio-Feedback (kurzer Ton bei Start/Stop)
- [ ] Konfigurations-Datei (~/.config/vox/config.yaml)
- [ ] History/Log der letzten Transkriptionen

### Prio 5 — Erweitert
- [ ] Streaming-Transkription (Echtzeit-Anzeige während des Sprechens)
- [ ] Kontext-Prompt: aktuelles Fenster/App als Kontext für LLM-Cleanup
- [ ] Mehrsprachig: automatische Spracherkennung statt -lang Flag
- [ ] Custom-Vocabulary/Glossar für Fachbegriffe
