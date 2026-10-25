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
)

// helperTimeout bounds one call of the keychain helper. A person unlocking a
// desktop keyring answers the prompt well within it; a helper that waits for a
// prompter that never comes — secret-tool on a server that has a Secret
// Service on the bus but no session to show the prompt in — is killed instead
// of hanging the caller. A variable so a test can shorten it.
var helperTimeout = 30 * time.Second

// stderrLimit is how much of the helper's stderr the error carries: one line
// is the cause, more is a stack of GLib warnings.
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
	// After the kill, do not wait forever for a grandchild that still holds
	// the output pipe.
	cmd.WaitDelay = time.Second
	out, err := cmd.Output()
	// exec reports the kill as an ExitError, not as the context's error —
	// the context says whether the deadline is what ended the run.
	if err != nil && ctx.Err() != nil {
		return out, fmt.Errorf("%s did not answer within %v: %w", tool, helperTimeout, ctx.Err())
	}
	return out, err
}

// lookupError is the error Get returns when run failed. A helper that is not
// on PATH means there is no keychain here (ErrUnavailable). One that ran and
// exited non-zero is "not found" only when it said nothing: secret-tool exits
// 1 both for "no such item" (silently) and for "no Secret Service on the bus"
// (with a message), and only the message tells the two apart — so it rides
// along, trimmed to one line.
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

// helperStderr returns what the helper wrote to stderr, collapsed to one line
// and cut to stderrLimit — empty when it wrote nothing or err is not an exit
// error.
func helperStderr(err error) string {
	var exitErr *exec.ExitError
	if !errors.As(err, &exitErr) {
		return ""
	}
	msg := strings.Join(strings.Fields(string(exitErr.Stderr)), " ")
	if len(msg) > stderrLimit {
		msg = msg[:stderrLimit] + "…"
	}
	return msg
}
