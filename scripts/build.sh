#!/bin/bash

set -e

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

# Configuration
BINARY_NAME="rockpi-penta"
BUILD_DIR="build"
INSTALL_PATH="/usr/local/bin"

# Function to print colored output
print_info() {
    echo -e "${GREEN}[INFO]${NC} $1"
}

print_warning() {
    echo -e "${YELLOW}[WARNING]${NC} $1"
}

print_error() {
    echo -e "${RED}[ERROR]${NC} $1"
}

# Check if Go is installed
if ! command -v go &> /dev/null; then
    print_error "Go is not installed. Please install Go 1.21 or later."
    exit 1
fi

# Check Go version
GO_VERSION=$(go version | awk '{print $3}' | sed 's/go//')
REQUIRED_VERSION="1.21"

if ! printf '%s\n%s\n' "$REQUIRED_VERSION" "$GO_VERSION" | sort -V | head -n1 | grep -q "$REQUIRED_VERSION"; then
    print_error "Go version $GO_VERSION is too old. Please install Go $REQUIRED_VERSION or later."
    exit 1
fi

print_info "Building RockPi Penta service..."

# Create build directory
mkdir -p "$BUILD_DIR"

# Build the application
print_info "Compiling Go application..."
CGO_ENABLED=1 go build -o "$BUILD_DIR/$BINARY_NAME" -ldflags "-s -w" ./cmd/

if [ $? -eq 0 ]; then
    print_info "Build successful! Binary created at $BUILD_DIR/$BINARY_NAME"
else
    print_error "Build failed!"
    exit 1
fi

# Check if installation was requested
INSTALL=false
if [ "$1" = "install" ] || [ "$1" = "-i" ] || [ "$1" = "--install" ]; then
    INSTALL=true
fi

if [ "$INSTALL" = true ]; then
    # Check if running as root for installation
    if [ "$EUID" -ne 0 ]; then
        print_error "Installation requires root privileges. Please run with sudo:"
        print_info "sudo $0 install"
        exit 1
    fi
    
    print_info "Installing binary to $INSTALL_PATH..."
    
    # Stop service if running
    if systemctl is-active --quiet rockpi-penta; then
        print_info "Stopping existing service..."
        systemctl stop rockpi-penta
        RESTART_SERVICE=true
    fi
    
    # Install binary
    cp "$BUILD_DIR/$BINARY_NAME" "$INSTALL_PATH/"
    chmod +x "$INSTALL_PATH/$BINARY_NAME"
    
    print_info "Binary installed successfully at $INSTALL_PATH/$BINARY_NAME"
    
    # Restart service if it was running
    if [ "$RESTART_SERVICE" = true ]; then
        print_info "Restarting service..."
        systemctl start rockpi-penta
        print_info "Service restarted"
    fi
else
    print_info "To install the binary, run: sudo $0 install"
    print_info "To test the application, run: sudo ./$BUILD_DIR/$BINARY_NAME"
fi

print_info "Build completed successfully!" 