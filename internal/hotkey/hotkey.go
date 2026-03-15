package hotkey

// Listener listens for global hotkey events.
type Listener interface {
	// Listen blocks and calls onPress/onRelease for hotkey events.
	// Returns when Close() is called or an error occurs.
	Listen(onPress func(), onRelease func()) error
	// Close stops the listener.
	Close() error
}

// Key represents a hotkey identifier.
type Key string

const (
	RightOption Key = "right_option"
	RightAlt    Key = "right_alt"
	F13         Key = "f13"
	F14         Key = "f14"
	F15         Key = "f15"
	F16         Key = "f16"
	F17         Key = "f17"
	F18         Key = "f18"
	F19         Key = "f19"
	F20         Key = "f20"
)

// ParseKey converts a config string to a Key. Defaults to RightOption/RightAlt.
func ParseKey(s string) Key {
	switch s {
	case "right_option", "right_alt":
		return RightOption
	case "f13":
		return F13
	case "f14":
		return F14
	case "f15":
		return F15
	case "f16":
		return F16
	case "f17":
		return F17
	case "f18":
		return F18
	case "f19":
		return F19
	case "f20":
		return F20
	default:
		return RightOption
	}
}
