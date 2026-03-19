#import <AVFoundation/AVFoundation.h>
#import <Cocoa/Cocoa.h>

bool checkMicrophonePermission(void) {
    if (@available(macOS 10.14, *)) {
        AVAuthorizationStatus status = [AVCaptureDevice authorizationStatusForMediaType:AVMediaTypeAudio];
        return status == AVAuthorizationStatusAuthorized;
    }
    return true;
}

void requestMicrophonePermission(void) {
    if (@available(macOS 10.14, *)) {
        [AVCaptureDevice requestAccessForMediaType:AVMediaTypeAudio completionHandler:^(BOOL granted) {
            // Permission result will be picked up by the next checkMicrophonePermission call
        }];
    }
}

void openAccessibilityPrefs(void) {
    NSURL *url = [NSURL URLWithString:@"x-apple.systempreferences:com.apple.preference.security?Privacy_Accessibility"];
    [[NSWorkspace sharedWorkspace] openURL:url];
}

void openMicrophonePrefs(void) {
    // First request permission (triggers macOS prompt if not yet asked)
    requestMicrophonePermission();
}
