//go:build !windows

package keychain

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os/exec"
	"strings"
	"time"
	"unicode/utf8"
)

// helperTimeout bounds one call of the keychain helper. A person unlocking a
// desktop keyring answers the prompt well within it; a helper that waits for a
// prompter that never comes — secret-tool on a server that has a Secret
// Service on the bus but no session to show the prompt in — is killed instead
// of hanging the caller. A variable so a test can shorten it.
var helperTimeout = 30 * time.Second

// stderrLimit is how many bytes of the helper's stderr the error carries,
// cut at a character boundary: one line is the cause, more is a stack of GLib
// warnings.
const stderrLimit = 200

// run executes the keychain helper with helperTimeout and returns its stdout.
// A helper that exits non-zero comes back as *exec.ExitError with its stderr
// attached; one that hit the timeout comes back as context.DeadlineExceeded,
// wrapped in a message that names the helper.
func run(stdin io.Reader, args ...string) ([]byte, error) {
	ctx, cancel := context.WithTimeout(context.Background(), helperTimeout)
	defer cancel()
	cmd := exec.CommandContext(ctx, tool, args...)
	cmd.Stdin = stdin
	// Bound the wait for the output pipes, after the kill and after a normal
	// exit alike: a grandchild that still holds them would otherwise keep the
	// call open for as long as it lives.
	cmd.WaitDelay = time.Second
	out, err := cmd.Output()
	// exec reports the kill as an ExitError, not as the context's error —
	// the context says whether the deadline is what ended the run.
	if err != nil && ctx.Err() != nil {
		return out, fmt.Errorf("%s did not answer within %v: %w", tool, helperTimeout, ctx.Err())
	}
	// A helper that answered and exited 0 while a child of its own still held
	// stdout comes back as ErrWaitDelay. Its answer is complete — the helper
	// itself had exited — so it counts.
	if errors.Is(err, exec.ErrWaitDelay) && cmd.ProcessState != nil && cmd.ProcessState.Success() {
		return out, nil
	}
	return out, err
}

// lookupError is the error Get returns when run failed. A helper that is not
// on PATH means there is no keychain here (ErrUnavailable). One that ran and
// exited non-zero is "not found" only when it said nothing: secret-tool exits
// 1 both for "no such item" (silently) and for "no Secret Service on the bus"
// (with a message), and only the message tells the two apart — so it rides
// along, trimmed to one line. security on macOS answers a missing item with
// its own exit status and a message; keychain_darwin.go maps that to notFound
// before it gets here.
func lookupError(key string, err error) error {
	if missingTool(err) {
		return unavailable(tool)
	}
	if errors.Is(err, context.DeadlineExceeded) {
		return fmt.Errorf("keychain: lookup of %q failed: %w", key, err)
	}
	if msg := helperStderr(err); msg != "" {
		return fmt.Errorf("keychain: lookup of %q failed: %w: %s", key, err, msg)
	}
	return notFound(key, err)
}

// notFound is the plain "no such key" answer of a helper that ran.
func notFound(key string, err error) error {
	return fmt.Errorf("keychain: key %q not found: %w", key, err)
}

// writeError is the error Set and Delete return when run failed; what is the
// verb of the message ("set", "delete").
func writeError(what, key string, err error) error {
	if missingTool(err) {
		return unavailable(tool)
	}
	if msg := helperStderr(err); msg != "" {
		return fmt.Errorf("keychain: failed to %s %q: %w: %s", what, key, err, msg)
	}
	return fmt.Errorf("keychain: failed to %s %q: %w", what, key, err)
}

// helperStderr returns what the helper wrote to stderr as one line, cut to
// stderrLimit — empty when it wrote nothing or err is not an exit error. The
// helper's own message ("secret-tool: …", "security: …") is the cause; when
// it wrote one, the last such line is returned on its own, so GLib warnings
// printed before it cannot push it past the cut. Otherwise all lines are
// joined.
func helperStderr(err error) string {
	var exitErr *exec.ExitError
	if !errors.As(err, &exitErr) {
		return ""
	}
	var lines []string
	cause := ""
	for _, line := range strings.Split(string(exitErr.Stderr), "\n") {
		line = strings.Join(strings.Fields(line), " ")
		if line == "" {
			continue
		}
		lines = append(lines, line)
		if strings.HasPrefix(line, tool+": ") {
			cause = line
		}
	}
	msg := cause
	if msg == "" {
		msg = strings.Join(lines, " ")
	}
	return cut(msg, stderrLimit)
}

// cut shortens s to at most limit bytes without splitting a character, and
// marks the cut with an ellipsis. Helper messages are translated, so a
// multi-byte character can sit at the limit.
func cut(s string, limit int) string {
	if len(s) <= limit {
		return s
	}
	end := limit
	for end > 0 && !utf8.RuneStart(s[end]) {
		end--
	}
	return s[:end] + "…"
}
