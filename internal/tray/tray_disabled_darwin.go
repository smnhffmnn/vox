//go:build !tray && darwin

package tray

/*
#cgo CFLAGS: -x objective-c
#cgo LDFLAGS: -framework Cocoa

#import <Cocoa/Cocoa.h>

static inline void voxTrayRunNSApp(void) {
    [NSApplication sharedApplication];
    [NSApp run];
}

static inline void voxTrayStopNSApp(void) {
    dispatch_async(dispatch_get_main_queue(), ^{
        [NSApp stop:nil];
        NSEvent *event = [NSEvent otherEventWithType:NSEventTypeApplicationDefined
                                            location:NSMakePoint(0, 0)
                                       modifierFlags:0
                                           timestamp:0
                                        windowNumber:0
                                             context:nil
                                             subtype:0
                                               data1:0
                                               data2:0];
        [NSApp postEvent:event atStart:YES];
    });
}
*/
import "C"

import "sync"

// noopTray is a no-op tray for macOS without the tray tag.
// It runs the NSApp event loop so hotkey monitors work.
type noopTray struct {
	quitOnce sync.Once
}

// New returns a no-op tray that manages the macOS NSApp event loop.
func New() Tray {
	return &noopTray{}
}

func (t *noopTray) Run(onReady func(), onQuit func()) {
	if onReady != nil {
		go onReady()
	}
	// macOS needs an NSApp event loop for hotkey monitors to receive events
	C.voxTrayRunNSApp()
	if onQuit != nil {
		onQuit()
	}
}

func (t *noopTray) Quit() {
	t.quitOnce.Do(func() { C.voxTrayStopNSApp() })
}

func (t *noopTray) SetState(state State)      {}
func (t *noopTray) SetStatus(text string)      {}
func (t *noopTray) SetSettingsPort(port int)   {}
