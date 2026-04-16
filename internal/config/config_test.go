package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// setHome redirects the user home directory to a temporary directory for
// the duration of the test. Sets both HOME (Unix) and USERPROFILE (Windows)
// so tests stay hermetic on all platforms supported by os.UserHomeDir.
func setHome(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	t.Setenv("HOME", dir)
	t.Setenv("USERPROFILE", dir)
	return dir
}

func TestDefaultConfig(t *testing.T) {
	cfg := DefaultConfig()

	tests := []struct {
		name string
		got  any
		want any
	}{
		{"Language", cfg.Language, "de"},
		{"Output", cfg.Output, "wtype"},
		{"Raw", cfg.Raw, false},
		{"Hotkey", cfg.Hotkey, "right_option"},
		{"Mode", cfg.Mode, "hold"},
		{"HandsfreeTimeout", cfg.HandsfreeTimeout, 360},
		{"DoubletapWindow", cfg.DoubletapWindow, 400},
		{"Notifications", cfg.Notifications, true},
		{"AudioFeedback", cfg.AudioFeedback, true},
		{"ShowOverlay", cfg.ShowOverlay, true},
		{"STTBackend", cfg.STTBackend, "openai"},
		{"LLMBackend", cfg.LLMBackend, "openai"},
		{"STTURL empty", cfg.STTURL, ""},
		{"LLMURL empty", cfg.LLMURL, ""},
		{"LLMModel empty", cfg.LLMModel, ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.got != tt.want {
				t.Errorf("got %v, want %v", tt.got, tt.want)
			}
		})
	}
}

func TestExtractYAMLValue(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want string
	}{
		{"plain", "foo", "foo"},
		{"leading trailing whitespace trimmed", "  foo  ", "foo"},
		{"double quoted", `"foo"`, "foo"},
		{"double quoted with spaces inside kept", `"foo bar"`, "foo bar"},
		{"quoted with leading whitespace", `  "foo"`, "foo"},
		{"empty", "", ""},
		{"whitespace only", "   ", ""},
		{"empty quotes", `""`, ""},
		{"single quote not stripped", `'foo'`, "'foo'"},
		{"unterminated opening quote", `"foo`, `"foo`},
		{"unterminated closing quote", `foo"`, `foo"`},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := extractYAMLValue(tt.in); got != tt.want {
				t.Errorf("extractYAMLValue(%q) = %q, want %q", tt.in, got, tt.want)
			}
		})
	}
}

func TestParseKV(t *testing.T) {
	tests := []struct {
		name      string
		in        string
		wantKey   string
		wantValue string
		wantOK    bool
	}{
		{"plain", "language: de", "language", "de", true},
		{"quoted value", `hotkey: "right_option"`, "hotkey", "right_option", true},
		{"extra whitespace", "   key   :   value   ", "key", "value", true},
		{"empty value", "key:", "key", "", true},
		{"no colon", "no separator here", "", "", false},
		{"value contains colon", "url: http://example.com", "url", "http://example.com", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			k, v, ok := parseKV(tt.in)
			if ok != tt.wantOK {
				t.Fatalf("ok = %v, want %v", ok, tt.wantOK)
			}
			if k != tt.wantKey {
				t.Errorf("key = %q, want %q", k, tt.wantKey)
			}
			if v != tt.wantValue {
				t.Errorf("value = %q, want %q", v, tt.wantValue)
			}
		})
	}
}

func TestParseConfig_AllKeys(t *testing.T) {
	data := `language: en
output: xdotool
raw: true
hotkey: "right_option"
mode: toggle
handsfree_timeout: 120
doubletap_window: 250
notifications: false
audio_feedback: false
show_overlay: false
stt_backend: local
stt_url: http://localhost:9000
llm_backend: ollama
llm_url: http://localhost:11434
llm_model: qwen2.5:7b
`
	cfg := DefaultConfig()
	parseConfig(data, cfg)

	if cfg.Language != "en" {
		t.Errorf("Language = %q", cfg.Language)
	}
	if cfg.Output != "xdotool" {
		t.Errorf("Output = %q", cfg.Output)
	}
	if !cfg.Raw {
		t.Errorf("Raw = %v, want true", cfg.Raw)
	}
	if cfg.Hotkey != "right_option" {
		t.Errorf("Hotkey = %q", cfg.Hotkey)
	}
	if cfg.Mode != "toggle" {
		t.Errorf("Mode = %q", cfg.Mode)
	}
	if cfg.HandsfreeTimeout != 120 {
		t.Errorf("HandsfreeTimeout = %d", cfg.HandsfreeTimeout)
	}
	if cfg.DoubletapWindow != 250 {
		t.Errorf("DoubletapWindow = %d", cfg.DoubletapWindow)
	}
	if cfg.Notifications {
		t.Errorf("Notifications = %v, want false", cfg.Notifications)
	}
	if cfg.AudioFeedback {
		t.Errorf("AudioFeedback = %v, want false", cfg.AudioFeedback)
	}
	if cfg.ShowOverlay {
		t.Errorf("ShowOverlay = %v, want false", cfg.ShowOverlay)
	}
	if cfg.STTBackend != "local" {
		t.Errorf("STTBackend = %q", cfg.STTBackend)
	}
	if cfg.STTURL != "http://localhost:9000" {
		t.Errorf("STTURL = %q", cfg.STTURL)
	}
	if cfg.LLMBackend != "ollama" {
		t.Errorf("LLMBackend = %q", cfg.LLMBackend)
	}
	if cfg.LLMURL != "http://localhost:11434" {
		t.Errorf("LLMURL = %q", cfg.LLMURL)
	}
	if cfg.LLMModel != "qwen2.5:7b" {
		t.Errorf("LLMModel = %q", cfg.LLMModel)
	}
}

func TestParseConfig_IgnoresCommentsAndBlankLines(t *testing.T) {
	data := `
# comment line
   # indented comment
language: en

   # another comment
output: wtype
`
	cfg := DefaultConfig()
	parseConfig(data, cfg)

	if cfg.Language != "en" {
		t.Errorf("Language = %q, want en", cfg.Language)
	}
	if cfg.Output != "wtype" {
		t.Errorf("Output = %q, want wtype", cfg.Output)
	}
}

func TestParseConfig_PreservesDefaultsForMissingKeys(t *testing.T) {
	data := `language: fr
`
	cfg := DefaultConfig()
	parseConfig(data, cfg)

	if cfg.Language != "fr" {
		t.Errorf("Language = %q", cfg.Language)
	}
	// Defaults should remain for unspecified keys.
	if cfg.Hotkey != "right_option" {
		t.Errorf("Hotkey should remain default, got %q", cfg.Hotkey)
	}
	if cfg.HandsfreeTimeout != 360 {
		t.Errorf("HandsfreeTimeout should remain default, got %d", cfg.HandsfreeTimeout)
	}
}

func TestParseConfig_InvalidIntegersIgnored(t *testing.T) {
	data := `handsfree_timeout: not_a_number
doubletap_window: xyz
`
	cfg := DefaultConfig()
	parseConfig(data, cfg)

	// Values should remain at defaults because parse fails silently.
	if cfg.HandsfreeTimeout != 360 {
		t.Errorf("HandsfreeTimeout = %d, want default 360", cfg.HandsfreeTimeout)
	}
	if cfg.DoubletapWindow != 400 {
		t.Errorf("DoubletapWindow = %d, want default 400", cfg.DoubletapWindow)
	}
}

func TestParseConfig_BoolTruthy(t *testing.T) {
	// Only exact "true" turns bool fields on; everything else is false.
	data := `raw: TRUE
notifications: 1
audio_feedback: yes
show_overlay: true
`
	cfg := DefaultConfig()
	parseConfig(data, cfg)

	if cfg.Raw {
		t.Errorf(`Raw: "TRUE" should not count as true`)
	}
	if cfg.Notifications {
		t.Errorf(`Notifications: "1" should not count as true`)
	}
	if cfg.AudioFeedback {
		t.Errorf(`AudioFeedback: "yes" should not count as true`)
	}
	if !cfg.ShowOverlay {
		t.Errorf(`ShowOverlay: "true" should be true`)
	}
}

func TestParseConfig_UnknownKeysIgnored(t *testing.T) {
	data := `language: en
unknown_key: some_value
another_unknown: 42
`
	cfg := DefaultConfig()
	parseConfig(data, cfg)

	if cfg.Language != "en" {
		t.Errorf("Language = %q", cfg.Language)
	}
	// No panic, no effect from unknown keys.
}

func TestLoad_MissingFileReturnsDefaults(t *testing.T) {
	setHome(t)

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() err = %v, want nil", err)
	}
	if cfg == nil {
		t.Fatal("Load() cfg = nil")
	}

	def := DefaultConfig()
	if cfg.Language != def.Language {
		t.Errorf("Language = %q, want default %q", cfg.Language, def.Language)
	}
	if cfg.HandsfreeTimeout != def.HandsfreeTimeout {
		t.Errorf("HandsfreeTimeout = %d, want default %d", cfg.HandsfreeTimeout, def.HandsfreeTimeout)
	}
}

func TestLoad_ReadsExistingFile(t *testing.T) {
	home := setHome(t)

	dir := filepath.Join(home, ".config", "vox")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	content := "language: fr\nhotkey: left_option\nhandsfree_timeout: 42\n"
	if err := os.WriteFile(filepath.Join(dir, "config.yaml"), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() err = %v", err)
	}
	if cfg.Language != "fr" {
		t.Errorf("Language = %q, want fr", cfg.Language)
	}
	if cfg.Hotkey != "left_option" {
		t.Errorf("Hotkey = %q, want left_option", cfg.Hotkey)
	}
	if cfg.HandsfreeTimeout != 42 {
		t.Errorf("HandsfreeTimeout = %d, want 42", cfg.HandsfreeTimeout)
	}
}

func TestSaveLoad_Roundtrip(t *testing.T) {
	setHome(t)

	original := DefaultConfig()
	original.Language = "es"
	original.Output = "xdotool"
	original.Raw = true
	original.Hotkey = "caps_lock"
	original.Mode = "toggle"
	original.HandsfreeTimeout = 90
	original.DoubletapWindow = 333
	original.Notifications = false
	original.AudioFeedback = false
	original.ShowOverlay = false
	original.STTBackend = "local"
	original.STTURL = "http://stt.local"
	original.LLMBackend = "ollama"
	original.LLMURL = "http://llm.local"
	original.LLMModel = "llama3:8b"

	if err := original.Save(); err != nil {
		t.Fatalf("Save() err = %v", err)
	}

	loaded, err := Load()
	if err != nil {
		t.Fatalf("Load() err = %v", err)
	}

	// Compare field by field; mu cannot be compared with ==.
	checks := []struct {
		name       string
		got, want  any
	}{
		{"Language", loaded.Language, original.Language},
		{"Output", loaded.Output, original.Output},
		{"Raw", loaded.Raw, original.Raw},
		{"Hotkey", loaded.Hotkey, original.Hotkey},
		{"Mode", loaded.Mode, original.Mode},
		{"HandsfreeTimeout", loaded.HandsfreeTimeout, original.HandsfreeTimeout},
		{"DoubletapWindow", loaded.DoubletapWindow, original.DoubletapWindow},
		{"Notifications", loaded.Notifications, original.Notifications},
		{"AudioFeedback", loaded.AudioFeedback, original.AudioFeedback},
		{"ShowOverlay", loaded.ShowOverlay, original.ShowOverlay},
		{"STTBackend", loaded.STTBackend, original.STTBackend},
		{"STTURL", loaded.STTURL, original.STTURL},
		{"LLMBackend", loaded.LLMBackend, original.LLMBackend},
		{"LLMURL", loaded.LLMURL, original.LLMURL},
		{"LLMModel", loaded.LLMModel, original.LLMModel},
	}
	for _, c := range checks {
		if c.got != c.want {
			t.Errorf("%s: got %v, want %v", c.name, c.got, c.want)
		}
	}
}

func TestSave_CreatesConfigDir(t *testing.T) {
	home := setHome(t)

	cfg := DefaultConfig()
	if err := cfg.Save(); err != nil {
		t.Fatalf("Save() err = %v", err)
	}

	path := filepath.Join(home, ".config", "vox", "config.yaml")
	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("expected config.yaml to exist: %v", err)
	}
	if info.IsDir() {
		t.Fatal("config.yaml should be a file, not a dir")
	}
}

func TestSave_OmitsEmptyOptionalURLs(t *testing.T) {
	home := setHome(t)

	cfg := DefaultConfig()
	// Leave STTURL, LLMURL, LLMModel empty.
	if err := cfg.Save(); err != nil {
		t.Fatalf("Save() err = %v", err)
	}

	data, err := os.ReadFile(filepath.Join(home, ".config", "vox", "config.yaml"))
	if err != nil {
		t.Fatal(err)
	}

	content := string(data)
	if strings.Contains(content, "stt_url:") {
		t.Error("empty STTURL should not appear in saved config")
	}
	if strings.Contains(content, "llm_url:") {
		t.Error("empty LLMURL should not appear in saved config")
	}
	if strings.Contains(content, "llm_model:") {
		t.Error("empty LLMModel should not appear in saved config")
	}
	// stt_backend and llm_backend are always written.
	if !strings.Contains(content, "stt_backend:") {
		t.Error("stt_backend should always be present")
	}
	if !strings.Contains(content, "llm_backend:") {
		t.Error("llm_backend should always be present")
	}
}

func TestConfigDir_UsesUserHome(t *testing.T) {
	home := setHome(t)

	dir, err := ConfigDir()
	if err != nil {
		t.Fatalf("ConfigDir() err = %v", err)
	}
	want := filepath.Join(home, ".config", "vox")
	if dir != want {
		t.Errorf("ConfigDir() = %q, want %q", dir, want)
	}
}
