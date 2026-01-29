#!/bin/bash
# Generate Go code from protobuf definitions

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(dirname "$SCRIPT_DIR")"
PROTO_DIR="$PROJECT_ROOT/api/proto"
OUT_DIR="$PROJECT_ROOT/api/gen"

# Create output directory
mkdir -p "$OUT_DIR"

# Check if protoc is installed
if ! command -v protoc &> /dev/null; then
    echo "Error: protoc is not installed"
    echo "Please install protobuf compiler:"
    echo "  macOS: brew install protobuf"
    echo "  Linux: apt-get install protobuf-compiler"
    exit 1
fi

# Check if Go plugins are installed
if ! command -v protoc-gen-go &> /dev/null; then
    echo "Installing protoc-gen-go..."
    go install google.golang.org/protobuf/cmd/protoc-gen-go@latest
fi

if ! command -v protoc-gen-go-grpc &> /dev/null; then
    echo "Installing protoc-gen-go-grpc..."
    go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@latest
fi

if ! command -v protoc-gen-grpc-gateway &> /dev/null; then
    echo "Installing protoc-gen-grpc-gateway..."
    go install github.com/grpc-ecosystem/grpc-gateway/v2/protoc-gen-grpc-gateway@latest
fi

# Download google/api annotations if needed
GOOGLE_API_DIR="$PROJECT_ROOT/third_party/google/api"
if [ ! -f "$GOOGLE_API_DIR/annotations.proto" ]; then
    echo "Downloading google/api proto files..."
    mkdir -p "$GOOGLE_API_DIR"
    curl -sSL -o "$GOOGLE_API_DIR/annotations.proto" \
        "https://raw.githubusercontent.com/googleapis/googleapis/master/google/api/annotations.proto"
    curl -sSL -o "$GOOGLE_API_DIR/http.proto" \
        "https://raw.githubusercontent.com/googleapis/googleapis/master/google/api/http.proto"
fi

echo "Generating Go code from proto files..."

protoc \
    --proto_path="$PROTO_DIR" \
    --proto_path="$PROJECT_ROOT/third_party" \
    --go_out="$OUT_DIR" \
    --go_opt=paths=source_relative \
    --go-grpc_out="$OUT_DIR" \
    --go-grpc_opt=paths=source_relative \
    --grpc-gateway_out="$OUT_DIR" \
    --grpc-gateway_opt=paths=source_relative \
    "$PROTO_DIR"/*.proto

echo "Proto generation complete!"
echo "Generated files in: $OUT_DIR"
