package cleanup

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/smnhffmnn/vox/internal/windowctx"
)

type appCategory int

const (
	categoryDefault appCategory = iota
	categoryChat
	categoryEmail
	categoryIDE
	categoryDocs
	categoryBrowser
)

func detectCategory(ctx *windowctx.Context) appCategory {
	if ctx == nil {
		return categoryDefault
	}

	app := strings.ToLower(ctx.AppName)
	title := strings.ToLower(ctx.WindowTitle)

	// Chat apps
	for _, name := range []string{"slack", "teams", "discord", "telegram", "signal", "imessage", "messages"} {
		if strings.Contains(app, name) {
			return categoryChat
		}
	}

	// Email
	for _, name := range []string{"mail", "gmail", "outlook", "thunderbird"} {
		if strings.Contains(app, name) || strings.Contains(title, name) {
			return categoryEmail
		}
	}

	// IDE / Terminal
	for _, name := range []string{"code", "cursor", "intellij", "phpstorm", "webstorm", "xcode", "vim", "neovim", "terminal", "iterm", "alacritty", "kitty"} {
		if strings.Contains(app, name) {
			return categoryIDE
		}
	}

	// Docs
	for _, name := range []string{"pages", "docs", "word", "notes", "notion", "obsidian"} {
		if strings.Contains(app, name) {
			return categoryDocs
		}
	}

	// Browser
	for _, name := range []string{"firefox", "chrome", "safari", "arc", "brave"} {
		if strings.Contains(app, name) {
			return categoryBrowser
		}
	}

	return categoryDefault
}

func toneInstruction(cat appCategory, language string) string {
	if language == "de" {
		switch cat {
		case categoryChat:
			return "\n- Ton: casual, kurze Sätze, kein Punkt am Satzende"
		case categoryEmail:
			return "\n- Ton: formal, korrekte Interpunktion, vollständige Sätze"
		case categoryIDE:
			return "\n- Ton: technisch, Fachbegriffe bevorzugen, camelCase/snake_case beibehalten"
		case categoryDocs:
			return "\n- Ton: neutral, saubere Interpunktion"
		case categoryBrowser:
			return "\n- Ton: neutral"
		}
		return ""
	}

	switch cat {
	case categoryChat:
		return "\n- Tone: casual, short sentences, no period at end of sentences"
	case categoryEmail:
		return "\n- Tone: formal, correct punctuation, complete sentences"
	case categoryIDE:
		return "\n- Tone: technical, prefer technical terms, keep camelCase/snake_case as-is"
	case categoryDocs:
		return "\n- Tone: neutral, clean punctuation"
	case categoryBrowser:
		return "\n- Tone: neutral"
	}
	return ""
}

const basePromptDE = `Du bist ein Textbereiniger für Spracheingabe. Bereinige den transkribierten Text.

Regeln:
- Korrigiere Interpunktion und Groß-/Kleinschreibung
- Entferne Füllwörter (ähm, äh, hmm, halt, sozusagen) nur wenn sie keinen Sinn tragen
- Korrigiere offensichtliche Transkriptionsfehler
- Behalte den Originalton und die Bedeutung exakt bei
- Technische Fachbegriffe korrekt schreiben (z.B. "kubernetes" → "Kubernetes", "github" → "GitHub")
- Bei gemischter Sprache (DE/EN): Beibehalten wie gesprochen
- Kürze oder paraphrasiere NICHT — nur bereinigen
- Gib NUR den bereinigten Text zurück, keine Erklärungen oder Anführungszeichen`

const basePromptEN = `You are a text cleaner for speech input. Clean up the transcribed text.

Rules:
- Fix punctuation and capitalization
- Remove filler words only when they carry no meaning
- Fix obvious transcription errors
- Keep the original tone and meaning exactly
- Write technical terms correctly (e.g. "kubernetes" → "Kubernetes", "github" → "GitHub")
- Do NOT shorten or paraphrase — only clean up
- Return ONLY the cleaned text, no explanations or quotes`

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

// Cleanup cleans up transcribed text using an LLM.
// ctx can be nil — in that case, default tone is used.
func (c *Cleaner) Cleanup(text, language string, ctx *windowctx.Context, dictionary []string) (string, error) {
	var prompt string
	if language == "de" {
		prompt = basePromptDE
	} else {
		prompt = basePromptEN
	}

	cat := detectCategory(ctx)
	prompt += toneInstruction(cat, language)

	if len(dictionary) > 0 {
		if language == "de" {
			prompt += "\n- Bevorzugte Schreibweisen (verwende diese exakt so): " + strings.Join(dictionary, ", ")
		} else {
			prompt += "\n- Preferred spellings (use these exactly): " + strings.Join(dictionary, ", ")
		}
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
