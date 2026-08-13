//go:build darwin

package inject

import (
	"os"
	"os/exec"
	"strings"
	"testing"
)

// skipUnlessPasteboardAllowed guards tests that overwrite the user's real
// clipboard. They always run on CI; locally they are opt-in.
func skipUnlessPasteboardAllowed(t *testing.T) {
	t.Helper()
	if os.Getenv("CI") == "" && os.Getenv("VOX_PASTEBOARD_TEST") == "" {
		t.Skip("overwrites the real clipboard; set VOX_PASTEBOARD_TEST=1 to run locally")
	}
}

// readPasteboardIndependently reads the general pasteboard through
// AppleScript. Deliberately NOT pbpaste: pbpaste makes the same
// locale-dependent conversion as pbcopy in reverse, so a pbcopy/pbpaste
// round trip looked clean while every other app saw mojibake (VOX-15).
func readPasteboardIndependently(t *testing.T) string {
	t.Helper()
	out, err := exec.Command("osascript", "-e", "the clipboard as «class utf8»").Output()
	if err != nil {
		t.Fatalf("reading pasteboard via osascript: %v", err)
	}
	// osascript terminates its output with exactly one newline.
	return strings.TrimSuffix(string(out), "\n")
}

// TestClipboardKeepsUTF8Intact pins the VOX-15 contract: what vox puts on the
// pasteboard is what an independent reader gets back, umlauts included. With
// the pbcopy implementation and no locale in the environment this returned
// "√∂" for "ö".
func TestClipboardKeepsUTF8Intact(t *testing.T) {
	skipUnlessPasteboardAllowed(t)

	const text = "Grüße — ö ü ä ß, 12 €, 日本語"
	if err := clipboard(text); err != nil {
		t.Fatalf("clipboard(%q): %v", text, err)
	}
	if got := readPasteboardIndependently(t); got != text {
		t.Errorf("pasteboard round trip = %q, want %q", got, text)
	}
}

// TestClipboardEmptyString: delivering an empty transcript must not fail.
func TestClipboardEmptyString(t *testing.T) {
	skipUnlessPasteboardAllowed(t)

	if err := clipboard(""); err != nil {
		t.Fatalf(`clipboard(""): %v`, err)
	}
	if got := readPasteboardIndependently(t); got != "" {
		t.Errorf("pasteboard after empty write = %q, want empty", got)
	}
}

// TestClipboardRejectsInvalidUTF8: transcription output is always valid
// UTF-8, but if a caller ever passes broken bytes the write must fail loudly
// instead of storing garbage.
func TestClipboardRejectsInvalidUTF8(t *testing.T) {
	skipUnlessPasteboardAllowed(t)

	if err := clipboard("\xff\xfe"); err == nil {
		t.Error("clipboard with invalid UTF-8 succeeded, want error")
	}
}
