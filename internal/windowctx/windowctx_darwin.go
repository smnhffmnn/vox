//go:build darwin

package windowctx

import (
	"os/exec"
	"strings"
)

// GetContext returns information about the currently focused window on macOS.
func GetContext() (Context, error) {
	var ctx Context

	// Get app name and window title
	out, err := exec.Command("osascript", "-e",
		`tell application "System Events" to get {name, title of front window} of first application process whose frontmost is true`,
	).Output()
	if err == nil {
		parts := strings.SplitN(strings.TrimSpace(string(out)), ", ", 2)
		if len(parts) >= 1 {
			ctx.AppName = strings.TrimSpace(parts[0])
		}
		if len(parts) >= 2 {
			ctx.WindowTitle = strings.TrimSpace(parts[1])
		}
	}

	// Get bundle ID if we have an app name
	if ctx.AppName != "" {
		out, err := exec.Command("osascript", "-e",
			`id of application "`+ctx.AppName+`"`,
		).Output()
		if err == nil {
			ctx.AppID = strings.TrimSpace(string(out))
		}
	}

	return ctx, nil
}
