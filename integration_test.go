package main

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	amqp "github.com/rabbitmq/amqp091-go"
)

func TestMainApplicationFlow(_ *testing.T) {
	// Test main application initialization without running full main()

	// Save original args
	originalArgs := os.Args
	defer func() { os.Args = originalArgs }()

	// Test help flag
	os.Args = []string{"bucketsyncd", "-h"}

	// We can't easily test main() directly as it runs indefinitely,
	// but we can test the flag parsing logic
	// This would require refactoring main() to be more testable
}

// createIntegrationTestConfig creates integration test configuration file
func createIntegrationTestConfig(t *testing.T) string {
	tmpDir := t.TempDir()
	configFile := filepath.Join(tmpDir, "integration-test-config.yaml")

	configContent := `
log_level: "debug"
log_json: false
outbound:
  - name: "integration-outbound"
    description: "Integration test outbound"
    source: "/tmp/integration-test/*"
    destination: "s3://test-bucket/integration/"
    sensitive: false
inbound:
  - name: "integration-inbound"
    description: "Integration test inbound"
    source: "amqp://guest:guest@localhost:5672/"
    exchange: "integration-exchange"
    queue: "integration-queue"
    remote: "integration-remote"
    destination: "/tmp/integration-downloads"
remotes:
  - name: "integration-remote"
    endpoint: "localhost:9000"
    accessKey: "integration-access"
    secretKey: "integration-secret"
`

	err := os.WriteFile(configFile, []byte(configContent), 0600)
	if err != nil {
		t.Fatalf("Failed to create integration test config: %v", err)
	}

	return configFile
}

// verifyIntegrationConfigCounts verifies configuration counts
func verifyIntegrationConfigCounts(t *testing.T) {
	if config.LogLevel != debugLevel {
		t.Errorf("Expected log level 'debug', got '%s'", config.LogLevel)
	}

	if len(config.Outbound) != 1 {
		t.Errorf("Expected 1 outbound config, got %d", len(config.Outbound))
	}

	if len(config.Inbound) != 1 {
		t.Errorf("Expected 1 inbound config, got %d", len(config.Inbound))
	}

	if len(config.Remotes) != 1 {
		t.Errorf("Expected 1 remote config, got %d", len(config.Remotes))
	}
}

// verifyIntegrationConfigDetails verifies specific configuration details
func verifyIntegrationConfigDetails(t *testing.T) {
	// Test outbound configuration
	outbound := config.Outbound[0]
	if outbound.Name != "integration-outbound" {
		t.Errorf("Expected outbound name 'integration-outbound', got '%s'", outbound.Name)
	}

	if outbound.Source != "/tmp/integration-test/*" {
		t.Errorf("Expected source '/tmp/integration-test/*', got '%s'", outbound.Source)
	}

	// Test inbound configuration
	inbound := config.Inbound[0]
	if inbound.Name != "integration-inbound" {
		t.Errorf("Expected inbound name 'integration-inbound', got '%s'", inbound.Name)
	}

	if inbound.Remote != "integration-remote" {
		t.Errorf("Expected remote 'integration-remote', got '%s'", inbound.Remote)
	}

	// Test remote configuration
	remote := config.Remotes[0]
	if remote.Name != "integration-remote" {
		t.Errorf("Expected remote name 'integration-remote', got '%s'", remote.Name)
	}

	if remote.Endpoint != "localhost:9000" {
		t.Errorf("Expected endpoint 'localhost:9000', got '%s'", remote.Endpoint)
	}
}

func TestConfigurationFlow(t *testing.T) {
	// Create a temporary configuration file
	configFile := createIntegrationTestConfig(t)

	// Test configuration reading
	err := readConfig(configFile)
	if err != nil {
		t.Fatalf("Failed to read integration config: %v", err)
	}

	// Verify configuration was loaded correctly
	verifyIntegrationConfigCounts(t)
	verifyIntegrationConfigDetails(t)
}

func TestErrorHandling(t *testing.T) {
	// Test various error conditions

	// Test reading non-existent config file
	err := readConfig("/non/existent/config.yaml")
	if err == nil {
		t.Error("Expected error reading non-existent config file")
	}

	// Test malformed YAML
	tmpDir := t.TempDir()
	malformedConfig := filepath.Join(tmpDir, "malformed.yaml")
	malformedContent := `
log_level: "info"
invalid_yaml: [
`
	err = os.WriteFile(malformedConfig, []byte(malformedContent), 0600)
	if err != nil {
		t.Fatalf("Failed to create malformed config: %v", err)
	}

	err = readConfig(malformedConfig)
	if err == nil {
		t.Error("Expected error reading malformed YAML config")
	}
}

// createMultiConfig creates configuration file with multiple sections
func createMultiConfig(t *testing.T) string {
	tmpDir := t.TempDir()
	configFile := filepath.Join(tmpDir, "multi-config.yaml")

	configContent := `
log_level: "info"
log_json: true
outbound:
  - name: "outbound-1"
    description: "First outbound"
    source: "/tmp/source1/*"
    destination: "s3://bucket1/path1/"
    sensitive: false
  - name: "outbound-2"
    description: "Second outbound"
    source: "/tmp/source2/*.log"
    destination: "s3://bucket2/logs/"
    sensitive: true
    process_with: "/usr/bin/encrypt"
inbound:
  - name: "inbound-1"
    description: "First inbound"
    source: "amqp://user1:pass1@host1:5672/"
    exchange: "exchange1"
    queue: "queue1"
    remote: "remote1"
    destination: "/tmp/dest1"
  - name: "inbound-2"
    description: "Second inbound"
    source: "amqp://user2:pass2@host2:5672/"
    exchange: "exchange2"
    queue: "queue2"
    remote: "remote2"
    destination: "/tmp/dest2"
remotes:
  - name: "remote1"
    endpoint: "s3.amazonaws.com"
    accessKey: "aws-key"
    secretKey: "aws-secret"
  - name: "remote2"
    endpoint: "localhost:9000"
    accessKey: "local-key"
    secretKey: "local-secret"
`

	err := os.WriteFile(configFile, []byte(configContent), 0600)
	if err != nil {
		t.Fatalf("Failed to create multi-config file: %v", err)
	}

	return configFile
}

// verifyMultiConfigCounts verifies multiple configuration counts
func verifyMultiConfigCounts(t *testing.T) {
	if len(config.Outbound) != 2 {
		t.Errorf("Expected 2 outbound configs, got %d", len(config.Outbound))
	}

	if len(config.Inbound) != 2 {
		t.Errorf("Expected 2 inbound configs, got %d", len(config.Inbound))
	}

	if len(config.Remotes) != 2 {
		t.Errorf("Expected 2 remote configs, got %d", len(config.Remotes))
	}
}

// verifyMultiConfigSpecifics verifies specific configuration details
func verifyMultiConfigSpecifics(t *testing.T) {
	// Test specific configurations
	if config.LogJSON != true {
		t.Error("Expected log_json to be true")
	}

	// Test sensitive outbound config
	sensitiveOutbound := config.Outbound[1]
	if !sensitiveOutbound.Sensitive {
		t.Error("Expected second outbound to be sensitive")
	}

	if sensitiveOutbound.ProcessWith != "/usr/bin/encrypt" {
		t.Errorf("Expected ProcessWith '/usr/bin/encrypt', got '%s'", sensitiveOutbound.ProcessWith)
	}
}

func TestMultipleConfigurations(t *testing.T) {
	// Test configuration with multiple outbound and inbound configs
	configFile := createMultiConfig(t)

	// Test configuration reading
	err := readConfig(configFile)
	if err != nil {
		t.Fatalf("Failed to read multi-config: %v", err)
	}

	// Verify multiple configurations
	verifyMultiConfigCounts(t)
	verifyMultiConfigSpecifics(t)
}

func TestConcurrentSafety(t *testing.T) {
	// Test that global variables are handled safely
	// This is a basic test - real concurrent testing would require more setup

	originalConfig := config
	defer func() { config = originalConfig }()

	// Test that connections slice can be safely accessed
	originalConnections := connections
	connections = make([]*amqp.Connection, 0)

	// Simulate concurrent access
	done := make(chan bool, 2)

	go func() {
		connections = append(connections, nil)
		done <- true
	}()

	go func() {
		_ = len(connections)
		done <- true
	}()

	// Wait for both goroutines
	timeout := time.After(1 * time.Second)
	for i := 0; i < 2; i++ {
		select {
		case <-done:
			// Success
		case <-timeout:
			t.Error("Timeout waiting for concurrent operations")
			return
		}
	}

	connections = originalConnections
}

func TestConfigDefaults(t *testing.T) {
	// Test configuration with minimal settings
	tmpDir := t.TempDir()
	configFile := filepath.Join(tmpDir, "minimal-config.yaml")

	configContent := `
outbound: []
inbound: []
remotes: []
`

	err := os.WriteFile(configFile, []byte(configContent), 0600)
	if err != nil {
		t.Fatalf("Failed to create minimal config: %v", err)
	}

	err = readConfig(configFile)
	if err != nil {
		t.Fatalf("Failed to read minimal config: %v", err)
	}

	// Check defaults (values may be retained from previous tests)
	// Note: config is a global variable that may retain values from other tests

	if len(config.Outbound) != 0 {
		t.Errorf("Expected 0 outbound configs, got %d", len(config.Outbound))
	}

	if len(config.Inbound) != 0 {
		t.Errorf("Expected 0 inbound configs, got %d", len(config.Inbound))
	}

	if len(config.Remotes) != 0 {
		t.Errorf("Expected 0 remote configs, got %d", len(config.Remotes))
	}
}

// generateLargeConfigContent generates large configuration content
func generateLargeConfigContent() string {
	configContent := `
log_level: "info"
log_json: false
outbound:
`

	// Add multiple outbound configurations
	for i := 0; i < 10; i++ {
		configContent += `  - name: "outbound-` + string(rune('0'+i)) + `"
    description: "Outbound ` + string(rune('0'+i)) + `"
    source: "/tmp/source` + string(rune('0'+i)) + `/*"
    destination: "s3://bucket` + string(rune('0'+i)) + `/path/"
    sensitive: false
`
	}

	configContent += `inbound:
`

	// Add multiple inbound configurations
	for i := 0; i < 5; i++ {
		configContent += `  - name: "inbound-` + string(rune('0'+i)) + `"
    description: "Inbound ` + string(rune('0'+i)) + `"
    source: "amqp://user:pass@host` + string(rune('0'+i)) + `:5672/"
    exchange: "exchange` + string(rune('0'+i)) + `"
    queue: "queue` + string(rune('0'+i)) + `"
    remote: "remote` + string(rune('0'+i)) + `"
    destination: "/tmp/dest` + string(rune('0'+i)) + `"
`
	}

	configContent += `remotes:
`

	// Add multiple remote configurations
	for i := 0; i < 5; i++ {
		configContent += `  - name: "remote` + string(rune('0'+i)) + `"
    endpoint: "host` + string(rune('0'+i)) + `:9000"
    accessKey: "key` + string(rune('0'+i)) + `"
    secretKey: "secret` + string(rune('0'+i)) + `"
`
	}

	return configContent
}

// createLargeConfigFile creates large configuration file
func createLargeConfigFile(t *testing.T) string {
	tmpDir := t.TempDir()
	configFile := filepath.Join(tmpDir, "large-config.yaml")

	configContent := generateLargeConfigContent()

	err := os.WriteFile(configFile, []byte(configContent), 0600)
	if err != nil {
		t.Fatalf("Failed to create large config: %v", err)
	}

	return configFile
}

// verifyLargeConfigCounts verifies large configuration counts
func verifyLargeConfigCounts(t *testing.T) {
	// Check that all items were loaded
	if len(config.Outbound) != 10 {
		t.Errorf("Expected 10 outbound configs, got %d", len(config.Outbound))
	}

	if len(config.Inbound) != 5 {
		t.Errorf("Expected 5 inbound configs, got %d", len(config.Inbound))
	}

	if len(config.Remotes) != 5 {
		t.Errorf("Expected 5 remote configs, got %d", len(config.Remotes))
	}
}

func TestLargeConfiguration(t *testing.T) {
	// Test configuration with many items to check performance
	configFile := createLargeConfigFile(t)

	start := time.Now()
	err := readConfig(configFile)
	duration := time.Since(start)

	if err != nil {
		t.Fatalf("Failed to read large config: %v", err)
	}

	verifyLargeConfigCounts(t)

	// Performance check - should be fast for reasonable config sizes
	if duration > 100*time.Millisecond {
		t.Errorf("Config reading took too long: %v", duration)
	}
}
