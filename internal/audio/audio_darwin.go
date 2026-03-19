//go:build darwin

package audio

/*
#cgo CFLAGS: -x objective-c
#cgo LDFLAGS: -framework CoreAudio

#include <CoreAudio/CoreAudio.h>
#include <string.h>

// getInputDeviceName returns the name of the default input device.
const char* getDefaultInputDeviceName(void) {
    AudioObjectPropertyAddress addr = {
        kAudioHardwarePropertyDefaultInputDevice,
        kAudioObjectPropertyScopeGlobal,
        kAudioObjectPropertyElementMain
    };
    AudioDeviceID deviceID = 0;
    UInt32 size = sizeof(deviceID);
    OSStatus status = AudioObjectGetPropertyData(kAudioObjectSystemObject, &addr, 0, NULL, &size, &deviceID);
    if (status != noErr || deviceID == 0) return "";

    AudioObjectPropertyAddress nameAddr = {
        kAudioObjectPropertyName,
        kAudioObjectPropertyScopeGlobal,
        kAudioObjectPropertyElementMain
    };
    CFStringRef name = NULL;
    size = sizeof(name);
    status = AudioObjectGetPropertyData(deviceID, &nameAddr, 0, NULL, &size, &name);
    if (status != noErr || name == NULL) return "";

    static char buf[256];
    CFStringGetCString(name, buf, sizeof(buf), kCFStringEncodingUTF8);
    CFRelease(name);
    return buf;
}
*/
import "C"
import "fmt"

func init() {
	name := C.GoString(C.getDefaultInputDeviceName())
	if name != "" {
		fmt.Printf("vox: default input device: %s\n", name)
	}
}
