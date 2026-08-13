package stt

import (
	"fmt"
	"io"
	"net/http"
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

// supportsVerboseJSON reports whether the model can return verbose_json with
// timestamped segments (VOX-13). Only whisper-1 does — the gpt-4o transcribe
// models reject the format, so they keep the default and return text only.
func supportsVerboseJSON(model string) bool { return model == "whisper-1" }

func (o *OpenAI) Transcribe(audioFile, language, prompt string) (Result, error) {
	fields := [][2]string{
		{"model", o.model},
		// Issue 9: hard-set temperature=0 to minimise Whisper hallucinations on
		// silent/leise audio. Supported by whisper-1 and the gpt-4o-* transcribe
		// models; omitting it lets the server fall back to a non-zero default.
		{"temperature", "0"},
		{"language", language},
		{"prompt", prompt},
	}
	if supportsVerboseJSON(o.model) {
		fields = append(fields, [2]string{"response_format", "verbose_json"})
	}

	buf, contentType, err := buildRequestBody(audioFile, fields)
	if err != nil {
		return Result{}, err
	}

	req, err := http.NewRequest("POST", o.baseURL+"/v1/audio/transcriptions", buf)
	if err != nil {
		return Result{}, err
	}
	req.Header.Set("Authorization", "Bearer "+o.apiKey)
	req.Header.Set("Content-Type", contentType)

	client := &http.Client{Timeout: 10 * time.Minute}
	resp, err := client.Do(req)
	if err != nil {
		return Result{}, fmt.Errorf("OpenAI API request: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(io.LimitReader(resp.Body, 10<<20)) // 10 MB limit
	if err != nil {
		return Result{}, fmt.Errorf("read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		if apierr.IsInsufficientCredits(resp.StatusCode, body) {
			return Result{}, fmt.Errorf("OpenAI API error (%d): %w: %s", resp.StatusCode, apierr.ErrInsufficientCredits, string(body))
		}
		return Result{}, fmt.Errorf("OpenAI API error (%d): %s", resp.StatusCode, string(body))
	}

	return parseTranscription(body)
}
