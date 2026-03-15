package stt

// Transcriber converts audio to text.
type Transcriber interface {
	Transcribe(audioFile, language, prompt string) (string, error)
}
