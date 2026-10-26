//go:build linux

package keychain

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// secret-tool store reads the secret from stdin — it must never appear in
// the arguments, where every user on the machine can read it via ps. The
// fake helper records both.
func TestSet_PassesSecretOnStdin(t *testing.T) {
	fakeHelper(t, `printf '%s\n' "$@" > "$0.args"; /bin/cat > "$0.stdin"`)
	script := filepath.Join(strings.Split(os.Getenv("PATH"), string(os.PathListSeparator))[0], tool)

	if err := Set("vox-test", "k", "sk-secret-value"); err != nil {
		t.Fatalf("Set: %v", err)
	}

	stdin, err := os.ReadFile(script + ".stdin")
	if err != nil {
		t.Fatal(err)
	}
	if string(stdin) != "sk-secret-value" {
		t.Errorf("stdin = %q, want the secret", stdin)
	}
	args, err := os.ReadFile(script + ".args")
	if err != nil {
		t.Fatal(err)
	}
	want := "store\n--label=vox: k\nservice\nvox-test\nkey\nk\n"
	if string(args) != want {
		t.Errorf("args = %q, want %q", args, want)
	}
	if strings.Contains(string(args), "sk-secret-value") {
		t.Error("the secret is in the arguments")
	}
}
