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
)

type OpenAI struct {
	apiKey string
	model  string
}

func NewOpenAI(apiKey string) *OpenAI {
	return &OpenAI{
		apiKey: apiKey,
		model:  "whisper-1",
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

	if language != "" {
		w.WriteField("language", language)
	}

	if prompt != "" {
		w.WriteField("prompt", prompt)
	}

	w.Close()

	req, err := http.NewRequest("POST", "https://api.openai.com/v1/audio/transcriptions", &buf)
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
		return "", fmt.Errorf("OpenAI API error (%d): %s", resp.StatusCode, string(body))
	}

	var result whisperResponse
	if err := json.Unmarshal(body, &result); err != nil {
		return "", fmt.Errorf("response parse: %w", err)
	}

	return result.Text, nil
}
