package inject

import (
	"fmt"
	"os/exec"
	"strings"
)

type Method string

const (
	Stdout    Method = "stdout"
	Clipboard Method = "clipboard"
	Wtype     Method = "wtype"
	Ydotool   Method = "ydotool"
)

func ParseMethod(s string) Method {
	switch s {
	case "clipboard":
		return Clipboard
	case "wtype":
		return Wtype
	case "ydotool":
		return Ydotool
	default:
		return Stdout
	}
}

// Inject delivers the text using the specified method.
func Inject(method Method, text string) error {
	switch method {
	case Clipboard:
		return clipboard(text)
	case Wtype:
		return wtype(text)
	case Ydotool:
		return ydotool(text)
	default:
		fmt.Print(text)
		return nil
	}
}

func clipboard(text string) error {
	cmd := exec.Command("wl-copy")
	cmd.Stdin = strings.NewReader(text)
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("wl-copy fehlgeschlagen (ist wl-clipboard installiert?): %w", err)
	}
	return nil
}

func wtype(text string) error {
	cmd := exec.Command("wtype", "--", text)
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("wtype fehlgeschlagen (ist wtype installiert?): %w", err)
	}
	return nil
}

func ydotool(text string) error {
	cmd := exec.Command("ydotool", "type", "--", text)
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("ydotool fehlgeschlagen (ist ydotool installiert?): %w", err)
	}
	return nil
}
