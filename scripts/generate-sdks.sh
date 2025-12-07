#!/bin/bash
# generate-sdks.sh - Generate gRPC client/server SDKs from proto files
#
# This script generates Go and TypeScript SDKs from the core proto definitions.
# The core proto contains public API definitions (Geography, Issues, Polls, etc.)
#
# Prerequisites:
#   - protoc (Protocol Buffer Compiler)
#   - protoc-gen-go (Go plugin)
#   - protoc-gen-go-grpc (Go gRPC plugin)
#   - protoc-gen-ts (TypeScript plugin)
#   - buf (optional, for better dependency management)

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
ROOT_DIR="$(dirname "$SCRIPT_DIR")"
PROTO_DIR="$ROOT_DIR/proto"
GEN_DIR="$ROOT_DIR/gen"

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

echo -e "${GREEN}======================================${NC}"
echo -e "${GREEN}  Politic Core SDK Generator${NC}"
echo -e "${GREEN}======================================${NC}"

# Check prerequisites
check_prereqs() {
    echo -e "\n${YELLOW}Checking prerequisites...${NC}"

    local missing=()

    if ! command -v protoc &> /dev/null; then
        missing+=("protoc")
    else
        echo "  ✓ protoc $(protoc --version | head -n1)"
    fi

    if ! command -v protoc-gen-go &> /dev/null; then
        missing+=("protoc-gen-go")
    else
        echo "  ✓ protoc-gen-go"
    fi

    if ! command -v protoc-gen-go-grpc &> /dev/null; then
        missing+=("protoc-gen-go-grpc")
    else
        echo "  ✓ protoc-gen-go-grpc"
    fi

    if [ ${#missing[@]} -ne 0 ]; then
        echo -e "\n${RED}Missing prerequisites: ${missing[*]}${NC}"
        echo -e "\nInstall with:"
        echo "  # Protocol Buffer Compiler"
        echo "  brew install protobuf  # macOS"
        echo "  # or: apt install protobuf-compiler  # Ubuntu"
        echo ""
        echo "  # Go plugins"
        echo "  go install google.golang.org/protobuf/cmd/protoc-gen-go@latest"
        echo "  go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@latest"
        echo ""
        echo "  # TypeScript plugin (optional)"
        echo "  npm install -g @protobuf-ts/plugin"
        exit 1
    fi
}

# Create output directories
create_dirs() {
    echo -e "\n${YELLOW}Creating output directories...${NC}"

    mkdir -p "$GEN_DIR/go/politic/core/v1"
    mkdir -p "$GEN_DIR/ts/politic/core/v1"

    echo "  ✓ $GEN_DIR/go/"
    echo "  ✓ $GEN_DIR/ts/"
}

# Generate Go SDK
generate_go() {
    echo -e "\n${YELLOW}Generating Go SDK...${NC}"

    protoc \
        --proto_path="$PROTO_DIR" \
        --go_out="$GEN_DIR/go" \
        --go_opt=paths=source_relative \
        --go-grpc_out="$GEN_DIR/go" \
        --go-grpc_opt=paths=source_relative \
        "$PROTO_DIR/politic.proto"

    echo "  ✓ Generated Go types and gRPC client/server"
    echo "  → $GEN_DIR/go/politic.pb.go"
    echo "  → $GEN_DIR/go/politic_grpc.pb.go"
}

# Generate TypeScript SDK
generate_ts() {
    echo -e "\n${YELLOW}Generating TypeScript SDK...${NC}"

    if ! command -v protoc-gen-ts &> /dev/null; then
        echo -e "  ${YELLOW}⚠ protoc-gen-ts not found, skipping TypeScript generation${NC}"
        echo "  Install with: npm install -g @protobuf-ts/plugin"
        return
    fi

    protoc \
        --proto_path="$PROTO_DIR" \
        --ts_out="$GEN_DIR/ts" \
        "$PROTO_DIR/politic.proto"

    echo "  ✓ Generated TypeScript types and client"
    echo "  → $GEN_DIR/ts/politic.ts"
}

# Generate JSON Schema (for API documentation)
generate_json_schema() {
    echo -e "\n${YELLOW}Generating JSON Schema...${NC}"

    if ! command -v protoc-gen-jsonschema &> /dev/null; then
        echo -e "  ${YELLOW}⚠ protoc-gen-jsonschema not found, skipping JSON Schema generation${NC}"
        echo "  Install with: go install github.com/chrusty/protoc-gen-jsonschema@latest"
        return
    fi

    mkdir -p "$GEN_DIR/jsonschema"

    protoc \
        --proto_path="$PROTO_DIR" \
        --jsonschema_out="$GEN_DIR/jsonschema" \
        "$PROTO_DIR/politic.proto"

    echo "  ✓ Generated JSON Schema"
    echo "  → $GEN_DIR/jsonschema/"
}

# Update go.mod with generated code
update_go_mod() {
    echo -e "\n${YELLOW}Updating Go module...${NC}"

    cd "$ROOT_DIR"
    go mod tidy

    echo "  ✓ Updated go.mod"
}

# Print usage
print_usage() {
    echo "Usage: $0 [OPTIONS]"
    echo ""
    echo "Options:"
    echo "  --go         Generate Go SDK only"
    echo "  --ts         Generate TypeScript SDK only"
    echo "  --json       Generate JSON Schema only"
    echo "  --all        Generate all SDKs (default)"
    echo "  --check      Check prerequisites only"
    echo "  --clean      Remove generated files"
    echo "  -h, --help   Show this help"
}

# Clean generated files
clean() {
    echo -e "\n${YELLOW}Cleaning generated files...${NC}"
    rm -rf "$GEN_DIR"
    echo "  ✓ Removed $GEN_DIR"
}

# Main
main() {
    local gen_go=false
    local gen_ts=false
    local gen_json=false

    # Parse arguments
    if [ $# -eq 0 ]; then
        gen_go=true
        gen_ts=true
        gen_json=true
    else
        for arg in "$@"; do
            case $arg in
                --go)
                    gen_go=true
                    ;;
                --ts)
                    gen_ts=true
                    ;;
                --json)
                    gen_json=true
                    ;;
                --all)
                    gen_go=true
                    gen_ts=true
                    gen_json=true
                    ;;
                --check)
                    check_prereqs
                    exit 0
                    ;;
                --clean)
                    clean
                    exit 0
                    ;;
                -h|--help)
                    print_usage
                    exit 0
                    ;;
                *)
                    echo -e "${RED}Unknown option: $arg${NC}"
                    print_usage
                    exit 1
                    ;;
            esac
        done
    fi

    check_prereqs
    create_dirs

    if [ "$gen_go" = true ]; then
        generate_go
    fi

    if [ "$gen_ts" = true ]; then
        generate_ts
    fi

    if [ "$gen_json" = true ]; then
        generate_json_schema
    fi

    if [ "$gen_go" = true ]; then
        update_go_mod
    fi

    echo -e "\n${GREEN}======================================${NC}"
    echo -e "${GREEN}  SDK generation complete!${NC}"
    echo -e "${GREEN}======================================${NC}"
    echo ""
    echo "Generated files:"
    find "$GEN_DIR" -type f -name "*.go" -o -name "*.ts" -o -name "*.json" 2>/dev/null | head -20 || true
}

main "$@"
