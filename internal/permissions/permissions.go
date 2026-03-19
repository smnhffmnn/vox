package permissions

// Status represents the state of a system permission.
type Status struct {
	Accessibility bool `json:"accessibility"`
	Microphone    bool `json:"microphone"`
}

// Check returns the current permission status.
func Check() Status {
	return checkPlatform()
}

// OpenAccessibilitySettings opens the OS accessibility settings.
func OpenAccessibilitySettings() {
	openAccessibilitySettings()
}

// OpenMicrophoneSettings opens the OS microphone privacy settings.
func OpenMicrophoneSettings() {
	openMicrophoneSettings()
}
