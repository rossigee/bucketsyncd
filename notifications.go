package main

import (
	"fmt"
	"os/exec"
	"runtime"
	"strings"

	log "github.com/sirupsen/logrus"
)

// SendNotification sends a desktop notification
func SendNotification(title, message string) {
	configMutex.RLock()
	enableNotifications := config.EnableNotifications
	configMutex.RUnlock()
	if !enableNotifications || message == "" {
		return
	}

	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "linux":
		cmd = exec.Command("notify-send", title, message) // #nosec G204 - args passed as separate parameters, no shell interpretation
	case "darwin":
		// Escape double quotes so message/title cannot break out of the AppleScript string context.
		safeMessage := strings.ReplaceAll(message, `"`, `\"`)
		safeTitle := strings.ReplaceAll(title, `"`, `\"`)
		script := fmt.Sprintf(`display notification "%s" with title "%s"`, safeMessage, safeTitle)
		cmd = exec.Command("osascript", "-e", script) // #nosec G204 - This is intentional use of exec.Command for notifications
	case "windows":
		// Escape single quotes for PowerShell single-quoted string literals ('' is the escape sequence).
		safeMessage := strings.ReplaceAll(message, `'`, `''`)
		safeTitle := strings.ReplaceAll(title, `'`, `''`)
		script := fmt.Sprintf(`Add-Type -AssemblyName System.Windows.Forms; [System.Windows.Forms.MessageBox]::Show('%s', '%s')`, safeMessage, safeTitle)
		cmd = exec.Command("powershell", "-Command", script) // #nosec G204 - This is intentional use of exec.Command for notifications
	default:
		log.Warn("Notifications not supported on this platform")
		return
	}

	// Run the command in background, don't wait
	go func() {
		if err := cmd.Run(); err != nil {
			log.WithFields(log.Fields{
				"os":    runtime.GOOS,
				"error": err,
			}).Debug("Failed to send notification")
		}
	}()
}