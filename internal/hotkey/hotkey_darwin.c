#include <Carbon/Carbon.h>
#import <Cocoa/Cocoa.h>

// Defined in Go via //export
extern void goHotkeyDown(void);
extern void goHotkeyUp(void);
extern void goEscapePressed(void);

static int targetKeyCode = 61; // Right Option default

static id keyDownMonitor = nil;
static id keyUpMonitor = nil;
static id flagsMonitor = nil;
static BOOL targetKeyPressed = NO;

// Escape key via CGEvent tap (more reliable than NSEvent global monitor)
static CFMachPortRef escapeTapPort = NULL;
static CFRunLoopSourceRef escapeTapSource = NULL;

void voxSetTargetKeyCode(int code) {
    targetKeyCode = code;
}

static BOOL isModifierKey(int code) {
    return (code == 61 || code == 58 || code == 54 || code == 55 ||
            code == 60 || code == 56 || code == 62 || code == 59 || code == 63);
}

void voxStartMonitor(void) {
    if (isModifierKey(targetKeyCode)) {
        flagsMonitor = [NSEvent addGlobalMonitorForEventsMatchingMask:NSEventMaskFlagsChanged
            handler:^(NSEvent *event) {
                if ([event keyCode] == targetKeyCode) {
                    NSEventModifierFlags flags = [event modifierFlags];
                    BOOL pressed = NO;

                    switch (targetKeyCode) {
                        case 61: case 58:
                            pressed = (flags & NSEventModifierFlagOption) != 0;
                            break;
                        case 54: case 55:
                            pressed = (flags & NSEventModifierFlagCommand) != 0;
                            break;
                        case 60: case 56:
                            pressed = (flags & NSEventModifierFlagShift) != 0;
                            break;
                        case 62: case 59:
                            pressed = (flags & NSEventModifierFlagControl) != 0;
                            break;
                        default:
                            pressed = NO;
                            break;
                    }

                    if (pressed && !targetKeyPressed) {
                        targetKeyPressed = YES;
                        goHotkeyDown();
                    } else if (!pressed && targetKeyPressed) {
                        targetKeyPressed = NO;
                        goHotkeyUp();
                    }
                }
            }];
    } else {
        keyDownMonitor = [NSEvent addGlobalMonitorForEventsMatchingMask:NSEventMaskKeyDown
            handler:^(NSEvent *event) {
                if ([event keyCode] == targetKeyCode && !targetKeyPressed) {
                    targetKeyPressed = YES;
                    goHotkeyDown();
                }
            }];
        keyUpMonitor = [NSEvent addGlobalMonitorForEventsMatchingMask:NSEventMaskKeyUp
            handler:^(NSEvent *event) {
                if ([event keyCode] == targetKeyCode && targetKeyPressed) {
                    targetKeyPressed = NO;
                    goHotkeyUp();
                }
            }];
    }
}

// CGEvent callback for escape key tap
static CGEventRef escapeTapCallback(CGEventTapProxy proxy, CGEventType type, CGEventRef event, void *userInfo) {
    if (type == kCGEventKeyDown) {
        CGKeyCode keyCode = (CGKeyCode)CGEventGetIntegerValueField(event, kCGKeyboardEventKeycode);
        if (keyCode == 53) { // Escape
            goEscapePressed();
        }
    }
    // Re-enable tap if it gets disabled (system does this under load)
    if (type == kCGEventTapDisabledByTimeout || type == kCGEventTapDisabledByUserInput) {
        if (escapeTapPort != NULL) {
            CGEventTapEnable(escapeTapPort, true);
        }
    }
    return event; // pass event through (don't consume it)
}

void voxStartEscapeMonitor(void) {
    dispatch_async(dispatch_get_main_queue(), ^{
        if (escapeTapPort != NULL) return;

        // Listen-only tap (kCGEventTapOptionListenOnly) — doesn't require extra permissions
        // beyond what we already have for Accessibility
        escapeTapPort = CGEventTapCreate(
            kCGSessionEventTap,
            kCGHeadInsertEventTap,
            kCGEventTapOptionListenOnly,
            CGEventMaskBit(kCGEventKeyDown),
            escapeTapCallback,
            NULL
        );

        if (escapeTapPort == NULL) {
            fprintf(stderr, "vox: escape tap: failed to create (no accessibility permission?)\n");
            return;
        }

        escapeTapSource = CFMachPortCreateRunLoopSource(kCFAllocatorDefault, escapeTapPort, 0);
        CFRunLoopAddSource(CFRunLoopGetMain(), escapeTapSource, kCFRunLoopCommonModes);
        CGEventTapEnable(escapeTapPort, true);
    });
}

void voxStopEscapeMonitor(void) {
    dispatch_async(dispatch_get_main_queue(), ^{
        if (escapeTapPort != NULL) {
            CGEventTapEnable(escapeTapPort, false);
            CFRunLoopRemoveSource(CFRunLoopGetMain(), escapeTapSource, kCFRunLoopCommonModes);
            CFRelease(escapeTapSource);
            CFRelease(escapeTapPort);
            escapeTapSource = NULL;
            escapeTapPort = NULL;
        }
    });
}

// Get main screen dimensions for overlay positioning
void voxGetMainScreenInfo(int *x, int *y, int *width, int *height, int *menuBarHeight) {
    NSScreen *screen = [NSScreen mainScreen];
    if (screen == nil) {
        *x = 0; *y = 0; *width = 1920; *height = 1080; *menuBarHeight = 38;
        return;
    }
    NSRect full = [screen frame];
    NSRect visible = [screen visibleFrame];
    *x = (int)visible.origin.x;
    *y = (int)visible.origin.y;
    *width = (int)visible.size.width;
    *height = (int)visible.size.height;
    // Menu bar (+ notch) height = top of full frame minus top of visible frame
    // In macOS coords (origin bottom-left): menu bar is at the top, so
    // menuBarHeight = fullHeight - (visibleY + visibleHeight)
    int mb = (int)(full.size.height - visible.origin.y - visible.size.height);
    *menuBarHeight = mb > 0 ? mb : 25; // safety minimum
}

void voxStopMonitor(void) {
    if (flagsMonitor != nil) {
        [NSEvent removeMonitor:flagsMonitor];
        flagsMonitor = nil;
    }
    if (keyDownMonitor != nil) {
        [NSEvent removeMonitor:keyDownMonitor];
        keyDownMonitor = nil;
    }
    if (keyUpMonitor != nil) {
        [NSEvent removeMonitor:keyUpMonitor];
        keyUpMonitor = nil;
    }
    targetKeyPressed = NO;
}
