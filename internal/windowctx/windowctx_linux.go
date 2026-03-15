//go:build linux

package windowctx

import (
	"encoding/json"
	"os"
	"os/exec"
	"strings"
)

// GetContext returns information about the currently focused window on Linux.
// Auto-detects the compositor and uses the appropriate method.
func GetContext() (Context, error) {
	sessionType := os.Getenv("XDG_SESSION_TYPE")
	desktop := os.Getenv("XDG_CURRENT_DESKTOP")

	if sessionType == "wayland" {
		switch {
		case strings.Contains(desktop, "sway"):
			return swayContext()
		case strings.Contains(desktop, "Hyprland"):
			return hyprlandContext()
		default:
			// GNOME or other Wayland compositors — try gdbus, fall back to xdotool
			ctx, err := gnomeContext()
			if err == nil {
				return ctx, nil
			}
			return x11Context()
		}
	}

	return x11Context()
}

func swayContext() (Context, error) {
	out, err := exec.Command("swaymsg", "-t", "get_tree").Output()
	if err != nil {
		return Context{}, err
	}

	focused := findFocused(out)
	return focused, nil
}

type swayNode struct {
	Name    string     `json:"name"`
	AppID   string     `json:"app_id"`
	Focused bool       `json:"focused"`
	Nodes   []swayNode `json:"nodes"`
	Floating []swayNode `json:"floating_nodes"`
}

func findFocused(data []byte) Context {
	var root swayNode
	if err := json.Unmarshal(data, &root); err != nil {
		return Context{}
	}
	return searchNode(root)
}

func searchNode(node swayNode) Context {
	if node.Focused && node.Name != "" {
		return Context{
			AppName:     node.AppID,
			AppID:       node.AppID,
			WindowTitle: node.Name,
		}
	}
	for _, child := range node.Nodes {
		if ctx := searchNode(child); ctx.WindowTitle != "" {
			return ctx
		}
	}
	for _, child := range node.Floating {
		if ctx := searchNode(child); ctx.WindowTitle != "" {
			return ctx
		}
	}
	return Context{}
}

func hyprlandContext() (Context, error) {
	out, err := exec.Command("hyprctl", "activewindow", "-j").Output()
	if err != nil {
		return Context{}, err
	}

	var win struct {
		Class string `json:"class"`
		Title string `json:"title"`
	}
	if err := json.Unmarshal(out, &win); err != nil {
		return Context{}, err
	}

	return Context{
		AppName:     win.Class,
		AppID:       win.Class,
		WindowTitle: win.Title,
	}, nil
}

func gnomeContext() (Context, error) {
	out, err := exec.Command("gdbus", "call",
		"--session",
		"--dest", "org.gnome.Shell",
		"--object-path", "/org/gnome/Shell/Extensions/WindowInfo",
		"--method", "org.gnome.Shell.Extensions.WindowInfo.GetActiveWindow",
	).Output()
	if err != nil {
		return Context{}, err
	}

	// gdbus output is a GVariant string, parse basic info
	s := string(out)
	return Context{
		WindowTitle: strings.TrimSpace(s),
	}, nil
}

func x11Context() (Context, error) {
	var ctx Context

	// Get window ID and title
	out, err := exec.Command("xdotool", "getactivewindow", "getwindowname").Output()
	if err == nil {
		ctx.WindowTitle = strings.TrimSpace(string(out))
	}

	// Get WM_CLASS for app identification
	idOut, err := exec.Command("xdotool", "getactivewindow").Output()
	if err == nil {
		winID := strings.TrimSpace(string(idOut))
		propOut, err := exec.Command("xprop", "-id", winID, "WM_CLASS").Output()
		if err == nil {
			// Format: WM_CLASS(STRING) = "instance", "class"
			s := string(propOut)
			if idx := strings.Index(s, "= "); idx >= 0 {
				parts := strings.Split(s[idx+2:], ", ")
				if len(parts) >= 2 {
					ctx.AppName = strings.Trim(parts[1], `"` + "\n")
					ctx.AppID = strings.Trim(parts[0], `"` + "\n")
				}
			}
		}
	}

	return ctx, nil
}
