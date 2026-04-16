package config

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func TestLoadDictionary_MissingFileReturnsNilNoError(t *testing.T) {
	setHome(t)

	words, err := LoadDictionary()
	if err != nil {
		t.Fatalf("LoadDictionary() err = %v, want nil", err)
	}
	if words != nil {
		t.Errorf("words = %v, want nil", words)
	}
}

func TestLoadDictionary_ReadsWords(t *testing.T) {
	home := setHome(t)
	dir := filepath.Join(home, ".config", "vox")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	content := `MYFIT24
Naskor
Whisper
`
	if err := os.WriteFile(filepath.Join(dir, "dictionary.txt"), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}

	words, err := LoadDictionary()
	if err != nil {
		t.Fatalf("LoadDictionary() err = %v", err)
	}
	want := []string{"MYFIT24", "Naskor", "Whisper"}
	if !reflect.DeepEqual(words, want) {
		t.Errorf("words = %v, want %v", words, want)
	}
}

func TestLoadDictionary_IgnoresCommentsAndBlankLines(t *testing.T) {
	home := setHome(t)
	dir := filepath.Join(home, ".config", "vox")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	content := `# Comment at top

  # Indented comment
MYFIT24

Naskor

# Another comment
Whisper
`
	if err := os.WriteFile(filepath.Join(dir, "dictionary.txt"), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}

	words, err := LoadDictionary()
	if err != nil {
		t.Fatalf("LoadDictionary() err = %v", err)
	}
	want := []string{"MYFIT24", "Naskor", "Whisper"}
	if !reflect.DeepEqual(words, want) {
		t.Errorf("words = %v, want %v", words, want)
	}
}

func TestLoadDictionary_TrimsWhitespace(t *testing.T) {
	home := setHome(t)
	dir := filepath.Join(home, ".config", "vox")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	content := "  MYFIT24  \n\tNaskor\t\n"
	if err := os.WriteFile(filepath.Join(dir, "dictionary.txt"), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}

	words, err := LoadDictionary()
	if err != nil {
		t.Fatalf("LoadDictionary() err = %v", err)
	}
	want := []string{"MYFIT24", "Naskor"}
	if !reflect.DeepEqual(words, want) {
		t.Errorf("words = %v, want %v", words, want)
	}
}

func TestLoadDictionary_EmptyFileReturnsNil(t *testing.T) {
	home := setHome(t)
	dir := filepath.Join(home, ".config", "vox")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "dictionary.txt"), []byte(""), 0o644); err != nil {
		t.Fatal(err)
	}

	words, err := LoadDictionary()
	if err != nil {
		t.Fatalf("LoadDictionary() err = %v", err)
	}
	if words != nil {
		t.Errorf("words = %v, want nil", words)
	}
}

func TestLoadDictionary_OnlyCommentsReturnsNil(t *testing.T) {
	home := setHome(t)
	dir := filepath.Join(home, ".config", "vox")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	content := "# only comments\n# nothing else\n"
	if err := os.WriteFile(filepath.Join(dir, "dictionary.txt"), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}

	words, err := LoadDictionary()
	if err != nil {
		t.Fatalf("LoadDictionary() err = %v", err)
	}
	if words != nil {
		t.Errorf("words = %v, want nil", words)
	}
}
