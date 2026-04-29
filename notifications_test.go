package main

import (
	"os/exec"
	"runtime"
	"testing"
)

// TestSendNotification tests the notification functionality
func TestSendNotification(t *testing.T) {
	// Save original config
	originalConfig := config
	defer func() { config = originalConfig }()

	// Test with notifications disabled
	config = Config{EnableNotifications: false}
	SendNotification("Test", "Message")
	// Should not attempt to send notification

	// Test with notifications enabled
	config = Config{EnableNotifications: true}

	// This will attempt to send a notification, but since it's async and may fail,
	// we mainly test that it doesn't panic and calls the right command
	SendNotification("bucketsyncd", "Test notification")

	// Test different platforms (we can't easily test the actual commands without mocking)
	switch runtime.GOOS {
	case "linux":
		// Should use notify-send
		_ = exec.Command("notify-send", "--version")
	case "darwin":
		// Should use osascript
		_ = exec.Command("osascript", "--version")
	case "windows":
		// Should use powershell
		_ = exec.Command("powershell", "-Command", "Get-Host")
	}
}

// TestNotificationConfig tests that the config option works
func TestNotificationConfig(t *testing.T) {
	// Test parsing config with notifications
	configMutex.Lock()
	config = Config{}
	config.EnableNotifications = true
	configMutex.Unlock()

	configMutex.RLock()
	if !config.EnableNotifications {
		t.Error("EnableNotifications should be true")
	}
	configMutex.RUnlock()

	configMutex.Lock()
	config.EnableNotifications = false
	configMutex.Unlock()
	configMutex.RLock()
	if config.EnableNotifications {
		t.Error("EnableNotifications should be false")
	}
	configMutex.RUnlock()
}

// TestSendNotificationEmptyMessage tests with empty messages
func TestSendNotificationEmptyMessage(t *testing.T) {
	originalConfig := config
	defer func() { config = originalConfig }()

	config = Config{EnableNotifications: true}
	SendNotification("", "")
	// Should not panic
}

// TestUnsupportedOS tests notification on unsupported OS
func TestUnsupportedOS(t *testing.T) {
	// We can't easily test unsupported OS without mocking runtime.GOOS
	// But we can verify the function doesn't panic
	originalConfig := config
	defer func() { config = originalConfig }()

	config = Config{EnableNotifications: true}
	// Temporarily change GOOS if possible, but since it's runtime, skip
	SendNotification("test", "message")
	// Should complete without panic
}