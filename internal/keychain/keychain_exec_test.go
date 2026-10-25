//go:build !windows

package keychain

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// With no helper on PATH — the headless-server case — every operation must
// report ErrUnavailable, and none may reach a real keychain. An empty PATH
// directory makes exec's lookup fail before anything runs.
func TestAllOperations_HelperMissing(t *testing.T) {
	t.Setenv("PATH", t.TempDir())

	if _, err := Get("vox-test", "k"); !errors.Is(err, ErrUnavailable) {
		t.Errorf("Get: got %v, want ErrUnavailable", err)
	}
	if err := Set("vox-test", "k", "v"); !errors.Is(err, ErrUnavailable) {
		t.Errorf("Set: got %v, want ErrUnavailable", err)
	}
	if err := Delete("vox-test", "k"); !errors.Is(err, ErrUnavailable) {
		t.Errorf("Delete: got %v, want ErrUnavailable", err)
	}
	if HasKey("vox-test", "k") {
		t.Error("HasKey: got true without a keychain")
	}
}

// fakeHelper puts a shell script named like the keychain helper first on
// PATH, so Get/Set/Delete run it instead of the real tool. body is the
// script after the shebang.
func fakeHelper(t *testing.T, body string) {
	t.Helper()
	dir := t.TempDir()
	script := filepath.Join(dir, tool)
	if err := os.WriteFile(script, []byte("#!/bin/sh\n"+body+"\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", dir)
}

// A helper that never answers — secret-tool waiting for a prompt nobody can
// see — must not hang the caller: the run is cut off at helperTimeout and the
// error says so.
func TestGet_HelperTimesOut(t *testing.T) {
	fakeHelper(t, "exec /bin/sleep 10")
	old := helperTimeout
	helperTimeout = 200 * time.Millisecond
	t.Cleanup(func() { helperTimeout = old })

	start := time.Now()
	_, err := Get("vox-test", "k")
	elapsed := time.Since(start)

	if err == nil {
		t.Fatal("expected an error from a helper that does not answer")
	}
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Errorf("error should wrap context.DeadlineExceeded, got %v", err)
	}
	if errors.Is(err, ErrUnavailable) {
		t.Errorf("a timeout is not a missing keychain, got %v", err)
	}
	for _, want := range []string{tool, "did not answer", "200ms"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("error should contain %q, got %q", want, err)
		}
	}
	if elapsed > 5*time.Second {
		t.Errorf("Get took %v, the timeout did not cut the helper off", elapsed)
	}
}

// secret-tool exits 1 both when the item is missing and when there is no
// Secret Service to ask; only its stderr tells the two apart, so a message it
// wrote is part of the error.
func TestGet_HelperFailsWithMessage(t *testing.T) {
	fakeHelper(t, `echo 'secret-tool: Cannot autolaunch D-Bus without X11 $DISPLAY' >&2; exit 1`)

	_, err := Get("vox-test", "k")
	if err == nil {
		t.Fatal("expected an error")
	}
	if errors.Is(err, ErrUnavailable) {
		t.Errorf("a helper that ran is not a missing keychain, got %v", err)
	}
	msg := err.Error()
	if !strings.Contains(msg, "Cannot autolaunch D-Bus") {
		t.Errorf("error should carry the helper's stderr, got %q", msg)
	}
	if strings.Contains(msg, "not found") {
		t.Errorf("a helper that said why did not report 'not found', got %q", msg)
	}
}

// A silent non-zero exit is the plain "no such item" answer.
func TestGet_HelperFailsSilently(t *testing.T) {
	fakeHelper(t, "exit 1")

	_, err := Get("vox-test", "k")
	if err == nil {
		t.Fatal("expected an error")
	}
	if !strings.Contains(err.Error(), `key "k" not found`) {
		t.Errorf("silent exit should read as not found, got %q", err)
	}
	if strings.HasSuffix(err.Error(), ": ") {
		t.Errorf("no stderr must not leave a dangling colon, got %q", err)
	}
	if HasKey("vox-test", "k") {
		t.Error("HasKey: got true for a key the helper does not have")
	}
}

func TestSetDelete_HelperFailsWithMessage(t *testing.T) {
	fakeHelper(t, `echo 'secret-tool: The name org.freedesktop.secrets was not provided by any .service files' >&2; exit 1`)

	if err := Set("vox-test", "k", "v"); err == nil || !strings.Contains(err.Error(), "org.freedesktop.secrets") {
		t.Errorf("Set: error should carry the helper's stderr, got %v", err)
	}
	if err := Delete("vox-test", "k"); err == nil || !strings.Contains(err.Error(), "org.freedesktop.secrets") {
		t.Errorf("Delete: error should carry the helper's stderr, got %v", err)
	}
}

// A helper that answers in time, with the value on stdout, is the normal path.
func TestGet_HelperAnswers(t *testing.T) {
	fakeHelper(t, `printf 'sk-fake-value\n'`)

	key, err := Get("vox-test", "k")
	if err != nil || key != "sk-fake-value" {
		t.Errorf("got (%q, %v), want the helper's stdout trimmed", key, err)
	}
}

func TestHelperStderr_OneLineAndCut(t *testing.T) {
	multi := &exec.ExitError{Stderr: []byte("  line one\n\n  line two  \n")}
	if got := helperStderr(multi); got != "line one line two" {
		t.Errorf("helperStderr = %q, want lines collapsed", got)
	}
	long := &exec.ExitError{Stderr: []byte(strings.Repeat("x", stderrLimit+50))}
	if got := helperStderr(long); len([]rune(got)) != stderrLimit+1 || !strings.HasSuffix(got, "…") {
		t.Errorf("helperStderr should cut at %d and mark it, got %d runes", stderrLimit, len([]rune(got)))
	}
	if got := helperStderr(errors.New("not an exit error")); got != "" {
		t.Errorf("helperStderr of a non-exit error = %q, want empty", got)
	}
}
