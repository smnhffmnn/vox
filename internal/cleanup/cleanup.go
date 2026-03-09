package cleanup

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
)

const systemPrompt = `Du bist ein Textbereiniger für Spracheingabe. Bereinige den transkribierten Text.

Regeln:
- Korrigiere Interpunktion und Groß-/Kleinschreibung
- Entferne Füllwörter (ähm, äh, hmm, halt, sozusagen) nur wenn sie keinen Sinn tragen
- Korrigiere offensichtliche Transkriptionsfehler
- Behalte den Originalton und die Bedeutung exakt bei
- Technische Fachbegriffe korrekt schreiben (z.B. "kubernetes" → "Kubernetes", "github" → "GitHub")
- Bei gemischter Sprache (DE/EN): Beibehalten wie gesprochen
- Kürze oder paraphrasiere NICHT — nur bereinigen
- Gib NUR den bereinigten Text zurück, keine Erklärungen oder Anführungszeichen`

type Cleaner struct {
	apiKey string
	model  string
}

func NewCleaner(apiKey string) *Cleaner {
	return &Cleaner{
		apiKey: apiKey,
		model:  "gpt-4o-mini",
	}
}

type chatRequest struct {
	Model    string        `json:"model"`
	Messages []chatMessage `json:"messages"`
}

type chatMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type chatResponse struct {
	Choices []struct {
		Message struct {
			Content string `json:"content"`
		} `json:"message"`
	} `json:"choices"`
}

func (c *Cleaner) Cleanup(text string, language string) (string, error) {
	prompt := systemPrompt
	if language != "de" {
		prompt = `You are a text cleaner for speech input. Clean up the transcribed text.

Rules:
- Fix punctuation and capitalization
- Remove filler words only when they carry no meaning
- Fix obvious transcription errors
- Keep the original tone and meaning exactly
- Write technical terms correctly (e.g. "kubernetes" → "Kubernetes", "github" → "GitHub")
- Do NOT shorten or paraphrase — only clean up
- Return ONLY the cleaned text, no explanations or quotes`
	}

	reqBody := chatRequest{
		Model: c.model,
		Messages: []chatMessage{
			{Role: "system", Content: prompt},
			{Role: "user", Content: text},
		},
	}

	jsonBody, err := json.Marshal(reqBody)
	if err != nil {
		return "", err
	}

	req, err := http.NewRequest("POST", "https://api.openai.com/v1/chat/completions", bytes.NewReader(jsonBody))
	if err != nil {
		return "", err
	}
	req.Header.Set("Authorization", "Bearer "+c.apiKey)
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("OpenAI API Anfrage: %w", err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("OpenAI API Fehler (%d): %s", resp.StatusCode, string(body))
	}

	var result chatResponse
	if err := json.Unmarshal(body, &result); err != nil {
		return "", fmt.Errorf("Antwort parsen: %w", err)
	}

	if len(result.Choices) == 0 {
		return "", fmt.Errorf("keine Antwort von OpenAI erhalten")
	}

	return result.Choices[0].Message.Content, nil
}
