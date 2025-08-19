package main

import (
	"os"
	"path/filepath"
	"testing"
)

const (
	testInboundName  = "test-inbound"
	testOutboundName = "test-outbound"
)

func createTestConfig() string {
	return `
log_level: "info"
log_json: false
outbound:
  - name: "` + testOutboundName + `"
    description: "Test outbound configuration"
    source: "/tmp/test"
    destination: "test-bucket/path"
    sensitive: false
inbound:
  - name: "` + testInboundName + `"
    description: "Test inbound configuration"
    source: "amqp://user:pass@localhost:5672/"
    exchange: "test-exchange"
    queue: "test-queue"
    remote: "test-remote"
    destination: "/tmp/downloads"
remotes:
  - name: "test-remote"
    endpoint: "http://localhost:9000"
    accessKey: "testkey"
    secretKey: "testsecret"
`
}

func TestReadConfig(t *testing.T) {
	configContent := createTestConfig()

	// Create temporary file
	tmpDir := t.TempDir()
	configFile := filepath.Join(tmpDir, "test-config.yaml")
	err := os.WriteFile(configFile, []byte(configContent), 0600)
	if err != nil {
		t.Fatalf("Failed to create test config file: %v", err)
	}

	// Test reading the config
	err = readConfig(configFile)
	if err != nil {
		t.Fatalf("Failed to read config: %v", err)
	}

	validateConfig(t)
}

func validateConfig(t *testing.T) {
	// Verify config was parsed correctly
	if config.LogLevel != infoLevel {
		t.Errorf("Expected log level 'info', got '%s'", config.LogLevel)
	}

	if config.LogJSON {
		t.Errorf("Expected log_json false, got %v", config.LogJSON)
	}

	if len(config.Outbound) != 1 {
		t.Errorf("Expected 1 outbound config, got %d", len(config.Outbound))
	}

	validateOutboundConfig(t)
	validateInboundConfig(t)
	validateRemoteConfig(t)
}

func validateOutboundConfig(t *testing.T) {
	if len(config.Outbound) == 0 {
		t.Fatal("No outbound config found")
	}
	outbound := config.Outbound[0]
	if outbound.Name != testOutboundName {
		t.Errorf("Expected outbound name '%s', got '%s'", testOutboundName, outbound.Name)
	}

	if outbound.Sensitive {
		t.Errorf("Expected outbound sensitive false, got %v", outbound.Sensitive)
	}
}

func validateInboundConfig(t *testing.T) {
	if len(config.Inbound) == 0 {
		t.Fatal("No inbound config found")
	}
	inbound := config.Inbound[0]
	if inbound.Name != testInboundName {
		t.Errorf("Expected inbound name '%s', got '%s'", testInboundName, inbound.Name)
	}

	if inbound.Exchange != "test-exchange" {
		t.Errorf("Expected exchange 'test-exchange', got '%s'", inbound.Exchange)
	}
}

func validateRemoteConfig(t *testing.T) {
	if len(config.Remotes) == 0 {
		t.Fatal("No remote config found")
	}
	remote := config.Remotes[0]
	if remote.Name != "test-remote" {
		t.Errorf("Expected remote name 'test-remote', got '%s'", remote.Name)
	}

	if remote.Endpoint != "http://localhost:9000" {
		t.Errorf("Expected endpoint 'http://localhost:9000', got '%s'", remote.Endpoint)
	}
}

func TestReadConfigNonExistentFile(t *testing.T) {
	err := readConfig("/non/existent/file.yaml")
	if err == nil {
		t.Error("Expected error when reading non-existent file, got nil")
	}
}

func TestReadConfigInvalidYAML(t *testing.T) {
	// Create temporary file with invalid YAML
	tmpDir := t.TempDir()
	configFile := filepath.Join(tmpDir, "invalid-config.yaml")
	invalidContent := `
log_level: "info"
invalid_yaml: [
`
	err := os.WriteFile(configFile, []byte(invalidContent), 0600)
	if err != nil {
		t.Fatalf("Failed to create invalid config file: %v", err)
	}

	err = readConfig(configFile)
	if err == nil {
		t.Error("Expected error when reading invalid YAML, got nil")
	}
}

func TestConfigStructures(t *testing.T) {
	// Test Remote struct
	remote := Remote{
		Name:      "test",
		Endpoint:  "http://localhost:9000",
		AccessKey: "key",
		SecretKey: "secret",
	}

	if remote.Name != "test" {
		t.Errorf("Expected remote name 'test', got '%s'", remote.Name)
	}

	// Test Outbound struct
	outbound := Outbound{
		Name:        "test-out",
		Description: "Test description",
		Sensitive:   true,
		Source:      "/tmp/source",
		Destination: "bucket/dest",
		ProcessWith: "script.sh",
	}

	if !outbound.Sensitive {
		t.Error("Expected outbound sensitive to be true")
	}

	// Test Inbound struct
	inbound := Inbound{
		Name:        "test-in",
		Description: "Test inbound",
		Source:      "amqp://localhost",
		Exchange:    "exchange",
		Queue:       "queue",
		Remote:      "remote",
		Destination: "/tmp/dest",
	}

	if inbound.Exchange != "exchange" {
		t.Errorf("Expected exchange 'exchange', got '%s'", inbound.Exchange)
	}
}
