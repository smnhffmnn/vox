//go:build windows

package windowctx

import (
	"os/exec"
	"strings"
)

// GetContext returns information about the currently focused window on Windows.
func GetContext() (Context, error) {
	script := `
Add-Type @"
using System;
using System.Runtime.InteropServices;
using System.Text;
public class WinAPI {
    [DllImport("user32.dll")] public static extern IntPtr GetForegroundWindow();
    [DllImport("user32.dll")] public static extern int GetWindowText(IntPtr hWnd, StringBuilder text, int count);
    [DllImport("user32.dll")] public static extern uint GetWindowThreadProcessId(IntPtr hWnd, out uint processId);
}
"@
$hwnd = [WinAPI]::GetForegroundWindow()
$sb = New-Object System.Text.StringBuilder 256
[void][WinAPI]::GetWindowText($hwnd, $sb, 256)
$pid = 0
[void][WinAPI]::GetWindowThreadProcessId($hwnd, [ref]$pid)
$proc = Get-Process -Id $pid -ErrorAction SilentlyContinue
Write-Output "$($proc.ProcessName)|$($sb.ToString())"
`
	out, err := exec.Command("powershell", "-NoProfile", "-Command", script).Output()
	if err != nil {
		return Context{}, err
	}

	parts := strings.SplitN(strings.TrimSpace(string(out)), "|", 2)
	var ctx Context
	if len(parts) >= 1 {
		ctx.AppName = parts[0]
		ctx.AppID = parts[0]
	}
	if len(parts) >= 2 {
		ctx.WindowTitle = parts[1]
	}
	return ctx, nil
}
