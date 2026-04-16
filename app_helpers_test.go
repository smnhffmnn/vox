package main

import "testing"

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
		// Negative cases that could false-trigger are worth guarding.
		{"word 'thanks' alone is fine", "Thanks, that was great.", false},
		{"word 'subtitles' alone is fine", "Add subtitles to the video.", false},
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
