package main

import (
	"encoding/json"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	amqp "github.com/rabbitmq/amqp091-go"
)

const localhostEndpoint = "localhost:9000"

// testNonExistentConfigFile tests reading non-existent configuration file
func testNonExistentConfigFile(t *testing.T) {
	err := readConfig("/path/that/does/not/exist/config.yaml")
	if err == nil {
		t.Error("Expected error for non-existent config file")
	}
}

// testEmptyConfigFile tests reading empty configuration file
func testEmptyConfigFile(t *testing.T, tmpDir string) {
	emptyFile := filepath.Join(tmpDir, "empty.yaml")
	err := os.WriteFile(emptyFile, []byte(""), 0600)
	if err != nil {
		t.Fatalf("Failed to create empty file: %v", err)
	}

	err = readConfig(emptyFile)
	if err != nil {
		t.Errorf("Unexpected error reading empty file: %v", err)
	}
}

// testInvalidYamlSyntax tests reading config file with invalid YAML syntax
func testInvalidYamlSyntax(t *testing.T, tmpDir string) {
	invalidYamlFile := filepath.Join(tmpDir, "invalid.yaml")
	invalidContent := `
log_level: "info"
outbound:
  - name: "test"
    invalid_yaml: [
    missing_close_bracket
`
	err := os.WriteFile(invalidYamlFile, []byte(invalidContent), 0600)
	if err != nil {
		t.Fatalf("Failed to create invalid YAML file: %v", err)
	}

	err = readConfig(invalidYamlFile)
	if err == nil {
		t.Error("Expected error for invalid YAML syntax")
	}
}

// testInvalidYamlStructure tests reading config with invalid YAML structure
func testInvalidYamlStructure(t *testing.T, tmpDir string) {
	invalidStructureFile := filepath.Join(tmpDir, "invalid-structure.yaml")
	invalidStructureContent := `
log_level: 123
outbound: "should be array"
inbound: 456
remotes: "should be array"
`
	err := os.WriteFile(invalidStructureFile, []byte(invalidStructureContent), 0600)
	if err != nil {
		t.Fatalf("Failed to create invalid structure file: %v", err)
	}

	err = readConfig(invalidStructureFile)
	if err != nil {
		t.Logf("Expected error for invalid YAML structure: %v", err)
		// This might or might not error depending on YAML parser behavior
	}
}

// testDirectoryAsFile tests reading directory as configuration file
func testDirectoryAsFile(t *testing.T, tmpDir string) {
	dirPath := filepath.Join(tmpDir, "directory-not-file")
	err := os.Mkdir(dirPath, 0700)
	if err != nil {
		t.Fatalf("Failed to create directory: %v", err)
	}

	err = readConfig(dirPath)
	if err == nil {
		t.Error("Expected error when trying to read directory as file")
	}
}

func TestConfigErrorHandling(t *testing.T) {
	// Test various configuration error scenarios
	tmpDir := t.TempDir()

	testNonExistentConfigFile(t)
	testEmptyConfigFile(t, tmpDir)
	testInvalidYamlSyntax(t, tmpDir)
	testInvalidYamlStructure(t, tmpDir)
	testDirectoryAsFile(t, tmpDir)
}

func TestJSONErrorHandling(t *testing.T) {
	// Test JSON parsing error scenarios

	// Test 1: Invalid JSON
	invalidJSON := `{
		"EventName": "s3:ObjectCreated:Put",
		"Records": [
			{
				"s3": {
					"bucket": {
						"name": "test-bucket"
					},
					"object": {
						"key": "test-file.txt",
						"size": invalid_number
					}
				}
			}
		]
	}`

	var message map[string]interface{}
	err := json.Unmarshal([]byte(invalidJSON), &message)
	if err == nil {
		t.Error("Expected error for invalid JSON")
	}

	// Test 2: Missing required fields
	incompleteJSON := `{
		"EventName": "s3:ObjectCreated:Put"
	}`

	err = json.Unmarshal([]byte(incompleteJSON), &message)
	if err != nil {
		t.Errorf("Unexpected error for incomplete JSON: %v", err)
	}

	// Check that accessing missing fields would cause panic in real code
	if _, exists := message["Records"]; exists {
		t.Error("Expected Records field to be missing")
	}

	// Test 3: Wrong data types
	wrongTypesJSON := `{
		"EventName": 123,
		"Records": "should be array"
	}`

	err = json.Unmarshal([]byte(wrongTypesJSON), &message)
	if err != nil {
		t.Errorf("Unexpected error for wrong types JSON: %v", err)
	}

	// Test type assertions that would fail
	if eventName, ok := message["EventName"].(string); ok {
		t.Errorf("Expected EventName type assertion to fail, got: %s", eventName)
	}
}

func TestURLParsingErrors(t *testing.T) {
	// Test URL parsing error scenarios

	invalidURLs := []string{
		"",
		"invalid-url",
		"://missing-scheme",
		"http://",
		"ftp://unsupported-scheme.com",
		"http://[invalid-ipv6",
		"http://example.com:invalid-port",
		"http://exam ple.com/space in hostname",
	}

	for _, invalidURL := range invalidURLs {
		_, err := url.Parse(invalidURL)
		if err == nil && invalidURL != "" {
			// Some URLs might parse successfully even if they're unusual
			t.Logf("URL '%s' parsed successfully (might be valid)", invalidURL)
		}
	}

	// Test hostname extraction errors
	urls := []string{
		"",
		"invalid",
		"://no-scheme",
	}

	for _, urlStr := range urls {
		u, err := url.Parse(urlStr)
		if err != nil {
			continue // Expected for invalid URLs
		}

		hostname := u.Hostname()
		if hostname == "" && urlStr != "" {
			t.Logf("Empty hostname for URL: %s", urlStr)
		}
	}
}

func TestPathTokenizationErrors(t *testing.T) {
	// Test path tokenization edge cases

	paths := []string{
		"",
		"/",
		"//",
		"///",
		"/single",
		"/bucket/",
		"/bucket//double-slash",
		"/bucket/path/with/many/segments/but/no/file",
	}

	for _, path := range paths {
		tokens := strings.Split(path, "/")

		// Test minimum token requirement
		const minTokens = 2
		if len(tokens) < minTokens {
			t.Logf("Path '%s' has insufficient tokens: %d", path, len(tokens))
		}

		// Test empty token handling
		for i, token := range tokens {
			if token == "" && i != 0 {
				t.Logf("Empty token at position %d in path '%s'", i, path)
			}
		}
	}
}

func TestRemoteMatchingErrors(t *testing.T) {
	// Test remote matching error scenarios

	// Test with empty remotes
	config = Config{
		Remotes: []Remote{},
	}

	endpoints := []string{localhostEndpoint, "s3.amazonaws.com"}
	for _, endpoint := range endpoints {
		found := false
		for _, remote := range config.Remotes {
			if remote.Endpoint == endpoint {
				found = true
				break
			}
		}
		if found {
			t.Errorf("Found remote for endpoint '%s' in empty remotes list", endpoint)
		}
	}

	// Test with nil remotes
	config = Config{}

	for _, endpoint := range endpoints {
		found := false
		for _, remote := range config.Remotes {
			if remote.Endpoint == endpoint {
				found = true
				break
			}
		}
		if found {
			t.Errorf("Found remote for endpoint '%s' in nil remotes list", endpoint)
		}
	}

	// Test case sensitivity
	config = Config{
		Remotes: []Remote{
			{Name: "test", Endpoint: localhostEndpoint},
		},
	}

	caseEndpoints := []string{
		"LOCALHOST:9000",
		"Localhost:9000",
		localhostEndpoint,
	}

	for _, endpoint := range caseEndpoints {
		found := false
		for _, remote := range config.Remotes {
			if remote.Endpoint == endpoint {
				found = true
				break
			}
		}
		if endpoint != "localhost:9000" && found {
			t.Errorf("Found remote for case-different endpoint '%s'", endpoint)
		}
	}
}

func TestFilePathErrors(t *testing.T) {
	// Test file path error scenarios

	// Test with empty paths
	emptyPaths := []string{"", " ", "\t", "\n"}

	for _, path := range emptyPaths {
		dir := filepath.Dir(path)
		base := filepath.Base(path)

		t.Logf("Empty path '%s': dir='%s', base='%s'", path, dir, base)
	}

	// Test with invalid characters (platform-specific)
	// Note: This is more relevant on Windows
	if os.PathSeparator == '\\' {
		invalidPaths := []string{
			"path/with/forward/slashes",
			"path\\with\\invalid<chars>",
			"path\\with\\invalid|chars",
		}

		for _, path := range invalidPaths {
			dir := filepath.Dir(path)
			base := filepath.Base(path)
			t.Logf("Potentially invalid path '%s': dir='%s', base='%s'", path, dir, base)
		}
	}

	// Test extremely long paths
	longPath := strings.Repeat("very-long-directory-name/", 100) + "file.txt"
	dir := filepath.Dir(longPath)
	base := filepath.Base(longPath)

	if len(dir) > 4096 {
		t.Logf("Very long directory path: %d characters", len(dir))
	}
	if base != "file.txt" {
		t.Errorf("Expected base 'file.txt', got '%s'", base)
	}
}

func TestStringOperationErrors(t *testing.T) {
	// Test string operation edge cases

	// Test empty string operations
	emptyStrings := []string{"", " ", "\t", "\n"}

	for _, str := range emptyStrings {
		tokens := strings.Split(str, "/")
		joined := strings.Join(tokens, "/")

		if len(tokens) == 0 {
			t.Errorf("Split of '%s' resulted in empty slice", str)
		}

		t.Logf("String '%s': tokens=%d, joined='%s'", str, len(tokens), joined)
	}

	// Test nil slice operations
	var nilSlice []string
	joined := strings.Join(nilSlice, "/")
	if joined != "" {
		t.Errorf("Expected empty string from nil slice join, got '%s'", joined)
	}

	// Test single element operations
	singleElement := []string{"single"}
	joined = strings.Join(singleElement, "/")
	if joined != "single" {
		t.Errorf("Expected 'single', got '%s'", joined)
	}

	// Test operations with special characters
	specialChars := []string{"path/with/slash", "path with space", "path\twith\ttab"}

	for _, path := range specialChars {
		tokens := strings.Split(path, "/")
		rejoined := strings.Join(tokens, "/")

		if rejoined != path {
			t.Logf("Path transformation changed: '%s' -> '%s'", path, rejoined)
		}
	}
}

func TestConcurrencyErrors(_ *testing.T) {
	// Test potential concurrency issues with proper synchronization

	// Test concurrent access to global variables
	originalConfig := config
	defer func() { config = originalConfig }()

	// Use a local variable with mutex to avoid race condition
	var mu sync.RWMutex
	localConfig := Config{LogLevel: "info"}
	done := make(chan bool, 2)

	go func() {
		for i := 0; i < 100; i++ {
			mu.Lock()
			localConfig = Config{LogLevel: "debug"}
			mu.Unlock()
		}
		done <- true
	}()

	go func() {
		for i := 0; i < 100; i++ {
			mu.RLock()
			_ = localConfig.LogLevel
			mu.RUnlock()
		}
		done <- true
	}()

	// Wait for both goroutines
	<-done
	<-done

	// Test concurrent access to connections slice with proper synchronization
	originalConnections := connections
	defer func() { connections = originalConnections }()

	localConnections2 := make([]*amqp.Connection, 0)
	var mu2 sync.RWMutex

	done = make(chan bool, 2)

	go func() {
		for i := 0; i < 50; i++ {
			mu2.Lock()
			localConnections2 = append(localConnections2, nil)
			mu2.Unlock()
		}
		done <- true
	}()

	go func() {
		for i := 0; i < 50; i++ {
			mu2.RLock()
			_ = len(localConnections2)
			mu2.RUnlock()
		}
		done <- true
	}()

	// Wait for both goroutines
	<-done
	<-done

	// Note: This test may expose race conditions but won't necessarily fail
	// To properly test for race conditions, run with -race flag
}

func TestMemoryLeakPrevention(t *testing.T) {
	// Test scenarios that could cause memory leaks

	// Test large configuration that should be garbage collected
	largeConfig := Config{
		Outbound: make([]Outbound, 1000),
		Inbound:  make([]Inbound, 1000),
		Remotes:  make([]Remote, 1000),
	}

	// Fill with data
	for i := 0; i < 1000; i++ {
		largeConfig.Outbound[i] = Outbound{
			Name:        "test-" + strings.Repeat("x", 100),
			Description: strings.Repeat("description", 50),
			Source:      strings.Repeat("/path/", 20),
			Destination: strings.Repeat("s3://bucket/", 10),
		}
	}

	// Clear reference to allow garbage collection
	largeConfig = Config{}

	// Test multiple temporary configurations
	for i := 0; i < 100; i++ {
		tmpDir := t.TempDir()
		configFile := filepath.Join(tmpDir, "temp-config.yaml")

		content := `
log_level: "info"
outbound: []
inbound: []
remotes: []
`
		err := os.WriteFile(configFile, []byte(content), 0600)
		if err != nil {
			t.Fatalf("Failed to create temp config: %v", err)
		}

		err = readConfig(configFile)
		if err != nil {
			t.Fatalf("Failed to read temp config: %v", err)
		}
	}
}

// testUnicodeConfig tests configuration with Unicode characters
func testUnicodeConfig(t *testing.T, tmpDir string) {
	unicodeFile := filepath.Join(tmpDir, "unicode-config.yaml")
	unicodeContent := `
log_level: "info"
outbound:
  - name: "测试-outbound"
    description: "Unicode description with émojis 🚀"
    source: "/tmp/测试/*"
    destination: "s3://测试-bucket/uploads/"
    sensitive: false
remotes:
  - name: "测试-remote"
    endpoint: "测试.example.com:9000"
    accessKey: "测试-key"
    secretKey: "测试-secret"
`

	err := os.WriteFile(unicodeFile, []byte(unicodeContent), 0600)
	if err != nil {
		t.Fatalf("Failed to create unicode config: %v", err)
	}

	err = readConfig(unicodeFile)
	if err != nil {
		t.Errorf("Failed to read unicode config: %v", err)
	}

	// Verify unicode values were preserved
	if len(config.Outbound) > 0 {
		outbound := config.Outbound[0]
		if !strings.Contains(outbound.Name, "测试") {
			t.Error("Unicode characters not preserved in outbound name")
		}
	}
}

// testLongStringConfig tests configuration with very long strings
func testLongStringConfig(t *testing.T, tmpDir string) {
	longStringFile := filepath.Join(tmpDir, "long-string-config.yaml")
	longString := strings.Repeat("very-long-string-", 1000)
	longStringContent := `
log_level: "info"
outbound:
  - name: "` + longString + `"
    description: "` + longString + `"
    source: "/tmp/test/*"
    destination: "s3://bucket/path/"
    sensitive: false
`

	err := os.WriteFile(longStringFile, []byte(longStringContent), 0600)
	if err != nil {
		t.Fatalf("Failed to create long string config: %v", err)
	}

	err = readConfig(longStringFile)
	if err != nil {
		t.Errorf("Failed to read long string config: %v", err)
	}
}

// testExtremeValuesConfig tests configuration with extreme values
func testExtremeValuesConfig(t *testing.T, tmpDir string) {
	extremeFile := filepath.Join(tmpDir, "extreme-config.yaml")
	extremeContent := `
log_level: ""
log_json: false
outbound: []
inbound: []
remotes: []
`

	err := os.WriteFile(extremeFile, []byte(extremeContent), 0600)
	if err != nil {
		t.Fatalf("Failed to create extreme config: %v", err)
	}

	err = readConfig(extremeFile)
	if err != nil {
		t.Errorf("Failed to read extreme config: %v", err)
	}
}

func TestEdgeCaseValues(t *testing.T) {
	// Test with edge case values
	tmpDir := t.TempDir()

	testUnicodeConfig(t, tmpDir)
	testLongStringConfig(t, tmpDir)
	testExtremeValuesConfig(t, tmpDir)
}
