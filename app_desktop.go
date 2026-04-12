//go:build !nogui

package main

import (
	"context"

	"github.com/wailsapp/wails/v3/pkg/application"
)

// ServiceStartup is called by Wails v3 when the service starts.
func (a *App) ServiceStartup(ctx context.Context, options application.ServiceOptions) error {
	a.Start()
	return nil
}

// ServiceShutdown is called by Wails v3 when the service stops.
func (a *App) ServiceShutdown() error {
	a.Shutdown()
	return nil
}
