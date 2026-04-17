package cleanup

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/smnhffmnn/vox/internal/apierr"
	"github.com/smnhffmnn/vox/internal/windowctx"
)

func TestDetectCategory_NilReturnsDefault(t *testing.T) {
	if got := detectCategory(nil); got != categoryDefault {
		t.Errorf("nil ctx: got %v, want categoryDefault", got)
	}
}

func TestDetectCategory_ByAppName(t *testing.T) {
	cases := []struct {
		appName string
		want    appCategory
	}{
		// Chat
		{"Slack", categoryChat},
		{"Microsoft Teams", categoryChat},
		{"Discord", categoryChat},
		{"Telegram Desktop", categoryChat},
		{"Signal", categoryChat},
		{"iMessage", categoryChat},
		{"Messages", categoryChat},

		// Email
		{"Mail", categoryEmail},
		{"Thunderbird", categoryEmail},
		{"Microsoft Outlook", categoryEmail},

		// IDE / Terminal
		{"Visual Studio Code", categoryIDE},
		{"Cursor", categoryIDE},
		{"IntelliJ IDEA", categoryIDE},
		{"PhpStorm", categoryIDE},
		{"WebStorm", categoryIDE},
		{"Xcode", categoryIDE},
		{"MacVim", categoryIDE},
		{"Neovim", categoryIDE},
		{"Terminal", categoryIDE},
		{"iTerm2", categoryIDE},
		{"Alacritty", categoryIDE},
		{"kitty", categoryIDE},

		// Docs
		{"Pages", categoryDocs},
		{"Google Docs", categoryDocs},
		{"Microsoft Word", categoryDocs},
		{"Notes", categoryDocs},
		{"Notion", categoryDocs},
		{"Obsidian", categoryDocs},

		// Browser
		{"Firefox", categoryBrowser},
		{"Google Chrome", categoryBrowser},
		{"Safari", categoryBrowser},
		{"Arc", categoryBrowser},
		{"Brave Browser", categoryBrowser},

		// Default
		{"Finder", categoryDefault},
		{"", categoryDefault},
		{"RandomApp", categoryDefault},
	}

	for _, c := range cases {
		ctx := &windowctx.Context{AppName: c.appName}
		if got := detectCategory(ctx); got != c.want {
			t.Errorf("AppName=%q: got %v, want %v", c.appName, got, c.want)
		}
	}
}

func TestDetectCategory_EmailByTitle(t *testing.T) {
	// A browser window showing Gmail should be classified as email, not browser.
	ctx := &windowctx.Context{AppName: "Google Chrome", WindowTitle: "Inbox - user@gmail.com - Gmail"}
	if got := detectCategory(ctx); got != categoryEmail {
		t.Errorf("Chrome with Gmail title: got %v, want categoryEmail", got)
	}

	ctx = &windowctx.Context{AppName: "Firefox", WindowTitle: "Outlook - Posteingang"}
	if got := detectCategory(ctx); got != categoryEmail {
		t.Errorf("Firefox with Outlook title: got %v, want categoryEmail", got)
	}
}

func TestDetectCategory_ChatTakesPrecedenceOverEmail(t *testing.T) {
	// If a chat app somehow also matches email keywords in the title, chat wins
	// because it's checked first. Documents the actual behavior.
	ctx := &windowctx.Context{AppName: "Slack", WindowTitle: "mail thread"}
	if got := detectCategory(ctx); got != categoryChat {
		t.Errorf("Slack with 'mail' in title: got %v, want categoryChat", got)
	}
}

func TestCategoryToString(t *testing.T) {
	cases := []struct {
		cat  appCategory
		want string
	}{
		{categoryChat, "chat"},
		{categoryEmail, "email"},
		{categoryIDE, "ide"},
		{categoryDocs, "docs"},
		{categoryBrowser, "browser"},
		{categoryDefault, "default"},
		{appCategory(99), "default"}, // unknown falls through to default
	}
	for _, c := range cases {
		if got := categoryToString(c.cat); got != c.want {
			t.Errorf("categoryToString(%v): got %q, want %q", c.cat, got, c.want)
		}
	}
}

func TestCategoryName(t *testing.T) {
	if got := CategoryName(nil); got != "default" {
		t.Errorf("nil ctx: got %q, want %q", got, "default")
	}

	cases := []struct {
		appName string
		want    string
	}{
		{"Slack", "chat"},
		{"Mail", "email"},
		{"Cursor", "ide"},
		{"Notion", "docs"},
		{"Safari", "browser"},
		{"Finder", "default"},
	}
	for _, c := range cases {
		ctx := &windowctx.Context{AppName: c.appName}
		if got := CategoryName(ctx); got != c.want {
			t.Errorf("AppName=%q: got %q, want %q", c.appName, got, c.want)
		}
	}
}

func TestToneInstruction_German(t *testing.T) {
	cases := []struct {
		cat        appCategory
		wantSubstr string
	}{
		{categoryChat, "casual"},
		{categoryEmail, "formal"},
		{categoryIDE, "technisch"},
		{categoryDocs, "neutral"},
		{categoryBrowser, "neutral"},
	}
	for _, c := range cases {
		got := toneInstruction(c.cat, "de")
		if got == "" {
			t.Errorf("de %v: got empty string", c.cat)
		}
		if !strings.Contains(got, c.wantSubstr) {
			t.Errorf("de %v: got %q, want substring %q", c.cat, got, c.wantSubstr)
		}
		if !strings.HasPrefix(got, "\n- Ton:") {
			t.Errorf("de %v: got %q, want prefix '\\n- Ton:'", c.cat, got)
		}
	}

	if got := toneInstruction(categoryDefault, "de"); got != "" {
		t.Errorf("de default: got %q, want empty", got)
	}
}

func TestToneInstruction_English(t *testing.T) {
	cases := []struct {
		cat        appCategory
		wantSubstr string
	}{
		{categoryChat, "casual"},
		{categoryEmail, "formal"},
		{categoryIDE, "technical"},
		{categoryDocs, "neutral"},
		{categoryBrowser, "neutral"},
	}
	for _, c := range cases {
		got := toneInstruction(c.cat, "en")
		if got == "" {
			t.Errorf("en %v: got empty string", c.cat)
		}
		if !strings.Contains(got, c.wantSubstr) {
			t.Errorf("en %v: got %q, want substring %q", c.cat, got, c.wantSubstr)
		}
		if !strings.HasPrefix(got, "\n- Tone:") {
			t.Errorf("en %v: got %q, want prefix '\\n- Tone:'", c.cat, got)
		}
	}

	if got := toneInstruction(categoryDefault, "en"); got != "" {
		t.Errorf("en default: got %q, want empty", got)
	}
}

func TestNilCleaner_ReturnsInputUnchanged(t *testing.T) {
	n := &NilCleaner{}
	cases := []string{
		"",
		"hello world",
		"ähm, das ist ein Test",
		"multi\nline\ntext",
	}
	for _, in := range cases {
		got, err := n.Cleanup(in, "de", nil, []string{"Kubernetes"})
		if err != nil {
			t.Errorf("NilCleaner.Cleanup(%q): unexpected error %v", in, err)
		}
		if got != in {
			t.Errorf("NilCleaner.Cleanup(%q): got %q, want %q", in, got, in)
		}
	}
}

func TestNewCleanerFromConfig_Ollama(t *testing.T) {
	// Default baseURL and model when unspecified.
	c := NewCleanerFromConfig("ollama", "ignored", "", "")
	cc, ok := c.(*Cleaner)
	if !ok {
		t.Fatalf("ollama: got %T, want *Cleaner", c)
	}
	if cc.baseURL != "http://localhost:11434/v1" {
		t.Errorf("ollama baseURL: got %q, want %q", cc.baseURL, "http://localhost:11434/v1")
	}
	if cc.model != "llama3.2" {
		t.Errorf("ollama model: got %q, want %q", cc.model, "llama3.2")
	}
	if cc.apiKey != "" {
		t.Errorf("ollama apiKey: got %q, want empty (ignored)", cc.apiKey)
	}

	// Custom baseURL and model are respected.
	c = NewCleanerFromConfig("ollama", "", "http://ollama.local:1234/v1", "my-model")
	cc = c.(*Cleaner)
	if cc.baseURL != "http://ollama.local:1234/v1" {
		t.Errorf("custom ollama baseURL: got %q, want %q", cc.baseURL, "http://ollama.local:1234/v1")
	}
	if cc.model != "my-model" {
		t.Errorf("custom ollama model: got %q, want %q", cc.model, "my-model")
	}
}

func TestNewCleanerFromConfig_None(t *testing.T) {
	c := NewCleanerFromConfig("none", "key", "url", "model")
	if _, ok := c.(*NilCleaner); !ok {
		t.Errorf("none: got %T, want *NilCleaner", c)
	}
}

func TestNewCleanerFromConfig_OpenAIDefault(t *testing.T) {
	// Empty backend falls through to OpenAI default.
	c := NewCleanerFromConfig("", "sk-xxx", "", "")
	cc, ok := c.(*Cleaner)
	if !ok {
		t.Fatalf("default: got %T, want *Cleaner", c)
	}
	if cc.baseURL != "https://api.openai.com/v1" {
		t.Errorf("default baseURL: got %q, want %q", cc.baseURL, "https://api.openai.com/v1")
	}
	if cc.model != "gpt-4o-mini" {
		t.Errorf("default model: got %q, want %q", cc.model, "gpt-4o-mini")
	}
	if cc.apiKey != "sk-xxx" {
		t.Errorf("default apiKey: got %q, want %q", cc.apiKey, "sk-xxx")
	}

	// Unknown backend also falls through to OpenAI. Custom model respected.
	c = NewCleanerFromConfig("openai", "sk-yyy", "", "gpt-5")
	cc = c.(*Cleaner)
	if cc.baseURL != "https://api.openai.com/v1" {
		t.Errorf("openai baseURL: got %q, want %q", cc.baseURL, "https://api.openai.com/v1")
	}
	if cc.model != "gpt-5" {
		t.Errorf("openai model: got %q, want %q", cc.model, "gpt-5")
	}
}

// newTestServer builds an httptest.Server that decodes the incoming chat
// request, calls the given handler for assertions, and returns a fixed
// response. The returned content is what Cleanup should produce on success.
func newTestServer(t *testing.T, statusCode int, responseContent string, inspect func(*testing.T, *http.Request, chatRequest)) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, err := io.ReadAll(r.Body)
		if err != nil {
			t.Errorf("read request body: %v", err)
		}
		var req chatRequest
		if len(body) > 0 {
			if err := json.Unmarshal(body, &req); err != nil {
				t.Errorf("unmarshal request: %v (body=%s)", err, string(body))
			}
		}
		if inspect != nil {
			inspect(t, r, req)
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(statusCode)
		if statusCode == http.StatusOK {
			resp := chatResponse{}
			resp.Choices = append(resp.Choices, struct {
				Message struct {
					Content string `json:"content"`
				} `json:"message"`
			}{})
			resp.Choices[0].Message.Content = responseContent
			_ = json.NewEncoder(w).Encode(resp)
		} else {
			_, _ = w.Write([]byte(`{"error":"boom"}`))
		}
	}))
}

func TestCleanup_HappyPath(t *testing.T) {
	ctx := &windowctx.Context{AppName: "Slack"}

	server := newTestServer(t, http.StatusOK, "hey there", func(t *testing.T, r *http.Request, req chatRequest) {
		// Endpoint
		if r.URL.Path != "/chat/completions" {
			t.Errorf("path: got %q, want %q", r.URL.Path, "/chat/completions")
		}
		// Method
		if r.Method != http.MethodPost {
			t.Errorf("method: got %q, want POST", r.Method)
		}
		// Auth header
		if got := r.Header.Get("Authorization"); got != "Bearer sk-test" {
			t.Errorf("auth header: got %q, want %q", got, "Bearer sk-test")
		}
		// Content-Type
		if got := r.Header.Get("Content-Type"); got != "application/json" {
			t.Errorf("content-type: got %q, want application/json", got)
		}
		// Model
		if req.Model != "test-model" {
			t.Errorf("model: got %q, want %q", req.Model, "test-model")
		}
		// Messages: system + user
		if len(req.Messages) != 2 {
			t.Fatalf("messages: got %d, want 2", len(req.Messages))
		}
		if req.Messages[0].Role != "system" {
			t.Errorf("message[0] role: got %q, want system", req.Messages[0].Role)
		}
		if req.Messages[1].Role != "user" {
			t.Errorf("message[1] role: got %q, want user", req.Messages[1].Role)
		}
		// User turn wraps the transcript in a sentinel tag (prompt-injection hardening).
		if !strings.Contains(req.Messages[1].Content, "hallo welt") {
			t.Errorf("user content should contain transcript: %q", req.Messages[1].Content)
		}
		if req.Messages[1].Content == "hallo welt" {
			t.Errorf("user content should be wrapped, got bare transcript: %q", req.Messages[1].Content)
		}
		// System prompt contains German base
		if !strings.Contains(req.Messages[0].Content, "Textbereiniger") {
			t.Errorf("system prompt missing German base: %q", req.Messages[0].Content)
		}
		// Tone instruction for chat/de
		if !strings.Contains(req.Messages[0].Content, "casual") {
			t.Errorf("system prompt missing chat tone: %q", req.Messages[0].Content)
		}
		// Dictionary appended
		if !strings.Contains(req.Messages[0].Content, "Bevorzugte Schreibweisen") {
			t.Errorf("system prompt missing dictionary header: %q", req.Messages[0].Content)
		}
		if !strings.Contains(req.Messages[0].Content, "Kubernetes") {
			t.Errorf("system prompt missing dictionary entry: %q", req.Messages[0].Content)
		}
	})
	defer server.Close()

	c := NewCleanerWithConfig("sk-test", server.URL, "test-model")
	got, err := c.Cleanup("hallo welt", "de", ctx, []string{"Kubernetes", "GitHub"})
	if err != nil {
		t.Fatalf("Cleanup: unexpected error %v", err)
	}
	if got != "hey there" {
		t.Errorf("Cleanup: got %q, want %q", got, "hey there")
	}
}

func TestCleanup_EnglishBasePromptAndDictionary(t *testing.T) {
	ctx := &windowctx.Context{AppName: "Visual Studio Code"}

	server := newTestServer(t, http.StatusOK, "cleaned", func(t *testing.T, r *http.Request, req chatRequest) {
		sys := req.Messages[0].Content
		if !strings.Contains(sys, "text cleaner for speech input") {
			t.Errorf("system prompt missing English base: %q", sys)
		}
		if !strings.Contains(sys, "technical") {
			t.Errorf("system prompt missing IDE tone (en): %q", sys)
		}
		if !strings.Contains(sys, "Preferred spellings") {
			t.Errorf("system prompt missing English dictionary header: %q", sys)
		}
	})
	defer server.Close()

	c := NewCleanerWithConfig("sk", server.URL, "m")
	if _, err := c.Cleanup("hello", "en", ctx, []string{"Kubernetes"}); err != nil {
		t.Fatalf("Cleanup: %v", err)
	}
}

func TestCleanup_NoAuthHeaderWhenAPIKeyEmpty(t *testing.T) {
	server := newTestServer(t, http.StatusOK, "ok", func(t *testing.T, r *http.Request, _ chatRequest) {
		if got := r.Header.Get("Authorization"); got != "" {
			t.Errorf("auth header should be empty, got %q", got)
		}
	})
	defer server.Close()

	c := NewCleanerWithConfig("", server.URL, "m")
	if _, err := c.Cleanup("hi", "en", nil, nil); err != nil {
		t.Fatalf("Cleanup: %v", err)
	}
}

func TestCleanup_NoDictionaryHeaderWhenEmpty(t *testing.T) {
	server := newTestServer(t, http.StatusOK, "ok", func(t *testing.T, _ *http.Request, req chatRequest) {
		sys := req.Messages[0].Content
		if strings.Contains(sys, "Preferred spellings") || strings.Contains(sys, "Bevorzugte Schreibweisen") {
			t.Errorf("empty dictionary should not add header: %q", sys)
		}
	})
	defer server.Close()

	c := NewCleanerWithConfig("sk", server.URL, "m")
	if _, err := c.Cleanup("hi", "en", nil, nil); err != nil {
		t.Fatalf("Cleanup: %v", err)
	}
}

func TestCleanup_ErrorStatus(t *testing.T) {
	server := newTestServer(t, http.StatusInternalServerError, "", nil)
	defer server.Close()

	c := NewCleanerWithConfig("sk", server.URL, "m")
	_, err := c.Cleanup("hi", "en", nil, nil)
	if err == nil {
		t.Fatal("expected error on 500, got nil")
	}
	if !strings.Contains(err.Error(), "LLM API error") {
		t.Errorf("error message: got %q, want substring %q", err.Error(), "LLM API error")
	}
	if !strings.Contains(err.Error(), "500") {
		t.Errorf("error message should include status 500: %q", err.Error())
	}
}

func TestCleanup_402ReturnsInsufficientCredits(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusPaymentRequired)
		_, _ = w.Write([]byte(`{"error":{"message":"You exceeded your current quota","type":"insufficient_quota","code":"insufficient_quota"}}`))
	}))
	defer server.Close()

	c := NewCleanerWithConfig("sk-broke", server.URL, "m")
	_, err := c.Cleanup("hi", "en", nil, nil)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !errors.Is(err, apierr.ErrInsufficientCredits) {
		t.Errorf("errors.Is(err, ErrInsufficientCredits) = false; err = %v", err)
	}
	if !strings.Contains(err.Error(), "402") {
		t.Errorf("error should mention status 402, got: %v", err)
	}
}

func TestCleanup_429WithInsufficientQuotaReturnsInsufficientCredits(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusTooManyRequests)
		_, _ = w.Write([]byte(`{"error":{"message":"You exceeded your current quota","type":"insufficient_quota","code":"insufficient_quota"}}`))
	}))
	defer server.Close()

	c := NewCleanerWithConfig("sk-broke", server.URL, "m")
	_, err := c.Cleanup("hi", "en", nil, nil)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !errors.Is(err, apierr.ErrInsufficientCredits) {
		t.Errorf("errors.Is(err, ErrInsufficientCredits) = false; err = %v", err)
	}
}

func TestCleanup_429PlainRateLimitIsNotCreditError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusTooManyRequests)
		_, _ = w.Write([]byte(`{"error":{"message":"Rate limit reached","type":"requests","code":"rate_limit_exceeded"}}`))
	}))
	defer server.Close()

	c := NewCleanerWithConfig("sk", server.URL, "m")
	_, err := c.Cleanup("hi", "en", nil, nil)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if errors.Is(err, apierr.ErrInsufficientCredits) {
		t.Errorf("plain rate-limit 429 should NOT be credit error; err = %v", err)
	}
	if !strings.Contains(err.Error(), "429") {
		t.Errorf("error should mention status 429, got: %v", err)
	}
}

func TestCleanup_EmptyChoices(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"choices":[]}`))
	}))
	defer server.Close()

	c := NewCleanerWithConfig("sk", server.URL, "m")
	_, err := c.Cleanup("hi", "en", nil, nil)
	if err == nil {
		t.Fatal("expected error on empty choices, got nil")
	}
	if !strings.Contains(err.Error(), "no response") {
		t.Errorf("error message: got %q, want substring %q", err.Error(), "no response")
	}
}

func TestCleanup_MalformedResponse(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`not json`))
	}))
	defer server.Close()

	c := NewCleanerWithConfig("sk", server.URL, "m")
	_, err := c.Cleanup("hi", "en", nil, nil)
	if err == nil {
		t.Fatal("expected error on malformed response, got nil")
	}
	if !strings.Contains(err.Error(), "response parse") {
		t.Errorf("error message: got %q, want substring %q", err.Error(), "response parse")
	}
}

func TestCleanupWithCustomPrompts_CategoryMatchReplacesBase(t *testing.T) {
	ctx := &windowctx.Context{AppName: "Slack"}

	server := newTestServer(t, http.StatusOK, "ok", func(t *testing.T, _ *http.Request, req chatRequest) {
		sys := req.Messages[0].Content
		// Custom prompt replaces base; base + tone should NOT appear.
		if !strings.HasPrefix(sys, "CUSTOM CHAT PROMPT") {
			t.Errorf("system prompt should start with custom: %q", sys)
		}
		if strings.Contains(sys, "Textbereiniger") {
			t.Errorf("German base prompt should not appear: %q", sys)
		}
		if strings.Contains(sys, "casual") {
			t.Errorf("tone instruction should not appear with custom prompt: %q", sys)
		}
		// Dictionary is still appended after the custom prompt.
		if !strings.Contains(sys, "Bevorzugte Schreibweisen") {
			t.Errorf("dictionary header missing: %q", sys)
		}
		if !strings.Contains(sys, "GitHub") {
			t.Errorf("dictionary entry missing: %q", sys)
		}
	})
	defer server.Close()

	c := NewCleanerWithConfig("sk", server.URL, "m")
	prompts := map[string]string{"chat": "CUSTOM CHAT PROMPT"}
	_, err := c.CleanupWithCustomPrompts("hi", "de", ctx, []string{"GitHub"}, prompts)
	if err != nil {
		t.Fatalf("Cleanup: %v", err)
	}
}

func TestCleanupWithCustomPrompts_FallsBackToBaseWhenCategoryMissing(t *testing.T) {
	// Custom prompts provided but none for the detected category → base prompt + tone.
	ctx := &windowctx.Context{AppName: "Slack"}

	server := newTestServer(t, http.StatusOK, "ok", func(t *testing.T, _ *http.Request, req chatRequest) {
		sys := req.Messages[0].Content
		if !strings.Contains(sys, "Textbereiniger") {
			t.Errorf("expected fallback to German base: %q", sys)
		}
		if !strings.Contains(sys, "casual") {
			t.Errorf("expected chat tone in fallback: %q", sys)
		}
	})
	defer server.Close()

	c := NewCleanerWithConfig("sk", server.URL, "m")
	prompts := map[string]string{"email": "only-email"}
	if _, err := c.CleanupWithCustomPrompts("hi", "de", ctx, nil, prompts); err != nil {
		t.Fatalf("Cleanup: %v", err)
	}
}

func TestNewCleaner_Defaults(t *testing.T) {
	c := NewCleaner("sk-abc")
	if c.apiKey != "sk-abc" {
		t.Errorf("apiKey: got %q, want %q", c.apiKey, "sk-abc")
	}
	if c.baseURL != "https://api.openai.com/v1" {
		t.Errorf("baseURL: got %q, want %q", c.baseURL, "https://api.openai.com/v1")
	}
	if c.model != "gpt-4o-mini" {
		t.Errorf("model: got %q, want %q", c.model, "gpt-4o-mini")
	}
}

// --- Prompt-Injection-Hardening ---

func TestBuildPromptMessages_WrapsTranscriptInSentinel(t *testing.T) {
	sys, user := buildPromptMessages("BASE", "hello world", "SENT123", "en")

	// System message keeps the original base prompt and appends hardening.
	if !strings.HasPrefix(sys, "BASE") {
		t.Errorf("system should start with base prompt, got %q", sys)
	}
	// System mentions the sentinel so the LLM knows where the data is.
	if !strings.Contains(sys, "<SENT123>") || !strings.Contains(sys, "</SENT123>") {
		t.Errorf("system should reference sentinel tags, got %q", sys)
	}

	// User message wraps the transcript in exactly the sentinel tags.
	if !strings.Contains(user, "<SENT123>") || !strings.Contains(user, "</SENT123>") {
		t.Errorf("user should contain sentinel tags, got %q", user)
	}
	if !strings.Contains(user, "hello world") {
		t.Errorf("user should contain the transcript, got %q", user)
	}

	// The transcript content must sit between the opening and closing tags.
	openIdx := strings.Index(user, "<SENT123>")
	closeIdx := strings.Index(user, "</SENT123>")
	if openIdx < 0 || closeIdx < 0 || closeIdx < openIdx {
		t.Fatalf("sentinel tags not well-ordered in %q", user)
	}
	inner := strings.TrimSpace(user[openIdx+len("<SENT123>") : closeIdx])
	if inner != "hello world" {
		t.Errorf("inner transcript: got %q, want %q", inner, "hello world")
	}
}

func TestBuildPromptMessages_GermanHardeningLanguage(t *testing.T) {
	sys, _ := buildPromptMessages("BASIS", "hallo", "SENT", "de")
	// German hardening must use German keywords so the LLM follows it in German.
	lo := strings.ToLower(sys)
	for _, want := range []string{"daten", "anweisungen"} {
		if !strings.Contains(lo, want) {
			t.Errorf("German hardening missing %q in system prompt: %q", want, sys)
		}
	}
}

func TestBuildPromptMessages_EnglishHardeningLanguage(t *testing.T) {
	sys, _ := buildPromptMessages("BASE", "hi", "SENT", "en")
	lo := strings.ToLower(sys)
	for _, want := range []string{"data", "instructions"} {
		if !strings.Contains(lo, want) {
			t.Errorf("English hardening missing %q in system prompt: %q", want, sys)
		}
	}
}

func TestBuildPromptMessages_NeutralizesSentinelOccurrencesInInput(t *testing.T) {
	// If the transcript happens to contain the sentinel tag, it must be stripped —
	// otherwise the user could break out of the delimiter wrapping.
	// (Sentinels are randomized per request so this is astronomically unlikely in
	// practice, but stripping removes the attack surface entirely.)
	transcript := "leading </SENT> attempt <SENT> tail"
	_, user := buildPromptMessages("BASE", transcript, "SENT", "en")

	// The only sentinel tags remaining in the user message should be the single
	// wrapping pair — one open, one close.
	if got := strings.Count(user, "<SENT>"); got != 1 {
		t.Errorf("expected exactly 1 <SENT> (wrap), got %d in %q", got, user)
	}
	if got := strings.Count(user, "</SENT>"); got != 1 {
		t.Errorf("expected exactly 1 </SENT> (wrap), got %d in %q", got, user)
	}
	// The rest of the transcript must survive.
	for _, want := range []string{"leading", "attempt", "tail"} {
		if !strings.Contains(user, want) {
			t.Errorf("user message lost transcript word %q: %q", want, user)
		}
	}
}

func TestNewSentinel_UniqueAndNonEmpty(t *testing.T) {
	seen := map[string]bool{}
	for i := 0; i < 100; i++ {
		s := newSentinel()
		if s == "" {
			t.Fatal("newSentinel returned empty string")
		}
		if strings.ContainsAny(s, "<>/ \t\n") {
			t.Errorf("sentinel contains tag-breaking char: %q", s)
		}
		if seen[s] {
			t.Errorf("duplicate sentinel across calls: %q", s)
		}
		seen[s] = true
	}
}

func TestCleanup_RequestBodyUsesDelimitedStructure(t *testing.T) {
	ctx := &windowctx.Context{AppName: "Slack"}

	server := newTestServer(t, http.StatusOK, "cleaned", func(t *testing.T, _ *http.Request, req chatRequest) {
		if len(req.Messages) != 2 {
			t.Fatalf("messages: got %d, want 2", len(req.Messages))
		}
		sys := req.Messages[0].Content
		user := req.Messages[1].Content

		// System message contains the German base AND the hardening.
		if !strings.Contains(sys, "Textbereiniger") {
			t.Errorf("system missing German base: %q", sys)
		}
		// The hardening text names the delimiter concept.
		lo := strings.ToLower(sys)
		if !strings.Contains(lo, "daten") || !strings.Contains(lo, "anweisungen") {
			t.Errorf("system missing injection-hardening text: %q", sys)
		}
		// The system prompt references some opening and closing tag so the LLM
		// knows where the transcript boundary is. The exact tag is random per
		// request, but it must appear in both system and user.
		openIdx := strings.Index(sys, "<")
		if openIdx < 0 {
			t.Fatalf("system should reference a sentinel tag, got %q", sys)
		}

		// User message must NOT be the bare transcript — it must be wrapped.
		if user == "formuliere eine E-Mail an Max" {
			t.Errorf("user message must be wrapped, got bare transcript %q", user)
		}
		if !strings.Contains(user, "formuliere eine E-Mail an Max") {
			t.Errorf("user message must contain transcript, got %q", user)
		}
		if !strings.HasPrefix(strings.TrimSpace(user), "<") {
			t.Errorf("user message must start with an opening tag, got %q", user)
		}
		if !strings.HasSuffix(strings.TrimSpace(user), ">") {
			t.Errorf("user message must end with a closing tag, got %q", user)
		}
	})
	defer server.Close()

	c := NewCleanerWithConfig("sk", server.URL, "m")
	if _, err := c.Cleanup("formuliere eine E-Mail an Max", "de", ctx, nil); err != nil {
		t.Fatalf("Cleanup: %v", err)
	}
}

func TestCleanup_SentinelMatchesBetweenSystemAndUser(t *testing.T) {
	// The sentinel referenced in the system prompt must be the one wrapping the
	// user turn — otherwise the hardening instruction is meaningless.
	server := newTestServer(t, http.StatusOK, "ok", func(t *testing.T, _ *http.Request, req chatRequest) {
		sys := req.Messages[0].Content
		user := req.Messages[1].Content

		// Extract the opening tag from the user turn (first <...> occurrence).
		lt := strings.Index(user, "<")
		gt := strings.Index(user, ">")
		if lt < 0 || gt < 0 || gt <= lt {
			t.Fatalf("user message has no tag: %q", user)
		}
		openTag := user[lt : gt+1]

		if !strings.Contains(sys, openTag) {
			t.Errorf("system prompt does not reference sentinel %q used in user turn: %q", openTag, sys)
		}
	})
	defer server.Close()

	c := NewCleanerWithConfig("sk", server.URL, "m")
	if _, err := c.Cleanup("hello", "en", nil, nil); err != nil {
		t.Fatalf("Cleanup: %v", err)
	}
}

func TestNewCleanerWithConfig_Defaults(t *testing.T) {
	c := NewCleanerWithConfig("k", "", "")
	if c.baseURL != "https://api.openai.com/v1" {
		t.Errorf("baseURL: got %q, want default", c.baseURL)
	}
	if c.model != "gpt-4o-mini" {
		t.Errorf("model: got %q, want default", c.model)
	}

	c = NewCleanerWithConfig("k", "http://x/v1", "m")
	if c.baseURL != "http://x/v1" {
		t.Errorf("baseURL: got %q, want http://x/v1", c.baseURL)
	}
	if c.model != "m" {
		t.Errorf("model: got %q, want m", c.model)
	}
}
