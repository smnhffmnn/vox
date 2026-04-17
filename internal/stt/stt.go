package stt

// Transcriber converts audio to text.
type Transcriber interface {
	Transcribe(audioFile, language, prompt string) (string, error)
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
