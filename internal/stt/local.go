package stt

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"
)

const defaultLocalURL = "http://localhost:8080"

// Local sends audio to a local Whisper-compatible HTTP server.
type Local struct {
	url string
}

// NewLocal creates a transcriber that sends audio to a local Whisper server.
// If url is empty, defaults to http://localhost:8080.
func NewLocal(url string) *Local {
	if url == "" {
		url = defaultLocalURL
	}
	return &Local{url: url}
}

func (l *Local) Transcribe(audioFile, language, prompt string) (string, error) {
	f, err := os.Open(audioFile)
	if err != nil {
		return "", fmt.Errorf("Audiodatei öffnen: %w", err)
	}
	defer f.Close()

	var buf bytes.Buffer
	w := multipart.NewWriter(&buf)

	part, err := w.CreateFormFile("file", filepath.Base(audioFile))
	if err != nil {
		return "", err
	}
	if _, err := io.Copy(part, f); err != nil {
		return "", err
	}

	w.WriteField("model", "whisper-1")

	if language != "" {
		w.WriteField("language", language)
	}

	if prompt != "" {
		w.WriteField("prompt", prompt)
	}

	w.Close()

	endpoint := l.url + "/v1/audio/transcriptions"
	req, err := http.NewRequest("POST", endpoint, &buf)
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", w.FormDataContentType())

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("Local Whisper Anfrage: %w", err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("Local Whisper Fehler (%d): %s", resp.StatusCode, string(body))
	}

	var result whisperResponse
	if err := json.Unmarshal(body, &result); err != nil {
		return "", fmt.Errorf("Antwort parsen: %w", err)
	}

	return result.Text, nil
}
