#include <Carbon/Carbon.h>
#import <Cocoa/Cocoa.h>

// Defined in Go via //export
extern void goHotkeyDown(void);
extern void goHotkeyUp(void);

static int targetKeyCode = 61; // Right Option default

static id keyDownMonitor = nil;
static id keyUpMonitor = nil;
static id flagsMonitor = nil;
static BOOL targetKeyPressed = NO;

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

