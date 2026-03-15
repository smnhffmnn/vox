package inject

import "fmt"

// Method represents a text output method.
type Method string

const (
	Stdout    Method = "stdout"
	Clipboard Method = "clipboard"
	Wtype     Method = "wtype"
	Ydotool   Method = "ydotool"
)

// ParseMethod converts a string to a Method constant.
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
