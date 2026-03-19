//go:build darwin

package permissions

/*
#cgo CFLAGS: -x objective-c
#cgo LDFLAGS: -framework Cocoa -framework ApplicationServices -framework AVFoundation

#include <stdbool.h>

// Accessibility
extern bool AXIsProcessTrusted(void);

// Microphone (implemented in permissions_darwin.m)
bool checkMicrophonePermission(void);
void requestMicrophonePermission(void);
void openAccessibilityPrefs(void);
void openMicrophonePrefs(void);
*/
import "C"

func checkPlatform() Status {
	return Status{
		Accessibility: bool(C.AXIsProcessTrusted()),
		Microphone:    bool(C.checkMicrophonePermission()),
	}
}

func openAccessibilitySettings() {
	C.openAccessibilityPrefs()
}

func openMicrophoneSettings() {
	// This triggers the macOS permission prompt (if not yet asked),
	// or opens the settings if already denied.
	C.openMicrophonePrefs()
}
