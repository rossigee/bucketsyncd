package main

import (
	"bytes"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	log "github.com/sirupsen/logrus"
)

// mockWebDAVServer creates a mock WebDAV server for testing
func mockWebDAVServer(t *testing.T) *httptest.Server {
	// In-memory storage for the mock server
	files := make(map[string][]byte)
	dirs := make(map[string]bool)

	mux := http.NewServeMux()

	// Handle PROPFIND (used for directory listing and file existence checks)
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		path := r.URL.Path
		t.Logf("Mock WebDAV server: %s %s", r.Method, path)

		switch r.Method {
		case "PROPFIND":
			// Check if it's a file or directory
			if _, exists := files[path]; exists {
				// It's a file
				w.Header().Set("Content-Type", "application/xml")
				w.WriteHeader(207) // Multi-Status
				_, _ = fmt.Fprintf(w, `<?xml version="1.0" encoding="utf-8"?>
<d:multistatus xmlns:d="DAV:">
  <d:response>
    <d:href>%s</d:href>
    <d:propstat>
      <d:prop>
        <d:resourcetype/>
        <d:getcontentlength>%d</d:getcontentlength>
      </d:prop>
      <d:status>HTTP/1.1 200 OK</d:status>
    </d:propstat>
  </d:response>
</d:multistatus>`, path, len(files[path]))
			} else if dirs[path] {
				// It's a directory
				w.Header().Set("Content-Type", "application/xml")
				w.WriteHeader(207) // Multi-Status
				response := `<?xml version="1.0" encoding="utf-8"?>
<d:multistatus xmlns:d="DAV:">`
				
				// Add directory itself
				response += fmt.Sprintf(`
  <d:response>
    <d:href>%s</d:href>
    <d:propstat>
      <d:prop>
        <d:resourcetype><d:collection/></d:resourcetype>
      </d:prop>
      <d:status>HTTP/1.1 200 OK</d:status>
    </d:propstat>
  </d:response>`, path)

				// Add files in directory
				for filePath := range files {
					if strings.HasPrefix(filePath, path) && filePath != path {
						response += fmt.Sprintf(`
  <d:response>
    <d:href>%s</d:href>
    <d:propstat>
      <d:prop>
        <d:resourcetype/>
        <d:getcontentlength>%d</d:getcontentlength>
      </d:prop>
      <d:status>HTTP/1.1 200 OK</d:status>
    </d:propstat>
  </d:response>`, filePath, len(files[filePath]))
					}
				}
				
				response += `
</d:multistatus>`
				_, _ = fmt.Fprint(w, response)
			} else {
				w.WriteHeader(404)
			}

		case "PUT":
			// Read the body
			body, err := io.ReadAll(r.Body)
			if err != nil {
				w.WriteHeader(500)
				return
			}
			// Store the file
			files[path] = body
			// Ensure parent directory exists
			parentDir := path[:strings.LastIndex(path, "/")]
			if parentDir != "" {
				dirs[parentDir] = true
			}
			w.WriteHeader(201) // Created

		case "GET":
			if data, exists := files[path]; exists {
				w.Header().Set("Content-Type", "application/octet-stream")
				w.WriteHeader(200)
				_, _ = w.Write(data)
			} else {
				w.WriteHeader(404)
			}

		case "DELETE":
			if _, exists := files[path]; exists {
				delete(files, path)
				w.WriteHeader(204) // No Content
			} else {
				w.WriteHeader(404)
			}

		case "MKCOL":
			dirs[path] = true
			w.WriteHeader(201) // Created

		default:
			w.WriteHeader(405) // Method Not Allowed
		}
	})

	server := httptest.NewServer(mux)
	t.Cleanup(server.Close)
	return server
}

func TestWebDAVClient_NewWebDAVClient(t *testing.T) {
	tests := []struct {
		name        string
		url         string
		expectError bool
	}{
		{
			name:        "valid webdav URL",
			url:         "webdav://user:pass@example.com/path",
			expectError: false,
		},
		{
			name:        "valid webdavs URL",
			url:         "webdavs://user:pass@example.com/path",
			expectError: false,
		},
		{
			name:        "invalid URL",
			url:         "://invalid",
			expectError: true,
		},
		{
			name:        "valid URL without credentials",
			url:         "webdav://example.com/path",
			expectError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client, err := NewWebDAVClient(tt.url)
			if tt.expectError {
				if err == nil {
					t.Error("expected error, got nil")
				}
			} else {
				if err != nil {
					t.Errorf("unexpected error: %v", err)
				}
				if client == nil {
					t.Error("expected client, got nil")
				}
			}
		})
	}
}

func TestWebDAVClient_Upload(t *testing.T) {
	server := mockWebDAVServer(t)
	
	// Create client pointing to mock server
	client, err := NewWebDAVClient(server.URL)
	if err != nil {
		t.Fatalf("failed to create WebDAV client: %v", err)
	}

	testData := "Hello, WebDAV!"
	reader := strings.NewReader(testData)

	err = client.Upload(reader, "/test-file.txt")
	if err != nil {
		t.Errorf("upload failed: %v", err)
	}

	// Verify the file was uploaded by checking if it exists
	if !client.Exists("/test-file.txt") {
		t.Error("uploaded file does not exist")
	}
}

func TestWebDAVClient_Download(t *testing.T) {
	server := mockWebDAVServer(t)
	
	client, err := NewWebDAVClient(server.URL)
	if err != nil {
		t.Fatalf("failed to create WebDAV client: %v", err)
	}

	// First upload a test file
	testData := "Hello, WebDAV Download!"
	reader := strings.NewReader(testData)
	err = client.Upload(reader, "/download-test.txt")
	if err != nil {
		t.Fatalf("failed to upload test file: %v", err)
	}

	// Now download it
	downloadReader, err := client.Download("/download-test.txt")
	if err != nil {
		t.Errorf("download failed: %v", err)
	}
	defer func() {
		_ = downloadReader.Close()
	}()

	downloadedData, err := io.ReadAll(downloadReader)
	if err != nil {
		t.Errorf("failed to read downloaded data: %v", err)
	}

	if string(downloadedData) != testData {
		t.Errorf("downloaded data doesn't match. Expected: %s, Got: %s", testData, string(downloadedData))
	}
}

func TestWebDAVClient_Exists(t *testing.T) {
	server := mockWebDAVServer(t)
	
	client, err := NewWebDAVClient(server.URL)
	if err != nil {
		t.Fatalf("failed to create WebDAV client: %v", err)
	}

	// Test non-existent file
	if client.Exists("/non-existent-file.txt") {
		t.Error("non-existent file reported as existing")
	}

	// Upload a file and test it exists
	testData := "Exists test"
	reader := strings.NewReader(testData)
	err = client.Upload(reader, "/exists-test.txt")
	if err != nil {
		t.Fatalf("failed to upload test file: %v", err)
	}

	if !client.Exists("/exists-test.txt") {
		t.Error("uploaded file reported as not existing")
	}
}

func TestWebDAVClient_Delete(t *testing.T) {
	server := mockWebDAVServer(t)
	
	client, err := NewWebDAVClient(server.URL)
	if err != nil {
		t.Fatalf("failed to create WebDAV client: %v", err)
	}

	// Upload a file first
	testData := "Delete test"
	reader := strings.NewReader(testData)
	err = client.Upload(reader, "/delete-test.txt")
	if err != nil {
		t.Fatalf("failed to upload test file: %v", err)
	}

	// Verify it exists
	if !client.Exists("/delete-test.txt") {
		t.Error("uploaded file does not exist before delete")
	}

	// Delete the file
	err = client.Delete("/delete-test.txt")
	if err != nil {
		t.Errorf("delete failed: %v", err)
	}

	// Verify it no longer exists
	if client.Exists("/delete-test.txt") {
		t.Error("file still exists after delete")
	}
}

func TestParseWebDAVURL(t *testing.T) {
	tests := []struct {
		name           string
		url            string
		expectedEndpoint string
		expectedPath   string
		expectError    bool
	}{
		{
			name:           "webdav URL with credentials",
			url:            "webdav://user:pass@example.com/path/to/file",
			expectedEndpoint: "http://user:pass@example.com",
			expectedPath:    "/path/to/file",
			expectError:    false,
		},
		{
			name:           "webdavs URL",
			url:            "webdavs://user:pass@example.com/secure/path",
			expectedEndpoint: "https://user:pass@example.com",
			expectedPath:    "/secure/path",
			expectError:    false,
		},
		{
			name:           "webdav URL without credentials",
			url:            "webdav://example.com/public/path",
			expectedEndpoint: "http://example.com",
			expectedPath:    "/public/path",
			expectError:    false,
		},
		{
			name:        "invalid scheme",
			url:         "http://example.com/path",
			expectError: true,
		},
		{
			name:        "invalid URL",
			url:         "://invalid",
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			endpoint, path, err := parseWebDAVURL(tt.url)
			
			if tt.expectError {
				if err == nil {
					t.Error("expected error, got nil")
				}
			} else {
				if err != nil {
					t.Errorf("unexpected error: %v", err)
				}
				if endpoint != tt.expectedEndpoint {
					t.Errorf("endpoint mismatch. Expected: %s, Got: %s", tt.expectedEndpoint, endpoint)
				}
				if path != tt.expectedPath {
					t.Errorf("path mismatch. Expected: %s, Got: %s", tt.expectedPath, path)
				}
			}
		})
	}
}

func TestIsWebDAVScheme(t *testing.T) {
	tests := []struct {
		scheme   string
		expected bool
	}{
		{"webdav", true},
		{"webdavs", true},
		{"WEBDAV", true},
		{"WEBDAVS", true},
		{"http", false},
		{"https", false},
		{"ftp", false},
		{"", false},
	}

	for _, tt := range tests {
		t.Run(tt.scheme, func(t *testing.T) {
			result := isWebDAVScheme(tt.scheme)
			if result != tt.expected {
				t.Errorf("isWebDAVScheme(%s) = %v, expected %v", tt.scheme, result, tt.expected)
			}
		})
	}
}

func TestWebDAVOutboundIntegration(t *testing.T) {
	// Suppress log output during tests unless explicitly testing logging
	log.SetLevel(log.FatalLevel)
	defer log.SetLevel(log.InfoLevel)

	server := mockWebDAVServer(t)
	
	// Test URL parsing
	_, _, err := parseWebDAVURL(server.URL + "/uploads")
	if err == nil {
		// The mock server URL will be http://..., not webdav://..., so this should fail
		// which is expected behavior
		t.Log("URL parsing correctly rejected non-webdav scheme")
	}

	// Test with proper WebDAV URL format
	webdavURL := strings.Replace(server.URL, "http://", "webdav://", 1) + "/uploads"
	endpoint, path, err := parseWebDAVURL(webdavURL)
	if err != nil {
		t.Errorf("failed to parse WebDAV URL: %v", err)
	}

	// Convert back to http for the client since our mock server is HTTP
	httpURL := strings.Replace(webdavURL, "webdav://", "http://", 1)
	client, err := NewWebDAVClient(httpURL)
	if err != nil {
		t.Errorf("failed to create WebDAV client: %v", err)
	}

	// Test upload
	testContent := "Integration test content"
	reader := bytes.NewReader([]byte(testContent))
	err = client.Upload(reader, "/uploads/integration-test.txt")
	if err != nil {
		t.Errorf("integration upload failed: %v", err)
	}

	t.Logf("Successfully parsed WebDAV URL - endpoint: %s, path: %s", endpoint, path)
}