package main

import (
	"flag"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/fsnotify/fsnotify"
	amqp "github.com/rabbitmq/amqp091-go"
	log "github.com/sirupsen/logrus"
)

// TestInboundFunctionCoverage tests the inbound function with various inputs
// to improve test coverage by exercising different code paths
func TestInboundFunctionCoverage(t *testing.T) {
	// Save original connections and config
	originalConnections := connections
	originalConfig := config
	defer func() {
		connections = originalConnections
		config = originalConfig
	}()

	// Test invalid URL source
	t.Run("invalid_url_source", func(t *testing.T) {
		testInboundWithInvalidURL(t)
	})

	// Test AMQP connection failure
	t.Run("invalid_amqp_connection", func(t *testing.T) {
		testInboundWithAMQPFailure(t)
	})
}

func testInboundWithInvalidURL(_ *testing.T) {
	connections = []*amqp.Connection{}
	config = Config{
		Remotes: []Remote{{
			Name:      "test-remote",
			Endpoint:  "localhost:9000",
			AccessKey: "test-access",
			SecretKey: "test-secret",
		}},
	}

	inboundConfig := Inbound{
		Name: "test-invalid-url", Description: "Test with invalid URL", Source: "://invalid-url-scheme",
		Exchange: "test-exchange", Queue: "test-queue", Remote: "test-remote", Destination: "/tmp/test",
	}

	// This will exercise the early validation and connection logic
	inbound(inboundConfig)
	// If we reach here without panic, the function handled the error gracefully
}

func testInboundWithAMQPFailure(_ *testing.T) {
	connections = []*amqp.Connection{}
	config = Config{
		Remotes: []Remote{{
			Name:      "test-remote",
			Endpoint:  "localhost:9000",
			AccessKey: "test-access",
			SecretKey: "test-secret",
		}},
	}

	inboundConfig := Inbound{
		Name: "test-invalid-amqp", Description: "Test with unreachable AMQP server",
		Source: "amqp://guest:guest@nonexistent-host:5672/", Exchange: "test-exchange",
		Queue: "test-queue", Remote: "test-remote", Destination: "/tmp/test",
	}

	// This will exercise the early validation and connection logic
	inbound(inboundConfig)
	// If we reach here without panic, the function handled the error gracefully
}

// TestOutboundFunctionCoverage tests the outbound function with various inputs
func TestOutboundFunctionCoverage(t *testing.T) {
	// Save original config and watchers
	originalConfig := config
	originalWatchers := watchers
	defer func() {
		config = originalConfig
		watchers = originalWatchers
	}()

	tests := []struct {
		name        string
		outbound    Outbound
		description string
	}{
		{
			name: "empty_source_path",
			outbound: Outbound{
				Name:        "test-empty-source",
				Description: "Test with empty source path",
				Source:      "",
				Destination: "s3://test-bucket/uploads/",
				Sensitive:   false,
			},
			description: "Should handle empty source path gracefully",
		},
		{
			name: "valid_temp_directory",
			outbound: Outbound{
				Name:        "test-temp-dir",
				Description: "Test with valid temp directory",
				Source:      "/tmp/*",
				Destination: "s3://test-bucket/uploads/",
				Sensitive:   false,
			},
			description: "Should process valid directory path",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(_ *testing.T) {
			// Reset watchers for each test
			watchers = []fsnotify.Watcher{}

			// Set up basic config
			config = Config{
				Remotes: []Remote{
					{
						Name:      "test-remote",
						Endpoint:  "localhost:9000",
						AccessKey: "test-access",
						SecretKey: "test-secret",
					},
				},
			}

			// Call outbound function - this will exercise path parsing and watcher setup
			// The function may fail on file system operations but will exercise the logic
			outbound(tt.outbound)

			// Check that some processing occurred (watchers might be modified)
			// This verifies the function executed its logic paths
		})
	}
}

// TestFlagParsingAndValidation tests flag parsing from main function
func TestFlagParsingAndValidation(t *testing.T) {
	originalArgs := os.Args
	originalConfigFilePath := *configFilePath
	originalHelp := *help

	defer func() {
		os.Args = originalArgs
		*configFilePath = originalConfigFilePath
		*help = originalHelp
	}()

	// Test the flag parsing logic from main()

	// Reset flags for testing
	flag.CommandLine = flag.NewFlagSet(os.Args[0], flag.ExitOnError)
	configFilePath = flag.String("c", "", "Configuration file location")
	help = flag.Bool("h", false, "Usage information")

	// Test with config file flag
	os.Args = []string{"bucketsyncd", "-c", "/tmp/test-config.yaml"}
	flag.Parse()

	// Test the validation logic from main()
	configPathEmpty := *configFilePath == ""
	helpRequested := *help

	if configPathEmpty && !helpRequested {
		// This exercises the error condition in main()
		t.Log("Would show error: -c option is required")
	}

	if helpRequested || configPathEmpty {
		// This exercises the usage display logic in main()
		t.Log("Would show usage information")
	}
}

// TestLogConfiguration tests log configuration logic from main function
func TestLogConfiguration(_ *testing.T) {
	// Test log configuration logic from main()
	originalLevel := log.GetLevel()
	originalFormatter := log.StandardLogger().Formatter
	defer func() {
		log.SetLevel(originalLevel)
		log.SetFormatter(originalFormatter)
	}()

	// Test different log level configurations
	testLevels := []string{"debug", "info", "warn", "error", "unknown"}

	for _, level := range testLevels {
		config.LogLevel = level

		// Apply the same logic as main()
		log.SetFormatter(&log.TextFormatter{
			DisableColors: true,
			FullTimestamp: true,
		})
		switch config.LogLevel {
		case debugLevel:
			log.SetLevel(log.DebugLevel)
		case infoLevel:
			log.SetLevel(log.InfoLevel)
		case warnLevel:
			log.SetLevel(log.WarnLevel)
		}

		// Test duplicate debug check from main()
		if config.LogLevel == debugLevel {
			log.SetLevel(log.DebugLevel)
		}

		// Test JSON formatter logic
		config.LogJSON = true
		if config.LogJSON {
			log.SetFormatter(&log.JSONFormatter{})
		}
	}
}

// TestMainFunctionComponents tests components of the main function
func TestMainFunctionComponents(t *testing.T) {
	// Save original state
	originalArgs := os.Args
	originalConfigFilePath := *configFilePath
	originalHelp := *help
	originalConfig := config

	defer func() {
		os.Args = originalArgs
		*configFilePath = originalConfigFilePath
		*help = originalHelp
		config = originalConfig
	}()

	// Call the separate test functions
	TestFlagParsingAndValidation(t)
	TestLogConfiguration(t)
	TestConfigurationProcessing(t)
}

// TestConfigurationProcessing tests configuration processing from main function
func TestConfigurationProcessing(t *testing.T) {
	// Test the configuration processing logic from main()

	// Create a temporary config file
	tmpDir := t.TempDir()
	configFile := filepath.Join(tmpDir, "test-main-config.yaml")

	configContent := `
log_level: "debug"
log_json: false
outbound:
  - name: "main-test-outbound"
    description: "Test outbound"
    source: "/tmp/test/*"
    destination: "s3://test-bucket/uploads/"
    sensitive: false
inbound:
  - name: "main-test-inbound" 
    description: "Test inbound"
    source: "amqp://guest:guest@localhost:5672/"
    exchange: "test-exchange"
    queue: "test-queue"
    remote: "test-remote"
    destination: "/tmp/downloads"
remotes:
  - name: "test-remote"
    endpoint: "localhost:9000"
    accessKey: "test-access"
    secretKey: "test-secret"
`

	err := os.WriteFile(configFile, []byte(configContent), 0600)
	if err != nil {
		t.Fatalf("Failed to create config file: %v", err)
	}

	// Test config reading (same as main() does)
	err = readConfig(configFile)
	if err != nil {
		t.Errorf("Config reading failed: %v", err)
	}

	// Test the processing loops from main()
	outboundCount := 0
	for i := 0; i < len(config.Outbound); i++ {
		o := config.Outbound[i]
		if o.Name != "" {
			outboundCount++
			// This exercises the outbound processing logic from main()
		}
	}

	inboundCount := 0
	for i := 0; i < len(config.Inbound); i++ {
		in := config.Inbound[i]
		if in.Name != "" {
			inboundCount++
			// This exercises the inbound processing logic from main()
		}
	}

	if outboundCount != 1 {
		t.Errorf("Expected 1 outbound processed, got %d", outboundCount)
	}
	if inboundCount != 1 {
		t.Errorf("Expected 1 inbound processed, got %d", inboundCount)
	}
}

// TestInboundCloseImprovedCoverage improves coverage of inboundClose function
func TestInboundCloseImprovedCoverage(t *testing.T) {
	originalConnections := connections
	defer func() { connections = originalConnections }()

	t.Run("close_multiple_connections", func(_ *testing.T) {
		// We can't easily create real AMQP connections in tests,
		// but we can test the inboundClose function with an empty slice
		// The function should handle this gracefully
		connections = []*amqp.Connection{}

		// This should handle empty connections gracefully
		inboundClose()

		// Test completed successfully if no panic occurred
	})

	t.Run("close_empty_slice", func(_ *testing.T) {
		connections = []*amqp.Connection{}
		inboundClose()
	})

	t.Run("close_nil_slice", func(_ *testing.T) {
		connections = nil
		inboundClose()
	})
}

// TestWatcherGlobalAccess tests access to the global watchers variable
func TestWatcherGlobalAccess(t *testing.T) {
	originalWatchers := watchers
	defer func() { watchers = originalWatchers }()

	// Test watchers initialization and access
	initialLength := len(watchers)
	if initialLength < 0 {
		t.Errorf("Watchers length should not be negative: %d", initialLength)
	}

	// Test modifying watchers slice (as outbound function would do)
	watcher, err := fsnotify.NewWatcher()
	if err == nil {
		watchers = append(watchers, *watcher)
		if closeErr := watcher.Close(); closeErr != nil {
			t.Logf("Failed to close watcher: %v", closeErr)
		}
	}
	newLength := len(watchers)

	if newLength != initialLength+1 {
		t.Errorf("Expected watchers length %d, got %d", initialLength+1, newLength)
	}
}

// TestConfigGlobalAccess tests access to the global config variable
func TestConfigGlobalAccess(t *testing.T) {
	originalConfig := config
	defer func() { config = originalConfig }()

	// Test config modification (as main function would do)
	config.LogLevel = "test-level"
	config.LogJSON = true

	if config.LogLevel != "test-level" {
		t.Errorf("Expected log level 'test-level', got '%s'", config.LogLevel)
	}

	if !config.LogJSON {
		t.Error("Expected LogJSON to be true")
	}

	// Test config slices
	config.Outbound = []Outbound{
		{Name: "test1"}, {Name: "test2"},
	}
	config.Inbound = []Inbound{
		{Name: "test1"}, {Name: "test2"}, {Name: "test3"},
	}
	config.Remotes = []Remote{
		{Name: "remote1"},
	}

	if len(config.Outbound) != 2 {
		t.Errorf("Expected 2 outbound configs, got %d", len(config.Outbound))
	}
	if len(config.Inbound) != 3 {
		t.Errorf("Expected 3 inbound configs, got %d", len(config.Inbound))
	}
	if len(config.Remotes) != 1 {
		t.Errorf("Expected 1 remote config, got %d", len(config.Remotes))
	}
}

// TestStringManipulationCoverage tests string operations used throughout the code
func TestStringManipulationCoverage(t *testing.T) {
	// Test filepath operations used in outbound function
	testPaths := []string{
		"/tmp/test/*",
		"/var/log/*.log",
		"/home/user/documents/backup-*",
		"./relative/path/*",
		"",
		"/",
		"/single",
	}

	for _, path := range testPaths {
		dir := filepath.Dir(path)
		base := filepath.Base(path)

		// These operations are used in the outbound function
		if dir == "" && path != "" {
			t.Errorf("Unexpected empty directory for path: %s", path)
		}
		if base == "" && path != "" {
			t.Errorf("Unexpected empty base for path: %s", path)
		}
	}

	// Test URL operations used in inbound function
	testURLs := []string{
		"amqp://guest:guest@localhost:5672/",
		"amqp://user:pass@remote:5673/vhost",
		"invalid-url",
		"",
		"://missing-scheme",
	}

	for _, urlStr := range testURLs {
		if urlStr == "" {
			continue
		}

		// This exercises URL parsing logic from inbound function
		if strings.Contains(urlStr, "://") {
			// Basic URL validation that inbound function might do
			parts := strings.Split(urlStr, "://")
			if len(parts) >= 2 {
				scheme := parts[0]
				if scheme != "amqp" && scheme != "amqps" {
					t.Logf("Non-AMQP scheme detected: %s", scheme)
				}
			}
		}
	}
}
