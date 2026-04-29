package main

import (
	"fmt"
	"os/exec"
	"runtime"

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
		cmd = exec.Command("notify-send", title, message) // #nosec G204 - This is intentional use of exec.Command for notifications
	case "darwin":
		// Use osascript for macOS notifications
		script := fmt.Sprintf(`display notification "%s" with title "%s"`, message, title)
		cmd = exec.Command("osascript", "-e", script) // #nosec G204 - This is intentional use of exec.Command for notifications
	case "windows":
		// Use PowerShell for Windows notifications
		script := fmt.Sprintf(`Add-Type -AssemblyName System.Windows.Forms; [System.Windows.Forms.MessageBox]::Show('%s', '%s')`, message, title)
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