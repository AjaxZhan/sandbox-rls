#!/bin/bash
# Build release binaries for multiple platforms
# Usage: ./scripts/build-release.sh [version]

set -e

VERSION=${1:-"dev"}
OUTPUT_DIR="dist"
BINARY_NAME="agentfense-server"

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

echo -e "${GREEN}Building AgentFense ${VERSION}${NC}"

# Clean previous builds
rm -rf "$OUTPUT_DIR"
mkdir -p "$OUTPUT_DIR"

# Platforms to build for
PLATFORMS=(
    "linux/amd64"
    "linux/arm64"
    "darwin/amd64"
    "darwin/arm64"
)

# Build for each platform
for PLATFORM in "${PLATFORMS[@]}"; do
    GOOS="${PLATFORM%/*}"
    GOARCH="${PLATFORM#*/}"
    OUTPUT_NAME="${BINARY_NAME}-${GOOS}-${GOARCH}"
    
    if [ "$GOOS" = "windows" ]; then
        OUTPUT_NAME="${OUTPUT_NAME}.exe"
    fi
    
    echo -e "${YELLOW}Building for ${GOOS}/${GOARCH}...${NC}"
    
    # Build with version info
    GOOS=$GOOS GOARCH=$GOARCH go build \
        -ldflags="-s -w -X main.Version=${VERSION}" \
        -o "${OUTPUT_DIR}/${OUTPUT_NAME}" \
        ./cmd/agentfense-server
    
    if [ $? -eq 0 ]; then
        echo -e "${GREEN}✓ Built ${OUTPUT_NAME}${NC}"
    else
        echo -e "${RED}✗ Failed to build ${OUTPUT_NAME}${NC}"
        exit 1
    fi
done

# Create archives
echo -e "${YELLOW}Creating archives...${NC}"
cd "$OUTPUT_DIR"

for PLATFORM in "${PLATFORMS[@]}"; do
    GOOS="${PLATFORM%/*}"
    GOARCH="${PLATFORM#*/}"
    OUTPUT_NAME="${BINARY_NAME}-${GOOS}-${GOARCH}"
    ARCHIVE_NAME="${BINARY_NAME}-${VERSION}-${GOOS}-${GOARCH}"
    
    if [ "$GOOS" = "windows" ]; then
        zip "${ARCHIVE_NAME}.zip" "${OUTPUT_NAME}.exe"
        rm "${OUTPUT_NAME}.exe"
    else
        tar -czvf "${ARCHIVE_NAME}.tar.gz" "${OUTPUT_NAME}"
        rm "${OUTPUT_NAME}"
    fi
    
    echo -e "${GREEN}✓ Created ${ARCHIVE_NAME}${NC}"
done

# Generate checksums
echo -e "${YELLOW}Generating checksums...${NC}"
sha256sum *.tar.gz *.zip 2>/dev/null > checksums.txt || sha256sum *.tar.gz > checksums.txt
echo -e "${GREEN}✓ Generated checksums.txt${NC}"

cd ..

echo ""
echo -e "${GREEN}Release build complete!${NC}"
echo "Files in ${OUTPUT_DIR}/:"
ls -la "${OUTPUT_DIR}/"
