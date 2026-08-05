package openpath

import (
	"strings"
	"testing"
)

// Reveal hands its argument to a file manager on the command line, so it must
// refuse anything that is not an absolute path — a relative value, or one that
// could be read as an option.
func TestReveal_RejectsRelativePaths(t *testing.T) {
	for _, path := range []string{"", "relative/file.wav", "./file.wav", "-R", "--help"} {
		err := Reveal(path)
		if err == nil {
			t.Errorf("Reveal(%q) was accepted, want a refusal", path)
			continue
		}
		if !strings.Contains(err.Error(), "relative") {
			t.Errorf("Reveal(%q) error = %v, want it to name the reason", path, err)
		}
	}
}
