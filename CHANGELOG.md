# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

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