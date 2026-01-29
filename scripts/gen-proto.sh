#!/bin/bash
# Generate Go and Python code from protobuf definitions

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(dirname "$SCRIPT_DIR")"
PROTO_DIR="$PROJECT_ROOT/api/proto"
GO_OUT_DIR="$PROJECT_ROOT/api/gen"
PYTHON_OUT_DIR="$PROJECT_ROOT/sdk/python/sandbox_sdk/_gen"

# Create output directories
mkdir -p "$GO_OUT_DIR"
mkdir -p "$PYTHON_OUT_DIR"

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
    --go_out="$GO_OUT_DIR" \
    --go_opt=paths=source_relative \
    --go-grpc_out="$GO_OUT_DIR" \
    --go-grpc_opt=paths=source_relative \
    --grpc-gateway_out="$GO_OUT_DIR" \
    --grpc-gateway_opt=paths=source_relative \
    "$PROTO_DIR"/*.proto

echo "Go proto generation complete!"
echo "Generated files in: $GO_OUT_DIR"

# ============================================
# Python Code Generation
# ============================================

echo ""
echo "Generating Python code from proto files..."

PYTHON_SDK_DIR="$PROJECT_ROOT/sdk/python"
PYTHON_VENV="$PYTHON_SDK_DIR/.venv"

# Check if virtual environment exists, create if not
if [ ! -d "$PYTHON_VENV" ]; then
    echo "Creating Python virtual environment..."
    python3 -m venv "$PYTHON_VENV"
fi

# Activate virtual environment
source "$PYTHON_VENV/bin/activate"

# Check if grpcio-tools is available
if ! python -c "import grpc_tools.protoc" &> /dev/null; then
    echo "Installing grpcio-tools..."
    pip install grpcio grpcio-tools
fi

# Generate Python code
python -m grpc_tools.protoc \
    --proto_path="$PROTO_DIR" \
    --proto_path="$PROJECT_ROOT/third_party" \
    --python_out="$PYTHON_OUT_DIR" \
    --pyi_out="$PYTHON_OUT_DIR" \
    --grpc_python_out="$PYTHON_OUT_DIR" \
    "$PROTO_DIR"/*.proto

# Create __init__.py for the _gen package
touch "$PYTHON_OUT_DIR/__init__.py"

# Fix Python imports (replace absolute imports with relative imports)
echo "Fixing Python imports..."
for file in "$PYTHON_OUT_DIR"/*_pb2_grpc.py; do
    if [ -f "$file" ]; then
        # Replace 'import xxx_pb2' with 'from . import xxx_pb2'
        sed -i.bak 's/^import \(.*_pb2\)/from . import \1/' "$file"
        rm -f "${file}.bak"
    fi
done

for file in "$PYTHON_OUT_DIR"/*_pb2.py; do
    if [ -f "$file" ]; then
        # Replace 'import common_pb2' with 'from . import common_pb2'
        sed -i.bak 's/^import common_pb2/from . import common_pb2/' "$file"
        rm -f "${file}.bak"
    fi
done

echo "Python proto generation complete!"
echo "Generated files in: $PYTHON_OUT_DIR"
echo ""
echo "All proto generation complete!"
