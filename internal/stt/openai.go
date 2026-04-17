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
	"time"

	"github.com/smnhffmnn/vox/internal/apierr"
)

const defaultOpenAIBaseURL = "https://api.openai.com"

type OpenAI struct {
	apiKey  string
	model   string
	baseURL string
}

// NewOpenAI creates a transcriber for OpenAI's /v1/audio/transcriptions
// endpoint. An empty model defaults to "whisper-1" to preserve the historical
// behaviour; callers can override to "gpt-4o-transcribe" or
// "gpt-4o-mini-transcribe" for fewer hallucinations on silent/noisy audio.
func NewOpenAI(apiKey, model string) *OpenAI {
	if model == "" {
		model = "whisper-1"
	}
	return &OpenAI{
		apiKey:  apiKey,
		model:   model,
		baseURL: defaultOpenAIBaseURL,
	}
}

type whisperResponse struct {
	Text string `json:"text"`
}

func (o *OpenAI) Transcribe(audioFile, language, prompt string) (string, error) {
	f, err := os.Open(audioFile)
	if err != nil {
		return "", fmt.Errorf("open audio file: %w", err)
	}
	defer f.Close()

	var buf bytes.Buffer
	w := multipart.NewWriter(&buf)

	// Audio file
	part, err := w.CreateFormFile("file", filepath.Base(audioFile))
	if err != nil {
		return "", err
	}
	if _, err := io.Copy(part, f); err != nil {
		return "", err
	}

	w.WriteField("model", o.model)

	// Issue 9: hard-set temperature=0 to minimise Whisper hallucinations on
	// silent/leise audio. Supported by whisper-1 and the gpt-4o-* transcribe
	// models; omitting it lets the server fall back to a non-zero default.
	w.WriteField("temperature", "0")

	if language != "" {
		w.WriteField("language", language)
	}

	if prompt != "" {
		w.WriteField("prompt", prompt)
	}

	w.Close()

	req, err := http.NewRequest("POST", o.baseURL+"/v1/audio/transcriptions", &buf)
	if err != nil {
		return "", err
	}
	req.Header.Set("Authorization", "Bearer "+o.apiKey)
	req.Header.Set("Content-Type", w.FormDataContentType())

	client := &http.Client{Timeout: 10 * time.Minute}
	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("OpenAI API request: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(io.LimitReader(resp.Body, 10<<20)) // 10 MB limit
	if err != nil {
		return "", fmt.Errorf("read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		if apierr.IsInsufficientCredits(resp.StatusCode, body) {
			return "", fmt.Errorf("OpenAI API error (%d): %w: %s", resp.StatusCode, apierr.ErrInsufficientCredits, string(body))
		}
		return "", fmt.Errorf("OpenAI API error (%d): %s", resp.StatusCode, string(body))
	}

	var result whisperResponse
	if err := json.Unmarshal(body, &result); err != nil {
		return "", fmt.Errorf("response parse: %w", err)
	}

	return result.Text, nil
}
