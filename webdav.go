package main

import (
	"fmt"
	"io"
	"net/url"
	"path"
	"strings"

	"github.com/studio-b12/gowebdav"
	log "github.com/sirupsen/logrus"
)

// WebDAVClient wraps the gowebdav client with additional functionality
type WebDAVClient struct {
	client   *gowebdav.Client
	baseURL  *url.URL
	username string
	password string
}

// NewWebDAVClient creates a new WebDAV client from a URL
func NewWebDAVClient(urlStr string) (*WebDAVClient, error) {
	u, err := url.Parse(urlStr)
	if err != nil {
		return nil, fmt.Errorf("failed to parse WebDAV URL: %w", err)
	}

	// Extract credentials from URL if present
	username := ""
	password := ""
	if u.User != nil {
		username = u.User.Username()
		if p, ok := u.User.Password(); ok {
			password = p
		}
	}

	// Create base URL without credentials for client
	baseURL := &url.URL{
		Scheme: u.Scheme,
		Host:   u.Host,
		Path:   u.Path,
	}

	client := gowebdav.NewClient(baseURL.String(), username, password)

	return &WebDAVClient{
		client:   client,
		baseURL:  baseURL,
		username: username,
		password: password,
	}, nil
}

// Upload uploads a file to the WebDAV server
func (w *WebDAVClient) Upload(localReader io.Reader, remotePath string) error {
	// Ensure the directory exists
	remoteDir := path.Dir(remotePath)
	if remoteDir != "/" && remoteDir != "." {
		if err := w.client.MkdirAll(remoteDir, 0755); err != nil {
			log.WithFields(log.Fields{
				"remote_dir": remoteDir,
			}).Warn("failed to create remote directory, continuing anyway: ", err)
		}
	}

	// Upload the file
	err := w.client.WriteStream(remotePath, localReader, 0644)
	if err != nil {
		return fmt.Errorf("failed to upload file to WebDAV: %w", err)
	}

	return nil
}

// Download downloads a file from the WebDAV server
func (w *WebDAVClient) Download(remotePath string) (io.ReadCloser, error) {
	reader, err := w.client.ReadStream(remotePath)
	if err != nil {
		return nil, fmt.Errorf("failed to download file from WebDAV: %w", err)
	}

	return reader, nil
}

// Exists checks if a file exists on the WebDAV server
func (w *WebDAVClient) Exists(remotePath string) bool {
	_, err := w.client.Stat(remotePath)
	return err == nil
}

// Delete deletes a file from the WebDAV server
func (w *WebDAVClient) Delete(remotePath string) error {
	err := w.client.Remove(remotePath)
	if err != nil {
		return fmt.Errorf("failed to delete file from WebDAV: %w", err)
	}
	return nil
}

// List lists files in a directory on the WebDAV server
func (w *WebDAVClient) List(remotePath string) ([]string, error) {
	infos, err := w.client.ReadDir(remotePath)
	if err != nil {
		return nil, fmt.Errorf("failed to list WebDAV directory: %w", err)
	}

	var files []string
	for _, info := range infos {
		if !info.IsDir() {
			files = append(files, info.Name())
		}
	}

	return files, nil
}

// parseWebDAVURL parses a WebDAV URL and extracts components
func parseWebDAVURL(urlStr string) (endpoint, remotePath string, err error) {
	u, err := url.Parse(urlStr)
	if err != nil {
		return "", "", fmt.Errorf("failed to parse WebDAV URL: %w", err)
	}

	// Validate scheme
	if !isWebDAVScheme(u.Scheme) {
		return "", "", fmt.Errorf("unsupported scheme: %s (expected webdav or webdavs)", u.Scheme)
	}

	// Convert webdav/webdavs to http/https for the client
	scheme := "http"
	if u.Scheme == "webdavs" {
		scheme = "https"
	}

	endpoint = fmt.Sprintf("%s://%s", scheme, u.Host)
	if u.User != nil {
		endpoint = fmt.Sprintf("%s://%s@%s", scheme, u.User.String(), u.Host)
	}

	remotePath = u.Path
	if remotePath == "" {
		remotePath = "/"
	}

	return endpoint, remotePath, nil
}

// isWebDAVScheme checks if the scheme is a WebDAV scheme
func isWebDAVScheme(scheme string) bool {
	scheme = strings.ToLower(scheme)
	return scheme == "webdav" || scheme == "webdavs"
}