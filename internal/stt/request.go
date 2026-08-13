package stt

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"os"
	"path/filepath"
)

// buildRequestBody assembles the multipart form for a transcription request:
// the audio file plus the given fields, in order. Fields with an empty value
// are omitted. It returns the body and its Content-Type.
//
// Shared by both backends — and rebuilt for every attempt, because a bytes
// buffer is consumed by sending it.
func buildRequestBody(audioFile string, fields [][2]string) (*bytes.Buffer, string, error) {
	f, err := os.Open(audioFile)
	if err != nil {
		return nil, "", fmt.Errorf("open audio file: %w", err)
	}
	defer f.Close()

	var buf bytes.Buffer
	w := multipart.NewWriter(&buf)

	part, err := w.CreateFormFile("file", filepath.Base(audioFile))
	if err != nil {
		return nil, "", err
	}
	if _, err := io.Copy(part, f); err != nil {
		return nil, "", err
	}

	for _, kv := range fields {
		if kv[1] != "" {
			w.WriteField(kv[0], kv[1])
		}
	}

	w.Close()
	return &buf, w.FormDataContentType(), nil
}

type whisperResponse struct {
	Text string `json:"text"`
	// Segments are present only in verbose_json responses; a plain json
	// response simply leaves them empty.
	Segments []Segment `json:"segments"`
}

// parseTranscription decodes a transcription response body. Segments are
// optional — a body without them is a complete result, not an error.
func parseTranscription(body []byte) (Result, error) {
	var r whisperResponse
	if err := json.Unmarshal(body, &r); err != nil {
		return Result{}, fmt.Errorf("response parse: %w", err)
	}
	return Result{Text: r.Text, Segments: r.Segments}, nil
}
