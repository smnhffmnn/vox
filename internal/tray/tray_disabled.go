//go:build !tray && !darwin

package tray

import "sync"

// noopTray is a no-op implementation when built without the tray tag (Linux).
type noopTray struct {
	doneCh   chan struct{}
	doneOnce sync.Once
}

// New returns a no-op tray when built without the tray build tag.
func New() Tray {
	return &noopTray{doneCh: make(chan struct{})}
}

func (t *noopTray) Run(onReady func(), onQuit func()) {
	if onReady != nil {
		go onReady()
	}
	<-t.doneCh
	if onQuit != nil {
		onQuit()
	}
}

func (t *noopTray) Quit() {
	t.doneOnce.Do(func() { close(t.doneCh) })
}

func (t *noopTray) SetState(state State) {}
func (t *noopTray) SetStatus(text string) {}
