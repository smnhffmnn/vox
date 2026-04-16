package config

import (
	"reflect"
	"testing"
)

func TestParseSnippets_Empty(t *testing.T) {
	snippets, err := parseSnippets("")
	if err != nil {
		t.Fatalf("parseSnippets err = %v", err)
	}
	if snippets != nil {
		t.Errorf("snippets = %v, want nil", snippets)
	}
}

func TestParseSnippets_DashTriggerForm(t *testing.T) {
	data := `- trigger: "greeting"
  text: "Hello, thanks for reaching out!"
- trigger: "bye"
  text: "Goodbye!"
`
	got, err := parseSnippets(data)
	if err != nil {
		t.Fatalf("parseSnippets err = %v", err)
	}
	want := []Snippet{
		{Trigger: "greeting", Text: "Hello, thanks for reaching out!"},
		{Trigger: "bye", Text: "Goodbye!"},
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got %v, want %v", got, want)
	}
}

func TestParseSnippets_BareTriggerForm(t *testing.T) {
	data := `trigger: foo
text: bar
trigger: baz
text: qux
`
	got, err := parseSnippets(data)
	if err != nil {
		t.Fatalf("parseSnippets err = %v", err)
	}
	want := []Snippet{
		{Trigger: "foo", Text: "bar"},
		{Trigger: "baz", Text: "qux"},
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got %v, want %v", got, want)
	}
}

func TestParseSnippets_UnquotedValues(t *testing.T) {
	data := `- trigger: hello
  text: world
`
	got, err := parseSnippets(data)
	if err != nil {
		t.Fatalf("parseSnippets err = %v", err)
	}
	want := []Snippet{{Trigger: "hello", Text: "world"}}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got %v, want %v", got, want)
	}
}

func TestParseSnippets_EscapedNewlines(t *testing.T) {
	// `\n` in the YAML source becomes a real newline in the snippet text.
	data := "- trigger: \"multi\"\n  text: \"line1\\nline2\\nline3\"\n"
	got, err := parseSnippets(data)
	if err != nil {
		t.Fatalf("parseSnippets err = %v", err)
	}
	want := []Snippet{{Trigger: "multi", Text: "line1\nline2\nline3"}}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestParseSnippets_DoubleEscapedNewlinesAreLiteral(t *testing.T) {
	// `\\n` in the YAML source must remain the literal two-char sequence `\n`
	// (backslash + n), not become a newline.
	data := "- trigger: \"lit\"\n  text: \"a\\\\nb\"\n"
	got, err := parseSnippets(data)
	if err != nil {
		t.Fatalf("parseSnippets err = %v", err)
	}
	want := []Snippet{{Trigger: "lit", Text: `a\nb`}}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestParseSnippets_MixedEscapes(t *testing.T) {
	// `a\nb\\nc` → `a` + newline + `b` + literal `\n` + `c`.
	data := "- trigger: mix\n  text: \"a\\nb\\\\nc\"\n"
	got, err := parseSnippets(data)
	if err != nil {
		t.Fatalf("parseSnippets err = %v", err)
	}
	want := []Snippet{{Trigger: "mix", Text: "a\nb" + `\n` + "c"}}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestParseSnippets_IgnoresCommentsAndBlankLines(t *testing.T) {
	data := `
# top comment
- trigger: "a"
  text: "A"

  # indented comment
- trigger: "b"
  text: "B"
`
	got, err := parseSnippets(data)
	if err != nil {
		t.Fatalf("parseSnippets err = %v", err)
	}
	want := []Snippet{
		{Trigger: "a", Text: "A"},
		{Trigger: "b", Text: "B"},
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got %v, want %v", got, want)
	}
}

func TestParseSnippets_TriggerWithoutText(t *testing.T) {
	// Trigger without a following text line still emits the snippet (with empty Text).
	data := `- trigger: "solo"
`
	got, err := parseSnippets(data)
	if err != nil {
		t.Fatalf("parseSnippets err = %v", err)
	}
	want := []Snippet{{Trigger: "solo", Text: ""}}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got %v, want %v", got, want)
	}
}

func TestParseSnippets_TextBeforeTriggerIgnored(t *testing.T) {
	// `text:` without a preceding trigger must not panic; it is simply ignored.
	data := `text: "orphan"
- trigger: "real"
  text: "value"
`
	got, err := parseSnippets(data)
	if err != nil {
		t.Fatalf("parseSnippets err = %v", err)
	}
	want := []Snippet{{Trigger: "real", Text: "value"}}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got %v, want %v", got, want)
	}
}

func TestMatchSnippet_CaseInsensitive(t *testing.T) {
	snippets := []Snippet{
		{Trigger: "Greeting", Text: "Hi!"},
		{Trigger: "BYE", Text: "bye."},
	}
	tests := []struct {
		name     string
		input    string
		wantText string
		wantOK   bool
	}{
		{"exact match", "Greeting", "Hi!", true},
		{"lower case", "greeting", "Hi!", true},
		{"upper case", "GREETING", "Hi!", true},
		{"upper trigger lower input", "bye", "bye.", true},
		{"no match", "hello", "", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			text, ok := MatchSnippet(tt.input, snippets)
			if ok != tt.wantOK {
				t.Errorf("ok = %v, want %v", ok, tt.wantOK)
			}
			if text != tt.wantText {
				t.Errorf("text = %q, want %q", text, tt.wantText)
			}
		})
	}
}

func TestMatchSnippet_TrimsWhitespace(t *testing.T) {
	snippets := []Snippet{{Trigger: "hello", Text: "world"}}
	text, ok := MatchSnippet("   hello  \n", snippets)
	if !ok {
		t.Fatal("expected match")
	}
	if text != "world" {
		t.Errorf("text = %q, want %q", text, "world")
	}
}

func TestMatchSnippet_EmptyInputs(t *testing.T) {
	if text, ok := MatchSnippet("", nil); ok || text != "" {
		t.Errorf("empty input on nil snippets: ok=%v text=%q", ok, text)
	}
	if text, ok := MatchSnippet("anything", nil); ok || text != "" {
		t.Errorf("on nil snippets: ok=%v text=%q", ok, text)
	}
}

func TestMatchSnippet_FirstMatchWins(t *testing.T) {
	snippets := []Snippet{
		{Trigger: "dup", Text: "first"},
		{Trigger: "DUP", Text: "second"},
	}
	text, ok := MatchSnippet("dup", snippets)
	if !ok {
		t.Fatal("expected match")
	}
	if text != "first" {
		t.Errorf("text = %q, want %q (first match should win)", text, "first")
	}
}

func TestMatchSnippet_PartialNotMatched(t *testing.T) {
	// Trigger matching is exact, not substring.
	snippets := []Snippet{{Trigger: "hello", Text: "world"}}
	if _, ok := MatchSnippet("hello there", snippets); ok {
		t.Error("substring should not match")
	}
	if _, ok := MatchSnippet("say hello", snippets); ok {
		t.Error("substring should not match")
	}
}
