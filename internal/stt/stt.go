package stt

// Transcriber converts audio to text.
type Transcriber interface {
	Transcribe(audioFile, language, prompt string) (string, error)
}

// NewTranscriber creates a Transcriber based on the backend name.
// Supported backends: "openai" (default), "local".
func NewTranscriber(backend, apiKey, url string) Transcriber {
	switch backend {
	case "local":
		return NewLocal(url)
	default:
		return NewOpenAI(apiKey)
	}
}
