package main

import (
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/smnhffmnn/vox/internal/logbuf"
)

func TestIsHallucination(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want bool
	}{
		{"empty string", "", true},
		{"only whitespace", "   \t\n", true},
		{"plain German sentence", "Das ist ein normaler Satz.", false},
		{"plain English sentence", "This is a regular transcription.", false},
		{"untertitel exact", "Untertitel", true},
		{"untertitel in context", "Untertitel im Auftrag des ZDF", true},
		{"amara marker", "Subtitles by the Amara.org community", true},
		{"thanks for watching", "Thanks for watching!", true},
		{"thank you for watching uppercase", "THANK YOU FOR WATCHING", true},
		{"please subscribe", "Please subscribe to my channel", true},
		{"bitte abonnieren", "Bitte abonnieren und liken", true},
		// "vielen dank f" with umlaut — stripNonLetters drops umlauts,
		// so "vielen dank für" becomes "vielen dank fr" which still contains "vielen dank f".
		{"vielen dank mit Umlaut", "Vielen Dank fürs Zuschauen", true},
		{"bis zum naechsten", "Bis zum nächsten Mal", true},
		{"mooji url", "www.mooji.org", true},
		{"watchmojo copyright", "Copyright WatchMojo 2020", true},
		{"subtitles by marker", "subtitles by someone", true},
		// Issue 9 additions — YouTube outro patterns and ZDF/SWR markers.
		{"abonniert den Kanal", "Abonniert den Kanal für mehr Videos", true},
		{"SWR 2019 outro", "Untertitel: SWR 2019", true},
		// Outro URL regex — URL at line end is a classic Whisper hallucination.
		{"url at end (de)", "Mehr Informationen auf www.mein-blog.de", true},
		{"url at end (com)", "Besuche uns auf www.example.com", true},
		{"url at end (org)", "Sieh dich um auf www.foo.org", true},
		{"url at end with trailing whitespace", "Danke für's Zuhören www.blog.de  \n", true},
		// Negative cases that could false-trigger are worth guarding.
		{"word 'thanks' alone is fine", "Thanks, that was great.", false},
		{"word 'subtitles' alone is fine", "Add subtitles to the video.", false},
		// "abonniert" alone (e.g. legitimate usage) must not trigger — only
		// "abonniert den" does. Catches the YouTube-outro pattern without
		// breaking normal German dictation about subscriptions.
		{"zeitung abonniert alone is fine", "Ich habe die Zeitung abonniert.", false},
		// URL regex only fires at end of line — mid-sentence URLs are legitimate
		// dictation content and must pass through.
		{"url mid-sentence is fine", "Die Webseite www.mein-blog.de steht dort.", false},
		{"url is not 'www' prefixed is fine", "Die Domain foo.de gehört uns, mehr nicht.", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := isHallucination(tt.in); got != tt.want {
				t.Errorf("isHallucination(%q) = %v, want %v", tt.in, got, tt.want)
			}
		})
	}
}

func TestStripNonLetters(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want string
	}{
		{"empty", "", ""},
		{"lowercase letters kept", "abcxyz", "abcxyz"},
		{"digits kept", "0123456789", "0123456789"},
		{"spaces kept", "a b c", "a b c"},
		{"uppercase stripped", "ABCdef", "def"},
		{"punctuation stripped", "hello, world!", "hello world"},
		{"umlauts stripped", "für über groß", "fr ber gro"},
		{"mixed", "Vielen Dank für's Zuschauen!", "ielen ank frs uschauen"},
		{"only symbols", "!@#$%^&*()", ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := stripNonLetters(tt.in); got != tt.want {
				t.Errorf("stripNonLetters(%q) = %q, want %q", tt.in, got, tt.want)
			}
		})
	}
}

func TestEscapeYAMLValue(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want string
	}{
		{"empty", "", ""},
		{"plain text", "hello world", "hello world"},
		{"single quote unchanged", "it's fine", "it's fine"},
		{"double quote escaped", `say "hi"`, `say \"hi\"`},
		{"backslash escaped", `a\b`, `a\\b`},
		{"backslash before quote: backslash escaped first", `a\"b`, `a\\\"b`},
		{"multiple backslashes", `\\`, `\\\\`},
		{"mixed", `a\b"c`, `a\\b\"c`},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := escapeYAMLValue(tt.in); got != tt.want {
				t.Errorf("escapeYAMLValue(%q) = %q, want %q", tt.in, got, tt.want)
			}
		})
	}
}

func TestRedactQueryStrings(t *testing.T) {
	cases := []struct {
		name string
		in   string
		want string
	}{
		{
			name: "credential in a backend URL is removed",
			in:   `Post "https://gateway.example/v1/audio?key=sk-secret123": dial tcp: timeout`,
			want: `Post "https://gateway.example/v1/audio?…": dial tcp: timeout`,
		},
		{
			name: "URL without a query is untouched",
			in:   `Post "https://api.openai.com/v1/audio/transcriptions": EOF`,
			want: `Post "https://api.openai.com/v1/audio/transcriptions": EOF`,
		},
		{
			name: "plain message is untouched",
			in:   "transcription: 401 invalid api key",
			want: "transcription: 401 invalid api key",
		},
		{
			name: "several URLs are all redacted",
			in:   "http://a.local/x?t=1 and https://b.local/y?u=2",
			want: "http://a.local/x?… and https://b.local/y?…",
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := redactQueryStrings(c.in); got != c.want {
				t.Errorf("redactQueryStrings(%q) = %q, want %q", c.in, got, c.want)
			}
		})
	}
}

func TestStorableError(t *testing.T) {
	if got := storableError(nil); got != "" {
		t.Errorf("storableError(nil) = %q, want empty", got)
	}

	// A backend can echo a whole response body; the entry should say what went
	// wrong, not archive the response.
	long := errors.New(strings.Repeat("a", maxStoredErrorLen+200))
	got := storableError(long)
	if len(got) > maxStoredErrorLen+len("…") {
		t.Errorf("storableError produced %d chars, want at most %d", len(got), maxStoredErrorLen+len("…"))
	}
	if !strings.HasSuffix(got, "…") {
		t.Error("a truncated message should be marked as truncated")
	}

	// Redaction applies to what gets persisted.
	withKey := errors.New(`Post "https://gw.local/v1?key=sk-abc": EOF`)
	if strings.Contains(storableError(withKey), "sk-abc") {
		t.Error("a query-string credential must not reach the history file")
	}
}

func TestStepOf(t *testing.T) {
	if got := stepOf(errors.New("plain"), logbuf.StepSTT); got != logbuf.StepSTT {
		t.Errorf("stepOf on a plain error = %q, want the fallback %q", got, logbuf.StepSTT)
	}
	wrapped := pipelineErrf(logbuf.StepCleanup, "boom")
	if got := stepOf(wrapped, logbuf.StepSTT); got != logbuf.StepCleanup {
		t.Errorf("stepOf = %q, want %q", got, logbuf.StepCleanup)
	}
	// The step must survive further wrapping, since callers use errors.Is too.
	if got := stepOf(fmt.Errorf("context: %w", wrapped), logbuf.StepSTT); got != logbuf.StepCleanup {
		t.Errorf("stepOf through a wrap = %q, want %q", got, logbuf.StepCleanup)
	}
}

func TestPipelineErrf_PreservesWrappedSentinels(t *testing.T) {
	err := pipelineErrf(logbuf.StepSTT, "%w", errNoSpeech)
	if !errors.Is(err, errNoSpeech) {
		t.Error("pipelineErrf must keep a wrapped sentinel matchable — the pipeline drops the recording based on it")
	}
	if stepOf(err, logbuf.StepApp) != logbuf.StepSTT {
		t.Error("step lost")
	}
}
