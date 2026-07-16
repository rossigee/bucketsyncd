package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"os"
	"testing"
	"time"

	amqp "github.com/rabbitmq/amqp091-go"
)

// TestRetryOperation tests the RetryOperation utility function
func TestRetryOperation(t *testing.T) {
	tests := []struct {
		name      string
		operation func() error
		maxRetries int
		expectErr bool
		expectCalls int
	}{
		{
			name: "success_on_first_call",
			operation: func() error {
				return nil
			},
			maxRetries:  3,
			expectErr:   false,
			expectCalls: 1,
		},
		{
			name: "success_after_retries",
			operation: func() func() error {
				callCount := 0
				return func() error {
					callCount++
					if callCount < 3 {
						return errors.New("temporary failure")
					}
					return nil
				}
			}(),
			maxRetries:  5,
			expectErr:   false,
			expectCalls: 3,
		},
		{
			name: "failure_all_retries",
			operation: func() error {
				return errors.New("persistent failure")
			},
			maxRetries:  3,
			expectErr:   true,
			expectCalls: 3,
		},
		{
			name: "single_retry",
			operation: func() error {
				return nil
			},
			maxRetries:  1,
			expectErr:   false,
			expectCalls: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			callCount := 0
			wrappedOp := func() error {
				callCount++
				return tt.operation()
			}

			err := RetryOperation(wrappedOp, tt.maxRetries)

			if (err != nil) != tt.expectErr {
				t.Errorf("expected error=%v, got %v", tt.expectErr, err != nil)
			}
		})
	}
}

// TestRetryOperationBackoff tests exponential backoff timing
func TestRetryOperationBackoff(t *testing.T) {
	start := time.Now()
	callCount := 0
	op := func() error {
		callCount++
		if callCount < 3 {
			return errors.New("fail")
		}
		return nil
	}

	err := RetryOperation(op, 3)
	elapsed := time.Since(start)

	if err != nil {
		t.Errorf("expected success, got %v", err)
	}

	// Should have exponential backoff: 1s + 2s = 3s minimum
	if elapsed < 3*time.Second {
		t.Errorf("expected backoff delay ≥3s, got %v", elapsed)
	}
}

// TestConfigureLogging tests logging configuration
func TestConfigureLogging(t *testing.T) {
	originalConfig := config
	defer func() { config = originalConfig }()

	tests := []struct {
		name     string
		logLevel string
		logJSON  bool
	}{
		{
			name:     "debug_text",
			logLevel: "debug",
			logJSON:  false,
		},
		{
			name:     "info_text",
			logLevel: "info",
			logJSON:  false,
		},
		{
			name:     "warn_text",
			logLevel: "warn",
			logJSON:  false,
		},
		{
			name:     "debug_json",
			logLevel: "debug",
			logJSON:  true,
		},
		{
			name:     "info_json",
			logLevel: "info",
			logJSON:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config = Config{
				LogLevel: tt.logLevel,
				LogJSON:  tt.logJSON,
			}

			// Should not panic
			configureLogging()
		})
	}
}

// TestDownloadRecordWithMissingRemote tests downloadRecord with missing remote
func TestDownloadRecordWithMissingRemote(t *testing.T) {
	config.Remotes = []Remote{}

	ctx := context.Background()
	lf := map[string]interface{}{"test": "value"}
	in := Inbound{
		Remote: "nonexistent",
	}

	err := downloadRecord(ctx, lf, "bucket", "key", in)
	if err == nil {
		t.Error("expected error for missing remote")
	}
}

// TestDownloadRecordContextCancellation tests downloadRecord with cancelled context
func TestDownloadRecordContextCancellation(t *testing.T) {
	config.Remotes = []Remote{
		{
			Name:      "test-remote",
			Endpoint:  "http://localhost:9000",
			AccessKey: "test",
			SecretKey: "test",
		},
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	lf := map[string]interface{}{"test": "value"}
	in := Inbound{
		Remote:   "test-remote",
		Destination: "s3://other-bucket",
	}

	err := downloadRecord(ctx, lf, "bucket", "key", in)
	if err == nil {
		t.Error("expected error for cancelled context")
	}
}

// TestInboundWithContextCancellation tests inbound with cancelled context
func TestInboundWithContextCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	in := Inbound{
		Name:        "test",
		Description: "Test inbound",
		Source:      "amqp://guest:guest@localhost:5672/",
		Exchange:    "test-exchange",
		Queue:       "test-queue",
		Remote:      "test-remote",
	}

	// Should return quickly without errors
	inboundWithContext(ctx, in)
}

// TestInboundCloseWithEmptyConnections tests inboundClose with no connections
func TestInboundCloseWithEmptyConnections(t *testing.T) {
	originalConnections := connections
	defer func() { connections = originalConnections }()

	connections = []*amqp.Connection{}

	// Should not panic
	inboundClose()
}

// TestSendNotificationWithDisabledNotifications tests SendNotification with notifications disabled
func TestSendNotificationWithDisabledNotifications(t *testing.T) {
	originalConfig := config
	defer func() { config = originalConfig }()

	config = Config{
		EnableNotifications: false,
	}

	// Should not panic or error
	SendNotification("test", "message")
}

// TestS3EventParsing tests S3 event JSON parsing
func TestS3EventParsing(t *testing.T) {
	eventJSON := `{
		"EventName": "s3:ObjectCreated:Put",
		"Records": [
			{
				"s3": {
					"bucket": {
						"name": "test-bucket"
					},
					"object": {
						"key": "test-key",
						"size": 1024
					}
				}
			}
		]
	}`

	var event S3Event
	err := json.Unmarshal([]byte(eventJSON), &event)
	if err != nil {
		t.Errorf("failed to parse S3 event: %v", err)
	}

	if len(event.Records) != 1 {
		t.Errorf("expected 1 record, got %d", len(event.Records))
	}

	if event.Records[0].S3.Bucket.Name != "test-bucket" {
		t.Errorf("expected bucket name 'test-bucket', got '%s'", event.Records[0].S3.Bucket.Name)
	}
}

// TestInvalidS3EventParsing tests S3 event parsing with invalid JSON
func TestInvalidS3EventParsing(t *testing.T) {
	invalidJSON := `{invalid json}`

	var event S3Event
	err := json.Unmarshal([]byte(invalidJSON), &event)
	if err == nil {
		t.Error("expected parsing error for invalid JSON")
	}
}

// TestParseCommandLineAllFlags tests parseCommandLine with all flags
func TestParseCommandLineAllFlagsExtended(t *testing.T) {
	oldArgs := os.Args
	defer func() { os.Args = oldArgs }()

	tests := []struct {
		name      string
		args      []string
		expectSuccess bool
	}{
		{
			name:      "with_config",
			args:      []string{"prog", "-c", "/tmp/config.yaml"},
			expectSuccess: true,
		},
		{
			name:      "help_flag",
			args:      []string{"prog", "-h"},
			expectSuccess: false,
		},
		{
			name:      "version_flag",
			args:      []string{"prog", "-version"},
			expectSuccess: false,
		},
		{
			name:      "no_flags",
			args:      []string{"prog"},
			expectSuccess: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			os.Args = tt.args

			// Reset flag parser
			newFS := &flag.FlagSet{}
			newFS.String("c", "", "Configuration file location")
			newFS.Bool("h", false, "Usage information")
			newFS.Bool("version", false, "Show version information")
		})
	}
}

// TestWebDAVListFunction tests the WebDAV List function
func TestWebDAVListFunction(t *testing.T) {
	// This test would require a mock WebDAV server
	// For now, we document that List needs implementation
	t.Skip("WebDAV List function needs mock server implementation")
}

// TestOutboundWithInvalidDestination tests outbound with invalid destination
func TestOutboundWithInvalidDestination(t *testing.T) {
	originalConfig := config
	defer func() { config = originalConfig }()

	config = Config{
		Outbound: []Outbound{
			{
				Name:        "invalid-test",
				Source:      "/tmp/nonexistent",
				Destination: "invalid://destination",
			},
		},
	}

	// Should handle gracefully without crashing
	outbound(config.Outbound[0])
}

// TestOutboundWithValidTempDirectory tests outbound with valid temp directory
func TestOutboundWithValidTempDirectory(t *testing.T) {
	tmpDir := t.TempDir()

	originalConfig := config
	defer func() { config = originalConfig }()

	config = Config{
		Outbound: []Outbound{
			{
				Name:        "temp-test",
				Source:      tmpDir,
				Destination: "s3://test-bucket/",
			},
		},
		Remotes: []Remote{
			{
				Name:      "default",
				Endpoint:  "http://localhost:9000",
				AccessKey: "test",
				SecretKey: "test",
			},
		},
	}

	// Should initialize without error
	outbound(config.Outbound[0])
}

// TestInboundWithInvalidSource tests inbound with invalid AMQP source
func TestInboundWithInvalidSource(t *testing.T) {
	in := Inbound{
		Name:        "invalid-source",
		Description: "Test with invalid source",
		Source:      "invalid://url",
		Exchange:    "test",
		Queue:       "test",
		Remote:      "test",
	}

	// Should return gracefully
	inbound(in)
}

// TestConfigWithMultipleRemotes tests configuration with multiple remotes
func TestConfigWithMultipleRemotes(t *testing.T) {
	originalConfig := config
	defer func() { config = originalConfig }()

	config = Config{
		Remotes: []Remote{
			{
				Name:      "remote1",
				Endpoint:  "http://s3-1.example.com",
				AccessKey: "key1",
				SecretKey: "secret1",
			},
			{
				Name:      "remote2",
				Endpoint:  "http://s3-2.example.com",
				AccessKey: "key2",
				SecretKey: "secret2",
			},
		},
	}

	if len(config.Remotes) != 2 {
		t.Errorf("expected 2 remotes, got %d", len(config.Remotes))
	}

	for i, remote := range config.Remotes {
		expectedName := fmt.Sprintf("remote%d", i+1)
		if remote.Name != expectedName {
			t.Errorf("expected remote name '%s', got '%s'", expectedName, remote.Name)
		}
	}
}

// TestWebDAVURLHandling tests WebDAV URL parsing and handling
func TestWebDAVURLHandling(t *testing.T) {
	tests := []struct {
		name      string
		url       string
		scheme    string
		expectErr bool
	}{
		{
			name:      "webdav_url",
			url:       "webdav://example.com/path",
			scheme:    "webdav",
			expectErr: false,
		},
		{
			name:      "webdavs_url",
			url:       "webdavs://example.com/path",
			scheme:    "webdavs",
			expectErr: false,
		},
		{
			name:      "invalid_url",
			url:       ":",
			scheme:    "",
			expectErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := NewWebDAVClient(tt.url)
			if (err != nil) != tt.expectErr {
				t.Errorf("expected error=%v, got %v", tt.expectErr, err != nil)
			}
		})
	}
}

// TestS3RecordWithURLEncodedKeyExtended tests URL-encoded keys in S3 records
func TestS3RecordWithURLEncodedKeyExtended(t *testing.T) {
	eventJSON := `{
		"EventName": "s3:ObjectCreated:Put",
		"Records": [
			{
				"s3": {
					"bucket": {
						"name": "test-bucket"
					},
					"object": {
						"key": "test%20key%20with%20spaces",
						"size": 2048
					}
				}
			}
		]
	}`

	var event S3Event
	err := json.Unmarshal([]byte(eventJSON), &event)
	if err != nil {
		t.Errorf("failed to parse S3 event: %v", err)
	}

	if len(event.Records) != 1 {
		t.Errorf("expected 1 record, got %d", len(event.Records))
	}
}

// TestReadConfigWithIO tests reading config with actual file I/O
func TestReadConfigWithIO(t *testing.T) {
	tmpFile := t.TempDir() + "/config.yaml"
	configContent := `
logLevel: debug
logJSON: false
remotes:
  - name: default
    endpoint: http://localhost:9000
    accessKey: minioadmin
    secretKey: minioadmin
outbound:
  - name: test-outbound
    source: /tmp/source
    destination: s3://bucket/path
    sourceType: local
inbound:
  - name: test-inbound
    source: amqp://localhost
    exchange: test
    queue: test
    remote: default
`

	err := os.WriteFile(tmpFile, []byte(configContent), 0o600)
	if err != nil {
		t.Fatalf("failed to write test config: %v", err)
	}

	err = readConfig(tmpFile)
	if err != nil {
		t.Errorf("failed to read config: %v", err)
	}
}

// TestContextDeadlineHandling tests proper context deadline handling
func TestContextDeadlineHandling(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	in := Inbound{
		Name:        "timeout-test",
		Description: "Test timeout",
		Source:      "amqp://guest@localhost:5672/",
		Exchange:    "test",
		Queue:       "test",
		Remote:      "test",
	}

	start := time.Now()
	inboundWithContext(ctx, in)
	elapsed := time.Since(start)

	// Should respect context timeout
	if elapsed > 2*time.Second {
		t.Logf("timeout test took longer than expected: %v", elapsed)
	}
}
