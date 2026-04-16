package inject

import "testing"

func TestParseMethod(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want Method
	}{
		{"clipboard", "clipboard", Clipboard},
		{"wtype", "wtype", Wtype},
		{"ydotool", "ydotool", Ydotool},
		{"stdout explicit", "stdout", Stdout},
		{"empty falls back to stdout", "", Stdout},
		{"unknown falls back to stdout", "xclip", Stdout},
		{"case sensitive: uppercase not matched", "CLIPBOARD", Stdout},
		{"case sensitive: mixed case not matched", "Wtype", Stdout},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := ParseMethod(tt.in); got != tt.want {
				t.Errorf("ParseMethod(%q) = %q, want %q", tt.in, got, tt.want)
			}
		})
	}
}
