//go:build windows

package notify

import (
	"os/exec"
	"strings"
)

// Send displays a Windows balloon notification with the given title and message.
func Send(title, message string) error {
	if runes := []rune(message); len(runes) > 100 {
		message = string(runes[:97]) + "..."
	}
	script := `
Add-Type -AssemblyName System.Windows.Forms
$notify = New-Object System.Windows.Forms.NotifyIcon
$notify.Icon = [System.Drawing.SystemIcons]::Information
$notify.Visible = $true
$notify.ShowBalloonTip(3000, "` + escapePowerShell(title) + `", "` + escapePowerShell(message) + `", [System.Windows.Forms.ToolTipIcon]::Info)
Start-Sleep -Milliseconds 3100
$notify.Dispose()
`
	return exec.Command("powershell", "-NoProfile", "-Command", script).Run()
}

func escapePowerShell(s string) string {
	s = strings.ReplaceAll(s, "`", "``")
	s = strings.ReplaceAll(s, `"`, "`\"")
	s = strings.ReplaceAll(s, "$", "`$")
	s = strings.ReplaceAll(s, "\n", " ")
	return s
}
