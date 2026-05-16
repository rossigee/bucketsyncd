#!/bin/bash

set -e

if [ $# -ne 1 ]; then
    echo "Usage: $0 <version>"
    echo "Example: $0 0.4.1"
    exit 1
fi

VERSION=$1

echo "Updating version to $VERSION..."

# Update main.go
sed -i "s/version = \".*\"/version = \"$VERSION\"/" main.go

# Update Makefile
sed -i "s/VERSION := .*/VERSION := $VERSION/" Makefile

# Update README.md title
sed -i "s/# bucketsyncd v.*/# bucketsyncd v$VERSION/" README.md

# Update README.md deb install example
sed -i "s/bucketsyncd_.*_amd64.deb/bucketsyncd_${VERSION}_amd64.deb/" README.md

# Update debian changelog
DATE=$(date -R)
sed -i "1i\\
bucketsyncd ($VERSION-1) unstable; urgency=medium\\
\\
  * Updated to version $VERSION.\\
\\
 -- Ross Golder <ross@golder.org>  $DATE\\
\\
" debian/changelog

echo "Version updated to $VERSION. Please review changes and add changelog details."
echo "Then run: git add . && git commit -m \"Release v$VERSION\" && git tag v$VERSION && git push origin master && git push origin v$VERSION"
