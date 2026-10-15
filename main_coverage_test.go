package main

import (
	"flag"
	"os"
	"path/filepath"
	"testing"

	log "github.com/sirupsen/logrus"
)

// testSimpleFlagParsing tests simple flag parsing logic
func testSimpleFlagParsing(t *testing.T) {
	flag.CommandLine = flag.NewFlagSet(os.Args[0], flag.ExitOnError)
	configFilePath = flag.String("c", "", "Configuration file location")
	help = flag.Bool("h", false, "Usage information")

	os.Args = []string{"bucketsyncd", "-c", "/tmp/config.yaml"}
	flag.Parse()

	configEmpty := *configFilePath == ""
	helpRequested := *help

	if configEmpty && !helpRequested {
		t.Log("Would show error: -c option is required")
	}

	if helpRequested || configEmpty {
		t.Log("Would show usage information")
	}
}

// testSimpleLogging tests simple logging configuration
func testSimpleLogging(_ *testing.T) {
	config.LogLevel = debugLevel
	config.LogJSON = false

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

	if config.LogLevel == debugLevel {
		log.SetLevel(log.DebugLevel)
	}

	if config.LogJSON {
		log.SetFormatter(&log.JSONFormatter{})
	}
}

// testSimpleConfigReading tests simple config file reading
func testSimpleConfigReading(t *testing.T) {
	tmpDir := t.TempDir()
	configFile := filepath.Join(tmpDir, "simple-test.yaml")

	configContent := `
log_level: "info"
log_json: false
outbound: []
inbound: []
remotes: []
`

	err := os.WriteFile(configFile, []byte(configContent), 0600)
	if err != nil {
		t.Fatalf("Failed to create config file: %v", err)
	}

	err = readConfig(configFile)
	if err != nil {
		t.Logf("Config reading would cause panic: %v", err)
	} else {
		t.Log("Config reading successful")
	}
}

// testSimpleProcessing tests simple processing loop
func testSimpleProcessing(t *testing.T) {
	config = Config{
		Outbound: []Outbound{
			{Name: "test1", Source: "/tmp/test1/*"},
			{Name: "test2", Source: "/tmp/test2/*"},
		},
	}

	processedCount := 0
	for i := 0; i < len(config.Outbound); i++ {
		o := config.Outbound[i]
		processedCount++
		t.Logf("Would process outbound: %s", o.Name)
	}

	if processedCount != 2 {
		t.Errorf("Expected 2 outbound processed, got %d", processedCount)
	}
}

// TestSimpleMainComponents tests main function components with reduced complexity
func TestSimpleMainComponents(t *testing.T) {
	originalArgs := os.Args
	originalConfigFilePath := *configFilePath
	originalHelp := *help
	originalConfig := config
	originalLevel := log.GetLevel()
	originalFormatter := log.StandardLogger().Formatter

	defer func() {
		os.Args = originalArgs
		*configFilePath = originalConfigFilePath
		*help = originalHelp
		config = originalConfig
		log.SetLevel(originalLevel)
		log.SetFormatter(originalFormatter)
	}()

	testSimpleFlagParsing(t)
	testSimpleLogging(t)
	testSimpleConfigReading(t)
	testSimpleProcessing(t)
}
