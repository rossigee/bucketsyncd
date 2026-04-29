package main

import (
	"flag"
	"os"
	"testing"
)

func TestParseCommandLine(t *testing.T) {
	tests := []struct {
		name     string
		args     []string
		expected bool
	}{
		{"version flag", []string{"-version"}, false},
		{"valid config", []string{"-c", "/tmp/config.yaml"}, true},
		{"default config", []string{}, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
		// Reset flags
		flag.CommandLine = flag.NewFlagSet(os.Args[0], flag.ExitOnError)
		configFilePath = flag.String("c", "/etc/bucketsyncd/config.yaml", "Configuration file location")
		showVersion = flag.Bool("version", false, "Show version information")
		help = flag.Bool("h", false, "Usage information")

			// Set args
			os.Args = append([]string{"bucketsyncd"}, tt.args...)

			result := parseCommandLine()
			if result != tt.expected {
				t.Errorf("parseCommandLine() = %v, want %v", result, tt.expected)
			}
		})
	}
}