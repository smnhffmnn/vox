package hotkey

import "testing"

func TestParseKey(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want Key
	}{
		{"right_option", "right_option", RightOption},
		// "right_alt" is an alias that also maps to RightOption (the canonical value).
		{"right_alt alias", "right_alt", RightOption},
		{"f13", "f13", F13},
		{"f14", "f14", F14},
		{"f15", "f15", F15},
		{"f16", "f16", F16},
		{"f17", "f17", F17},
		{"f18", "f18", F18},
		{"f19", "f19", F19},
		{"f20", "f20", F20},
		{"empty falls back to RightOption", "", RightOption},
		{"unknown falls back to RightOption", "space", RightOption},
		{"case sensitive: uppercase F13 not matched", "F13", RightOption},
		{"case sensitive: RIGHT_OPTION not matched", "RIGHT_OPTION", RightOption},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := ParseKey(tt.in); got != tt.want {
				t.Errorf("ParseKey(%q) = %q, want %q", tt.in, got, tt.want)
			}
		})
	}
}
