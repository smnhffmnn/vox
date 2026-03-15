//go:build !tray

package tray

// noopTray is a no-op implementation when built without the tray tag.
type noopTray struct{}

// New returns a no-op tray when built without the tray build tag.
func New() Tray {
	return &noopTray{}
}

func (t *noopTray) Run(onReady func(), onQuit func()) {
	// No tray available — just call onReady immediately
	if onReady != nil {
		onReady()
	}
	// Block forever (caller should handle shutdown via other means)
	select {}
}

func (t *noopTray) SetState(state State) {}
func (t *noopTray) SetStatus(text string) {}
