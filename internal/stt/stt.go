package stt

// Segment is one timestamped span of the transcript, in seconds from the
// start of the audio. The JSON tags match the OpenAI verbose_json segment
// fields, so the response can be decoded into it directly.
type Segment struct {
	Start float64 `json:"start"`
	End   float64 `json:"end"`
	Text  string  `json:"text"`
}

// Result is a transcription plus optional timing detail.
type Result struct {
	Text string
	// Segments carry per-span timestamps when the backend delivered them
	// (whisper-1 with verbose_json; the gpt-4o transcribe models and plain
	// servers return text only). Callers must treat them as optional —
	// their absence is normal, never an error.
	Segments []Segment
}

// Transcriber converts audio to text.
type Transcriber interface {
	Transcribe(audioFile, language, prompt string) (Result, error)
}

// NewTranscriber creates a Transcriber based on the backend name.
// Supported backends: "openai" (default), "local".
// model is forwarded to the OpenAI backend only (local servers ignore it);
// an empty string uses the historical "whisper-1" default.
func NewTranscriber(backend, apiKey, url, model string) Transcriber {
	switch backend {
	case "local":
		return NewLocal(url)
	default:
		return NewOpenAI(apiKey, model)
	}
}
