// Package openpath reveals a file in the platform's file manager.
//
// Used for the history JSONL and stored recordings: revealing a file is
// reliable everywhere, whereas "open with the default app" depends on a
// handler being registered for the extension — for .jsonl there usually
// is none.
package openpath

import (
	"fmt"
	"path/filepath"
)

// Reveal shows path in the platform file manager: with the file selected where
// the platform supports that, otherwise at the containing directory.
//
// The path must be absolute. Every caller passes one built from the config
// directory, and requiring it here keeps a relative or option-like value from
// ever reaching the command line.
func Reveal(path string) error {
	if !filepath.IsAbs(path) {
		return fmt.Errorf("refusing to reveal a relative path: %q", path)
	}
	return reveal(path)
}
