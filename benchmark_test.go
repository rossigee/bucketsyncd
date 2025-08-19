package main

import (
	"encoding/json"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ryanuber/go-glob"
)

func BenchmarkConfigReading(b *testing.B) {
	// Create a test configuration file
	tmpDir := b.TempDir()
	configFile := filepath.Join(tmpDir, "benchmark-config.yaml")

	configContent := `
log_level: "info"
log_json: false
outbound:
  - name: "benchmark-outbound"
    description: "Benchmark test outbound"
    source: "/tmp/benchmark/*"
    destination: "s3://benchmark-bucket/uploads/"
    sensitive: false
inbound:
  - name: "benchmark-inbound"
    description: "Benchmark test inbound"
    source: "amqp://user:pass@localhost:5672/"
    exchange: "benchmark-exchange"
    queue: "benchmark-queue"
    remote: "benchmark-remote"
    destination: "/tmp/benchmark-downloads"
remotes:
  - name: "benchmark-remote"
    endpoint: "localhost:9000"
    accessKey: "benchmark-access"
    secretKey: "benchmark-secret"
`

	err := os.WriteFile(configFile, []byte(configContent), 0600)
	if err != nil {
		b.Fatalf("Failed to create benchmark config: %v", err)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		err := readConfig(configFile)
		if err != nil {
			b.Fatalf("Failed to read config: %v", err)
		}
	}
}

func BenchmarkLargeConfigReading(b *testing.B) {
	// Create a large configuration file
	tmpDir := b.TempDir()
	configFile := filepath.Join(tmpDir, "large-benchmark-config.yaml")

	configContent := `
log_level: "info"
log_json: false
outbound:
`

	// Add 100 outbound configurations
	for i := 0; i < 100; i++ {
		configContent += `  - name: "outbound-` + strings.Repeat("x", i%10+1) + `"
    description: "Outbound description ` + strings.Repeat("desc", i%5+1) + `"
    source: "/tmp/source` + strings.Repeat("path", i%3+1) + `/*"
    destination: "s3://bucket` + strings.Repeat("name", i%4+1) + `/path/"
    sensitive: false
`
	}

	configContent += `inbound:
`

	// Add 50 inbound configurations
	for i := 0; i < 50; i++ {
		configContent += `  - name: "inbound-` + strings.Repeat("x", i%10+1) + `"
    description: "Inbound description ` + strings.Repeat("desc", i%5+1) + `"
    source: "amqp://user:pass@host` + strings.Repeat("host", i%3+1) + `:5672/"
    exchange: "exchange` + strings.Repeat("ex", i%4+1) + `"
    queue: "queue` + strings.Repeat("q", i%3+1) + `"
    remote: "remote` + strings.Repeat("r", i%5+1) + `"
    destination: "/tmp/dest` + strings.Repeat("dest", i%2+1) + `"
`
	}

	configContent += `remotes:
`

	// Add 25 remote configurations
	for i := 0; i < 25; i++ {
		configContent += `  - name: "remote` + strings.Repeat("r", i%8+1) + `"
    endpoint: "host` + strings.Repeat("endpoint", i%3+1) + `:9000"
    accessKey: "key` + strings.Repeat("accesskey", i%4+1) + `"
    secretKey: "secret` + strings.Repeat("secretkey", i%5+1) + `"
`
	}

	err := os.WriteFile(configFile, []byte(configContent), 0600)
	if err != nil {
		b.Fatalf("Failed to create large benchmark config: %v", err)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		err := readConfig(configFile)
		if err != nil {
			b.Fatalf("Failed to read large config: %v", err)
		}
	}
}

func BenchmarkJSONUnmarshaling(b *testing.B) {
	// Benchmark JSON unmarshaling for S3 event messages
	eventMessage := map[string]interface{}{
		"EventName": "s3:ObjectCreated:Put",
		"Records": []interface{}{
			map[string]interface{}{
				"s3": map[string]interface{}{
					"bucket": map[string]interface{}{
						"name": "benchmark-bucket-with-very-long-name-for-testing",
					},
					"object": map[string]interface{}{
						"key":  "very/long/path/to/file/with/many/segments/benchmark-file-with-long-name.txt",
						"size": float64(1048576), // 1MB
					},
				},
			},
		},
	}

	jsonData, err := json.Marshal(eventMessage)
	if err != nil {
		b.Fatalf("Failed to marshal test message: %v", err)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		var parsedMessage map[string]interface{}
		err := json.Unmarshal(jsonData, &parsedMessage)
		if err != nil {
			b.Fatalf("Failed to unmarshal message: %v", err)
		}
	}
}

func BenchmarkLargeJSONUnmarshaling(b *testing.B) {
	// Benchmark JSON unmarshaling with multiple records
	records := make([]interface{}, 100)
	for i := 0; i < 100; i++ {
		records[i] = map[string]interface{}{
			"s3": map[string]interface{}{
				"bucket": map[string]interface{}{
					"name": "benchmark-bucket-" + strings.Repeat("name", i%10+1),
				},
				"object": map[string]interface{}{
					"key":  "path/to/file-" + strings.Repeat("filename", i%5+1) + ".txt",
					"size": float64(1024 * (i%100 + 1)),
				},
			},
		}
	}

	eventMessage := map[string]interface{}{
		"EventName": "s3:ObjectCreated:Put",
		"Records":   records,
	}

	jsonData, err := json.Marshal(eventMessage)
	if err != nil {
		b.Fatalf("Failed to marshal large test message: %v", err)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		var parsedMessage map[string]interface{}
		err := json.Unmarshal(jsonData, &parsedMessage)
		if err != nil {
			b.Fatalf("Failed to unmarshal large message: %v", err)
		}
	}
}

func BenchmarkGlobMatching(b *testing.B) {
	patterns := []string{"*", "*.txt", "*.log", "data-*", "backup-*.sql", "temp_*.tmp"}
	filenames := []string{
		"test.txt", "data.log", "backup.sql", "temp.tmp",
		"data-file.txt", "backup-20240101.sql", "temp_12345.tmp",
		"very-long-filename-with-many-segments.txt",
		"file.with.multiple.dots.log",
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		pattern := patterns[i%len(patterns)]
		filename := filenames[i%len(filenames)]
		_ = glob.Glob(pattern, filename)
	}
}

func BenchmarkURLParsing(b *testing.B) {
	urls := []string{
		"s3://test-bucket/uploads/",
		"https://s3.amazonaws.com/my-bucket/data/logs/",
		"http://localhost:9000/local-bucket/files/",
		"s3://very-long-bucket-name-for-testing/very/long/path/to/files/",
		"https://custom.s3.endpoint.com/bucket/nested/deep/path/structure/",
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		urlStr := urls[i%len(urls)]
		_, err := url.Parse(urlStr)
		if err != nil {
			b.Fatalf("Failed to parse URL: %v", err)
		}
	}
}

func BenchmarkPathTokenization(b *testing.B) {
	paths := []string{
		"/bucket/path",
		"/bucket/very/long/path/with/many/segments",
		"/bucket",
		"/bucket/data/logs/2024/01/01/file.log",
		"/very-long-bucket-name/nested/deep/directory/structure/file.txt",
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		path := paths[i%len(paths)]
		tokens := strings.Split(path, "/")
		_ = len(tokens)
	}
}

func BenchmarkRemoteMatching(b *testing.B) {
	// Set up test configuration with many remotes
	remotes := make([]Remote, 100)
	for i := 0; i < 100; i++ {
		remotes[i] = Remote{
			Name:      "remote-" + strings.Repeat("name", i%10+1),
			Endpoint:  "endpoint-" + strings.Repeat("host", i%5+1) + ".com",
			AccessKey: "access-" + strings.Repeat("key", i%3+1),
			SecretKey: "secret-" + strings.Repeat("secret", i%4+1),
		}
	}

	config = Config{
		Remotes: remotes,
	}

	endpoints := []string{
		"endpoint-host.com",
		"endpoint-hosthost.com",
		"endpoint-hosthosthost.com",
		"endpoint-hosthost.com",
		"unknown-endpoint.com",
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		endpoint := endpoints[i%len(endpoints)]
		found := false
		for _, remote := range config.Remotes {
			if remote.Endpoint == endpoint {
				found = true
				break
			}
		}
		_ = found
	}
}

func BenchmarkFilePathOperations(b *testing.B) {
	sources := []string{
		"/tmp/test/*",
		"/var/log/*.log",
		"/data/files/backup-*",
		"/uploads/images/*.jpg",
		"/very/long/path/to/files/with/pattern-*.txt",
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		source := sources[i%len(sources)]
		_ = filepath.Dir(source)
		_ = filepath.Base(source)
	}
}

func BenchmarkStringOperations(b *testing.B) {
	paths := []string{
		"/bucket/uploads",
		"/bucket/data/logs/2024",
		"/very-long-bucket-name/nested/deep/path",
		"/bucket/files/images/thumbnails/large",
	}
	filenames := []string{
		"test.txt",
		"data-file-with-long-name.log",
		"backup-20240101-full.sql",
		"image-thumbnail-large-format.jpg",
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		path := paths[i%len(paths)]
		filename := filenames[i%len(filenames)]
		tokens := strings.Split(path, "/")
		if len(tokens) >= 2 {
			_ = strings.Join(tokens[2:], "/") + "/" + filename
		}
	}
}

func BenchmarkConcurrentConfigAccess(b *testing.B) {
	// Benchmark concurrent access to global config
	config = Config{
		LogLevel: "info",
		LogJSON:  false,
		Outbound: []Outbound{
			{Name: "test1", Source: "/tmp/1/*", Destination: "s3://bucket1/"},
			{Name: "test2", Source: "/tmp/2/*", Destination: "s3://bucket2/"},
		},
		Inbound: []Inbound{
			{Name: "in1", Remote: "remote1", Destination: "/tmp/in1"},
			{Name: "in2", Remote: "remote2", Destination: "/tmp/in2"},
		},
		Remotes: []Remote{
			{Name: "remote1", Endpoint: "host1:9000"},
			{Name: "remote2", Endpoint: "host2:9000"},
		},
	}

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			// Simulate concurrent access to config
			_ = config.LogLevel
			_ = len(config.Outbound)
			_ = len(config.Inbound)
			_ = len(config.Remotes)
		}
	})
}
