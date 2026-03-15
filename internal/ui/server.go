package ui

import (
	"embed"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/smnhffmnn/vox/internal/config"
	"github.com/smnhffmnn/vox/internal/history"
	"github.com/smnhffmnn/vox/internal/keychain"
)

//go:embed static/index.html
var staticFiles embed.FS

// Server serves the web UI and REST API.
type Server struct {
	cfg     *config.Config
	history *history.History
	port    int
	started time.Time

	state   string
	stateMu sync.RWMutex
}

// NewServer creates a UI server.
func NewServer(cfg *config.Config, hist *history.History, port int) *Server {
	return &Server{
		cfg:     cfg,
		history: hist,
		port:    port,
		started: time.Now(),
		state:   "idle",
	}
}

// SetState updates the current state (thread-safe).
func (s *Server) SetState(state string) {
	s.stateMu.Lock()
	s.state = state
	s.stateMu.Unlock()
}

// GetState returns the current state.
func (s *Server) GetState() string {
	s.stateMu.RLock()
	defer s.stateMu.RUnlock()
	return s.state
}

// Start begins serving the HTTP API on localhost.
func (s *Server) Start() error {
	mux := http.NewServeMux()

	// Static files
	mux.HandleFunc("/", s.handleIndex)

	// REST API
	mux.HandleFunc("/api/config", s.handleConfig)
	mux.HandleFunc("/api/dictionary", s.handleDictionary)
	mux.HandleFunc("/api/snippets", s.handleSnippets)
	mux.HandleFunc("/api/history", s.handleHistory)
	mux.HandleFunc("/api/status", s.handleStatus)
	mux.HandleFunc("/api/test/stt", s.handleTestSTT)
	mux.HandleFunc("/api/test/llm", s.handleTestLLM)
	mux.HandleFunc("/api/key", s.handleKey)

	addr := fmt.Sprintf("127.0.0.1:%d", s.port)
	fmt.Fprintf(os.Stderr, "vox: UI server at http://%s\n", addr)

	go http.ListenAndServe(addr, mux)
	return nil
}

func (s *Server) handleIndex(w http.ResponseWriter, r *http.Request) {
	data, err := staticFiles.ReadFile("static/index.html")
	if err != nil {
		http.Error(w, "UI not found", 500)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Write(data)
}

func (s *Server) handleConfig(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		writeJSON(w, map[string]any{
			"language":       s.cfg.Language,
			"output":         s.cfg.Output,
			"raw":            s.cfg.Raw,
			"hotkey":         s.cfg.Hotkey,
			"mode":           s.cfg.Mode,
			"notifications":  s.cfg.Notifications,
			"audio_feedback": s.cfg.AudioFeedback,
			"stt_backend":    s.cfg.STTBackend,
			"stt_url":        s.cfg.STTURL,
			"llm_backend":    s.cfg.LLMBackend,
			"llm_url":        s.cfg.LLMURL,
			"llm_model":      s.cfg.LLMModel,
			"ui_port":        s.cfg.UIPort,
		})
	case http.MethodPut:
		var body map[string]any
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			writeError(w, "Invalid JSON", 400)
			return
		}
		applyConfigUpdate(s.cfg, body)
		if err := s.cfg.Save(); err != nil {
			writeError(w, err.Error(), 500)
			return
		}
		writeJSON(w, map[string]string{"status": "saved"})
	default:
		w.WriteHeader(405)
	}
}

func applyConfigUpdate(cfg *config.Config, body map[string]any) {
	if v, ok := body["language"].(string); ok {
		cfg.Language = v
	}
	if v, ok := body["output"].(string); ok {
		cfg.Output = v
	}
	if v, ok := body["raw"].(bool); ok {
		cfg.Raw = v
	}
	if v, ok := body["hotkey"].(string); ok {
		cfg.Hotkey = v
	}
	if v, ok := body["mode"].(string); ok {
		cfg.Mode = v
	}
	if v, ok := body["notifications"].(bool); ok {
		cfg.Notifications = v
	}
	if v, ok := body["audio_feedback"].(bool); ok {
		cfg.AudioFeedback = v
	}
	if v, ok := body["stt_backend"].(string); ok {
		cfg.STTBackend = v
	}
	if v, ok := body["stt_url"].(string); ok {
		cfg.STTURL = v
	}
	if v, ok := body["llm_backend"].(string); ok {
		cfg.LLMBackend = v
	}
	if v, ok := body["llm_url"].(string); ok {
		cfg.LLMURL = v
	}
	if v, ok := body["llm_model"].(string); ok {
		cfg.LLMModel = v
	}
	if v, ok := body["ui_port"].(float64); ok {
		cfg.UIPort = int(v)
	}
}

func (s *Server) handleDictionary(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		words, _ := config.LoadDictionary()
		if words == nil {
			words = []string{}
		}
		writeJSON(w, words)
	case http.MethodPut:
		data, err := io.ReadAll(r.Body)
		if err != nil {
			writeError(w, "Read error", 400)
			return
		}
		var words []string
		if err := json.Unmarshal(data, &words); err != nil {
			writeError(w, "Invalid JSON", 400)
			return
		}
		if err := saveDictionary(words); err != nil {
			writeError(w, err.Error(), 500)
			return
		}
		writeJSON(w, map[string]string{"status": "saved"})
	default:
		w.WriteHeader(405)
	}
}

func saveDictionary(words []string) error {
	dir, err := config.ConfigDir()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	content := strings.Join(words, "\n") + "\n"
	return os.WriteFile(filepath.Join(dir, "dictionary.txt"), []byte(content), 0o644)
}

func (s *Server) handleSnippets(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		snippets, _ := config.LoadSnippets()
		if snippets == nil {
			snippets = []config.Snippet{}
		}
		writeJSON(w, snippets)
	case http.MethodPut:
		data, err := io.ReadAll(r.Body)
		if err != nil {
			writeError(w, "Read error", 400)
			return
		}
		var snippets []config.Snippet
		if err := json.Unmarshal(data, &snippets); err != nil {
			writeError(w, "Invalid JSON", 400)
			return
		}
		if err := saveSnippets(snippets); err != nil {
			writeError(w, err.Error(), 500)
			return
		}
		writeJSON(w, map[string]string{"status": "saved"})
	default:
		w.WriteHeader(405)
	}
}

func saveSnippets(snippets []config.Snippet) error {
	dir, err := config.ConfigDir()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	var b strings.Builder
	for _, s := range snippets {
		trigger := strings.ReplaceAll(s.Trigger, `"`, `\"`)
		text := strings.ReplaceAll(s.Text, "\n", `\n`)
		text = strings.ReplaceAll(text, `"`, `\"`)
		b.WriteString(fmt.Sprintf("- trigger: \"%s\"\n  text: \"%s\"\n", trigger, text))
	}
	return os.WriteFile(filepath.Join(dir, "snippets.yaml"), []byte(b.String()), 0o644)
}

func (s *Server) handleHistory(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.WriteHeader(405)
		return
	}
	entries := s.history.Entries()
	if entries == nil {
		entries = []history.Entry{}
	}
	writeJSON(w, entries)
}

func (s *Server) handleStatus(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.WriteHeader(405)
		return
	}
	uptime := time.Since(s.started).Truncate(time.Second).String()
	writeJSON(w, map[string]any{
		"state":  s.GetState(),
		"uptime": uptime,
		"port":   s.port,
	})
}

func (s *Server) handleTestSTT(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.WriteHeader(405)
		return
	}
	url := "https://api.openai.com/v1/models"
	if s.cfg.STTBackend == "local" {
		u := s.cfg.STTURL
		if u == "" {
			u = "http://localhost:8080"
		}
		url = u + "/v1/models"
	}

	client := &http.Client{Timeout: 5 * time.Second}
	req, _ := http.NewRequest("GET", url, nil)
	if s.cfg.STTBackend != "local" {
		if key := resolveKey(); key != "" {
			req.Header.Set("Authorization", "Bearer "+key)
		}
	}
	resp, err := client.Do(req)
	if err != nil {
		writeJSON(w, map[string]any{"ok": false, "error": err.Error()})
		return
	}
	resp.Body.Close()
	writeJSON(w, map[string]any{"ok": resp.StatusCode == 200, "status": resp.StatusCode})
}

func (s *Server) handleTestLLM(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.WriteHeader(405)
		return
	}

	baseURL := "https://api.openai.com/v1"
	switch s.cfg.LLMBackend {
	case "ollama":
		baseURL = s.cfg.LLMURL
		if baseURL == "" {
			baseURL = "http://localhost:11434/v1"
		}
	case "none":
		writeJSON(w, map[string]any{"ok": true, "message": "LLM disabled"})
		return
	}

	url := baseURL + "/models"
	client := &http.Client{Timeout: 5 * time.Second}
	req, _ := http.NewRequest("GET", url, nil)
	if s.cfg.LLMBackend != "ollama" {
		if key := resolveKey(); key != "" {
			req.Header.Set("Authorization", "Bearer "+key)
		}
	}
	resp, err := client.Do(req)
	if err != nil {
		writeJSON(w, map[string]any{"ok": false, "error": err.Error()})
		return
	}
	resp.Body.Close()
	writeJSON(w, map[string]any{"ok": resp.StatusCode == 200, "status": resp.StatusCode})
}

func (s *Server) handleKey(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodPost:
		var body struct {
			Key   string `json:"key"`
			Value string `json:"value"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			writeError(w, "Invalid JSON", 400)
			return
		}
		if body.Key == "" || body.Value == "" {
			writeError(w, "key and value required", 400)
			return
		}
		if err := keychain.Set("vox", body.Key, body.Value); err != nil {
			writeError(w, err.Error(), 500)
			return
		}
		writeJSON(w, map[string]string{"status": "saved"})
	case http.MethodDelete:
		var body struct {
			Key string `json:"key"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			writeError(w, "Invalid JSON", 400)
			return
		}
		if err := keychain.Delete("vox", body.Key); err != nil {
			writeError(w, err.Error(), 500)
			return
		}
		writeJSON(w, map[string]string{"status": "deleted"})
	case http.MethodGet:
		key := r.URL.Query().Get("key")
		if key == "" {
			key = "openai-api-key"
		}
		writeJSON(w, map[string]bool{"exists": keychain.HasKey("vox", key)})
	default:
		w.WriteHeader(405)
	}
}

func resolveKey() string {
	if key := os.Getenv("OPENAI_API_KEY"); key != "" {
		return key
	}
	key, _ := keychain.Get("vox", "openai-api-key")
	return key
}

func writeJSON(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(v)
}

func writeError(w http.ResponseWriter, msg string, code int) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	json.NewEncoder(w).Encode(map[string]string{"error": msg})
}
