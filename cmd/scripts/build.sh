#!/bin/bash

# ROCK Pi Penta SATA HAT Controller (Go) - Build Script
# Supports cross-compilation for ARM64 and ARM32

set -e

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

# Configuration
BINARY_NAME="rocki-penta"
VERSION=$(git describe --tags --always --dirty 2>/dev/null || echo "v0.1.0-dev")
BUILD_TIME=$(date -u +"%Y-%m-%dT%H:%M:%SZ")
GO_VERSION=$(go version | cut -d' ' -f3)

# Function to print colored output
log_info() {
    echo -e "${GREEN}[INFO]${NC} $1"
}

log_warn() {
    echo -e "${YELLOW}[WARN]${NC} $1"
}

log_error() {
    echo -e "${RED}[ERROR]${NC} $1"
}

# Function to check if Go is installed
check_go() {
    if ! command -v go &> /dev/null; then
        log_error "Go is not installed. Please install Go 1.21 or later."
        exit 1
    fi
    
    local go_version=$(go version | cut -d' ' -f3 | sed 's/go//')
    local required_version="1.21"
    
    if ! printf '%s\n' "$required_version" "$go_version" | sort -C -V; then
        log_error "Go version $go_version is too old. Please install Go $required_version or later."
        exit 1
    fi
    
    log_info "Using Go version: $go_version"
}

# Function to clean previous builds
clean() {
    log_info "Cleaning previous builds..."
    rm -rf build/
    mkdir -p build/
}

# Function to download dependencies
download_deps() {
    log_info "Downloading dependencies..."
    go mod download
    go mod tidy
}

# Function to run tests
run_tests() {
    log_info "Running tests..."
    go test -v ./...
}

# Function to build for a specific architecture
build_arch() {
    local os="$1"
    local arch="$2"
    local output_dir="build/${os}-${arch}"
    local output_file="${output_dir}/${BINARY_NAME}"
    
    if [[ "$os" == "windows" ]]; then
        output_file="${output_file}.exe"
    fi
    
    log_info "Building for $os/$arch..."
    
    # Set build flags
    local ldflags="-X main.version=${VERSION} -X main.buildTime=${BUILD_TIME} -X main.goVersion=${GO_VERSION}"
    
    # Create output directory
    mkdir -p "$output_dir"
    
    # Build
    CGO_ENABLED=0 GOOS="$os" GOARCH="$arch" go build \
        -ldflags "$ldflags" \
        -o "$output_file" \
        ./cmd/rocki-penta/
    
    # Copy additional files
    cp -r configs/ "$output_dir/"
    cp -r scripts/ "$output_dir/"
    cp README.md "$output_dir/" 2>/dev/null || true
    
    log_info "Built successfully: $output_file"
}

# Function to create archives
create_archives() {
    log_info "Creating archives..."
    
    cd build/
    
    for dir in */; do
        if [[ -d "$dir" ]]; then
            local archive_name="${dir%/}"
            log_info "Creating archive: ${archive_name}.tar.gz"
            tar -czf "${archive_name}.tar.gz" "$dir"
        fi
    done
    
    cd ..
    
    log_info "Archives created in build/ directory"
}

# Function to build for Raspberry Pi (ARM64 and ARM32)
build_raspberry_pi() {
    log_info "Building for Raspberry Pi..."
    
    # ARM64 (aarch64) - for Raspberry Pi 4/5 64-bit
    build_arch "linux" "arm64"
    
    # ARM32 (armv7) - for Raspberry Pi 4 32-bit and older models
    build_arch "linux" "arm"
}

# Function to build for all supported architectures
build_all() {
    log_info "Building for all supported architectures..."
    
    # Raspberry Pi
    build_raspberry_pi
    
    # x86_64 (for testing on PC)
    build_arch "linux" "amd64"
}

# Function to install locally
install_local() {
    log_info "Installing locally..."
    
    local local_arch=$(uname -m)
    local go_arch
    
    case "$local_arch" in
        "x86_64")
            go_arch="amd64"
            ;;
        "aarch64")
            go_arch="arm64"
            ;;
        "armv7l")
            go_arch="arm"
            ;;
        *)
            log_error "Unsupported architecture: $local_arch"
            exit 1
            ;;
    esac
    
    local binary_path="build/linux-${go_arch}/${BINARY_NAME}"
    
    if [[ ! -f "$binary_path" ]]; then
        log_error "Binary not found: $binary_path"
        log_info "Please run 'build' command first"
        exit 1
    fi
    
    # Install using the install script
    sudo ./scripts/install.sh "$binary_path" "configs/rockpi-penta.conf" "configs/systemd/rocki-penta.service"
}

# Function to show usage
usage() {
    echo "Usage: $0 [command]"
    echo ""
    echo "Commands:"
    echo "  build       Build for Raspberry Pi (ARM64 and ARM32)"
    echo "  build-all   Build for all supported architectures"
    echo "  test        Run tests"
    echo "  clean       Clean build artifacts"
    echo "  install     Install locally (requires sudo)"
    echo "  help        Show this help message"
    echo ""
    echo "Environment variables:"
    echo "  VERSION     Override version string"
    echo "  CGO_ENABLED Override CGO setting (default: 0)"
    echo ""
    echo "Examples:"
    echo "  $0 build                    # Build for Raspberry Pi"
    echo "  $0 build-all               # Build for all architectures"
    echo "  VERSION=v1.0.0 $0 build    # Build with custom version"
}

# Main function
main() {
    local command="${1:-build}"
    
    case "$command" in
        "build")
            check_go
            clean
            download_deps
            build_raspberry_pi
            create_archives
            ;;
        "build-all")
            check_go
            clean
            download_deps
            build_all
            create_archives
            ;;
        "test")
            check_go
            run_tests
            ;;
        "clean")
            clean
            ;;
        "install")
            install_local
            ;;
        "help"|"-h"|"--help")
            usage
            ;;
        *)
            log_error "Unknown command: $command"
            usage
            exit 1
            ;;
    esac
}

# Run main function
main "$@" 