package stt

import (
	"fmt"
	"io"
	"net/http"
	"time"
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

func (l *Local) Transcribe(audioFile, language, prompt string) (Result, error) {
	// First ask for verbose_json to get timestamped segments (VOX-13). Not
	// every OpenAI-compatible server knows that format, so a non-OK answer is
	// retried once without it — segments are diagnostic detail and never worth
	// failing a dictation over. The retry costs one extra round trip against a
	// server that is local anyway.
	res, retryable, err := l.attempt(audioFile, language, prompt, true)
	if err != nil && retryable {
		res, _, err = l.attempt(audioFile, language, prompt, false)
	}
	return res, err
}

// attempt runs one transcription request. retryable is true only when the
// server answered with a non-OK status — the one case where dropping the
// verbose_json request can make a difference.
func (l *Local) attempt(audioFile, language, prompt string, verbose bool) (Result, bool, error) {
	fields := [][2]string{
		{"model", "whisper-1"},
		{"language", language},
		{"prompt", prompt},
	}
	if verbose {
		fields = append(fields, [2]string{"response_format", "verbose_json"})
	}

	buf, contentType, err := buildRequestBody(audioFile, fields)
	if err != nil {
		return Result{}, false, err
	}

	endpoint := l.url + "/v1/audio/transcriptions"
	req, err := http.NewRequest("POST", endpoint, buf)
	if err != nil {
		return Result{}, false, err
	}
	req.Header.Set("Content-Type", contentType)

	client := &http.Client{Timeout: 10 * time.Minute}
	resp, err := client.Do(req)
	if err != nil {
		return Result{}, false, fmt.Errorf("local Whisper request: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(io.LimitReader(resp.Body, 10<<20)) // 10 MB limit
	if err != nil {
		return Result{}, false, fmt.Errorf("read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return Result{}, true, fmt.Errorf("local Whisper error (%d): %s", resp.StatusCode, string(body))
	}

	res, err := parseTranscription(body)
	return res, false, err
}
