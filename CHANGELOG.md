# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [v0.4.2] - 2026-05-16

### Fixed
- Outbound watcher goroutine now survives transient errors (file-open failures, missing credentials, MinIO client creation errors) via `continue` instead of `return`, preventing permanent loss of file-watch coverage
- Inbound AMQP reconnection loop now correctly exits the inner select loop on channel close via labeled `break`, eliminating a busy-spin that prevented reconnection
- `defer` inside event/message processing loops replaced with explicit resource cleanup, fixing file-handle and context leaks that accumulated until goroutine exit
- Missing `continue` after `url.Parse` failure in outbound handler (previously continued with a nil/partial URL)
- Watcher slice changed to pointer type (`[]*fsnotify.Watcher`) enabling test teardown to properly stop goroutines and eliminate data races

### Security
- Notification messages and titles are now sanitized before interpolation into macOS AppleScript and Windows PowerShell command strings, preventing command injection via crafted filenames
- Removed redundant `username` and `password` fields from `WebDAVClient` struct — credentials already held by the gowebdav client, extra copies increased in-memory exposure

### Changed
- Exponential backoff in retry helpers uses multiplication instead of `int → uint` bit-shift to avoid CWE-190 integer overflow (gosec G115)
- Noisy info-level log loop that printed all remote endpoints on every S3 event removed

### Dependencies
- `fsnotify` v1.9.0 → v1.10.1
- `klauspost/compress` v1.18.5 → v1.18.6
- `golang.org/x/crypto` v0.50.0 → v0.51.0
- `golang.org/x/net` v0.53.0 → v0.54.0
- `golang.org/x/sys` v0.43.0 → v0.44.0
- `golang.org/x/text` v0.36.0 → v0.37.0

## [v0.4.0] - 2026-04-29

### Added
- Configurable ignore patterns for outbound configurations to skip temporary/partial files
- Build information logging (version, build time, git commit) on startup
- Service account support for enhanced security with MinIO
- Comprehensive test coverage expansion with new unit tests
- Improved notification messages with file details and empty message prevention

### Changed
- Notifications now include full file paths and prevent empty notifications
- File watcher improved to handle Create/Write events and avoid premature watcher closure
- Endpoint matching fixed to use u.Host instead of u.Hostname() for ports
- Error handling enhanced with better logging and retry mechanisms

### Fixed
- Watcher lifecycle management to prevent event loss
- Configurable file filtering to ignore unwanted files
- Notification system robustness

### Security
- Service account credentials with restricted permissions
- Improved secure credential handling

## [v0.3.2] - 2026-04-29

### Fixed
- AMQP connection reconnection on timeout/failure (closes #1)
- Improved error handling and logging for connection states
- Prevented goroutine leaks in message processing

## [v0.3.1] - 2026-04-28

### Added
- `.deb` packaging for Debian/Ubuntu with complete documentation
- Makefile with `make deb` target for local packaging
- Example configuration and systemd service included in package
- CHANGELOG.md included in package documentation

### Changed
- Improved CI workflow to build .deb packages for Linux amd64/arm64
- Enhanced package metadata and file structure

## [v0.3.0] - 2026-04-27

### Added
- Version flag (`-version`) to display current version
- Retry logic with exponential backoff for MinIO operations (up to 3 retries)
- Timeouts (30 seconds) for all MinIO upload/download operations
- Structured JSON parsing for S3 event messages to prevent panics
- Graceful error handling for AMQP message processing with proper nack/ack

### Changed
- Updated yaml dependency from v2 to v3 for improved performance
- Replaced `log.Fatal()` with graceful error logging in file watching and MinIO operations
- Improved AMQP consumer tag from "backupsyncd" to "bucketsyncd"
- Enhanced logging with better error context and redaction

### Fixed
- Potential panics from unsafe type assertions in JSON parsing
- Unchecked AMQP nack errors
- Service crashes on recoverable errors

### Security
- Safer handling of configuration and message parsing
- Proper error handling to prevent information leakage

## [v0.2.0] - 2026-03-01

### Added
- WebDAV support for outbound file uploads
- AMQP inbound processing for S3 event notifications
- File watching with glob patterns
- Docker build support
- Comprehensive test suite

### Changed
- Improved logging with structured fields
- Enhanced CI/CD pipeline with security scanning

## [v0.1.0] - 2026-01-01

### Added
- Initial release with S3 outbound syncing
- Basic file watching and upload functionality
- YAML configuration support
- Logging with logrus