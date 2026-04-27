.PHONY: all build clean test lint deb

# Variables
BINARY_NAME=bucketsyncd
VERSION?=v0.3.0
ARCH?=amd64
PKG_VERSION=$(VERSION:v%=%)
BUILD_DIR=build
DEB_DIR=$(BUILD_DIR)/debian

# Default target
all: build

# Build the binary
build:
	mkdir -p $(BUILD_DIR)
	go build -ldflags="-s -w" -o "$(BUILD_DIR)/$(BINARY_NAME)" .

# Run tests
test:
	go test ./...

# Run linter
lint:
	golangci-lint run

# Create .deb package
deb: build
	@echo "Building .deb package for $(PKG_VERSION) on $(ARCH)"
	# Create Debian package structure
	mkdir -p $(DEB_DIR)/DEBIAN
	mkdir -p $(DEB_DIR)/usr/bin
	mkdir -p $(DEB_DIR)/usr/share/doc/bucketsyncd/examples
	# Copy binary
	cp "$(BUILD_DIR)/$(BINARY_NAME)" $(DEB_DIR)/usr/bin/$(BINARY_NAME)
	chmod 755 $(DEB_DIR)/usr/bin/$(BINARY_NAME)
	# Copy documentation
	cp README.md $(DEB_DIR)/usr/share/doc/bucketsyncd/
	cp CHANGELOG.md $(DEB_DIR)/usr/share/doc/bucketsyncd/
	cp -r example/* $(DEB_DIR)/usr/share/doc/bucketsyncd/examples/
	# Create control file
	echo "Package: $(BINARY_NAME)" > $(DEB_DIR)/DEBIAN/control
	echo "Version: $(PKG_VERSION)" >> $(DEB_DIR)/DEBIAN/control
	echo "Architecture: $(ARCH)" >> $(DEB_DIR)/DEBIAN/control
	echo "Maintainer: rossigee <ross@example.com>" >> $(DEB_DIR)/DEBIAN/control
	echo "Description: Bucket synchronisation service for automatic file sync" >> $(DEB_DIR)/DEBIAN/control
	echo " This application provides a bucket synchronisation service that automatically" >> $(DEB_DIR)/DEBIAN/control
	echo " downloads files to a local folder as they appear in a remote bucket, or uploads" >> $(DEB_DIR)/DEBIAN/control
	echo " files to a remote bucket or WebDAV server from a local folder." >> $(DEB_DIR)/DEBIAN/control
	echo "Depends: libc6" >> $(DEB_DIR)/DEBIAN/control
	echo "Section: utils" >> $(DEB_DIR)/DEBIAN/control
	echo "Priority: optional" >> $(DEB_DIR)/DEBIAN/control
	echo "Homepage: https://github.com/rossigee/bucketsyncd" >> $(DEB_DIR)/DEBIAN/control
	echo "License: MIT" >> $(DEB_DIR)/DEBIAN/control
	# Build .deb package
	cd $(BUILD_DIR) && dpkg-deb --build debian "$(BINARY_NAME)-$(PKG_VERSION)-$(ARCH).deb"
	@echo "Package created: $(BUILD_DIR)/$(BINARY_NAME)-$(PKG_VERSION)-$(ARCH).deb"

# Clean build artifacts
clean:
	rm -rf $(BUILD_DIR)