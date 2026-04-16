package config

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func TestLoadCustomPrompts_MissingDirReturnsNil(t *testing.T) {
	setHome(t)

	if got := LoadCustomPrompts(); got != nil {
		t.Errorf("got %v, want nil", got)
	}
}

func TestLoadCustomPrompts_EmptyDirReturnsNil(t *testing.T) {
	home := setHome(t)
	dir := filepath.Join(home, ".config", "vox", "prompts")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}

	if got := LoadCustomPrompts(); got != nil {
		t.Errorf("got %v, want nil", got)
	}
}

func TestLoadCustomPrompts_ReadsTxtFiles(t *testing.T) {
	home := setHome(t)
	dir := filepath.Join(home, ".config", "vox", "prompts")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}

	files := map[string]string{
		"chat.txt":    "chat prompt",
		"email.txt":   "email prompt",
		"default.txt": "default prompt",
	}
	for name, content := range files {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	got := LoadCustomPrompts()
	want := map[string]string{
		"chat":    "chat prompt",
		"email":   "email prompt",
		"default": "default prompt",
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got %v, want %v", got, want)
	}
}

func TestLoadCustomPrompts_TrimsWhitespace(t *testing.T) {
	home := setHome(t)
	dir := filepath.Join(home, ".config", "vox", "prompts")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}

	content := "  \n\n  hello there  \n\n  "
	if err := os.WriteFile(filepath.Join(dir, "chat.txt"), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}

	got := LoadCustomPrompts()
	want := map[string]string{"chat": "hello there"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got %v, want %v", got, want)
	}
}

func TestLoadCustomPrompts_IgnoresNonTxtFiles(t *testing.T) {
	home := setHome(t)
	dir := filepath.Join(home, ".config", "vox", "prompts")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}

	files := map[string]string{
		"chat.txt":         "kept",
		"email.md":         "ignored",
		"README":           "ignored",
		"config.yaml":      "ignored",
		"notes.txt.backup": "ignored",
	}
	for name, content := range files {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	got := LoadCustomPrompts()
	want := map[string]string{"chat": "kept"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got %v, want %v", got, want)
	}
}

func TestLoadCustomPrompts_IgnoresSubdirectories(t *testing.T) {
	home := setHome(t)
	dir := filepath.Join(home, ".config", "vox", "prompts")
	if err := os.MkdirAll(filepath.Join(dir, "subdir.txt"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "chat.txt"), []byte("content"), 0o644); err != nil {
		t.Fatal(err)
	}

	got := LoadCustomPrompts()
	want := map[string]string{"chat": "content"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got %v, want %v", got, want)
	}
}

func TestLoadCustomPrompts_SkipsEmptyAfterTrim(t *testing.T) {
	home := setHome(t)
	dir := filepath.Join(home, ".config", "vox", "prompts")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "empty.txt"), []byte("   \n\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "chat.txt"), []byte("real"), 0o644); err != nil {
		t.Fatal(err)
	}

	got := LoadCustomPrompts()
	want := map[string]string{"chat": "real"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got %v, want %v", got, want)
	}
}

func TestLoadCustomPrompts_OnlyEmptyFilesReturnsNil(t *testing.T) {
	home := setHome(t)
	dir := filepath.Join(home, ".config", "vox", "prompts")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "a.txt"), []byte(""), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "b.txt"), []byte("  \n\t"), 0o644); err != nil {
		t.Fatal(err)
	}

	if got := LoadCustomPrompts(); got != nil {
		t.Errorf("got %v, want nil", got)
	}
}
