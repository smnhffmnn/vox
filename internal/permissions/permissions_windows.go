//go:build windows

package permissions

func checkPlatform() Status {
	return Status{
		Accessibility: true, // Windows doesn't gate keyboard hooks via permissions
		Microphone:    true, // Windows prompts at recording time
	}
}

func openAccessibilitySettings() {}
func openMicrophoneSettings()    {}
