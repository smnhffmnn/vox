package stt

// Transcriber converts audio to text.
type Transcriber interface {
	Transcribe(audioFile string, language string) (string, error)
}
