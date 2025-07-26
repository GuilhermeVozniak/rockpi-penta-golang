#!/bin/bash

set -e

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

# Configuration
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(dirname "$SCRIPT_DIR")"
BASE_LINUX_SETUP_VERSION="latest"
TEMP_DIR="/tmp/rockpi-penta-install"

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

# Check if running as root
if [ "$EUID" -ne 0 ]; then
    print_error "This script must be run as root. Please use sudo:"
    print_info "sudo $0"
    exit 1
fi

print_info "RockPi Penta Golang - Enhanced Installation"
print_info "Using base-linux-setup for intelligent dependency management"
echo

# Create temporary directory
mkdir -p "$TEMP_DIR"
cd "$TEMP_DIR"

# Detect architecture
ARCH=$(uname -m)
case $ARCH in
    "x86_64") BIN_ARCH="amd64" ;;
    "aarch64"|"arm64") BIN_ARCH="arm64" ;;
    "armv7l"|"armv6l") BIN_ARCH="arm" ;;
    *) print_error "Unsupported architecture: $ARCH"; exit 1 ;;
esac

print_info "Detected architecture: $ARCH (binary: $BIN_ARCH)"

# Download base-linux-setup binary
DOWNLOAD_URL="https://github.com/GuilhermeVozniak/base-linux-setup/releases/latest/download/base-linux-setup-linux-$BIN_ARCH"
BINARY_PATH="$TEMP_DIR/base-linux-setup"

print_info "Downloading base-linux-setup binary..."
if ! wget -q "$DOWNLOAD_URL" -O "$BINARY_PATH"; then
    print_error "Failed to download base-linux-setup binary"
    print_info "Please check your internet connection and try again"
    exit 1
fi

# Make binary executable
chmod +x "$BINARY_PATH"

print_info "Base-linux-setup binary downloaded successfully"

# Verify the binary works
if ! "$BINARY_PATH" --version >/dev/null 2>&1; then
    print_error "Downloaded binary is not working properly"
    exit 1
fi

# Run base-linux-setup with our configuration
print_info "Running base-linux-setup with RockPi Penta configuration..."
echo "This will install dependencies with intelligent checks and proper ordering..."
echo

CONFIG_FILE="$PROJECT_ROOT/scripts/rockpi-penta-setup.json"

if [ ! -f "$CONFIG_FILE" ]; then
    print_error "Configuration file not found: $CONFIG_FILE"
    exit 1
fi

# Execute base-linux-setup
if "$BINARY_PATH" --config "$CONFIG_FILE"; then
    print_info "Dependencies installed successfully!"
else
    print_error "Dependency installation failed"
    exit 1
fi

# Build and install the application
print_info "Building RockPi Penta Golang service..."
cd "$PROJECT_ROOT"

if [ ! -f "scripts/build.sh" ]; then
    print_error "Build script not found: scripts/build.sh"
    exit 1
fi

# Make build script executable
chmod +x scripts/build.sh

# Build and install
if ./scripts/build.sh install; then
    print_info "Application built and installed successfully!"
else
    print_error "Application build/installation failed"
    exit 1
fi

# Enable and start service
print_info "Configuring systemd service..."

systemctl daemon-reload
systemctl enable rockpi-penta

# Ask user if they want to start the service now
echo
print_warning "The service is ready but not started yet."
print_info "Before starting, you may want to:"
print_info "1. Review configuration: /etc/rockpi-penta.conf"
print_info "2. Review environment: /etc/rockpi-penta.env"
print_info "3. Reboot if I2C was just enabled"

echo
read -p "Start the service now? (y/N): " -n 1 -r
echo
if [[ $REPLY =~ ^[Yy]$ ]]; then
    print_info "Starting RockPi Penta service..."
    if systemctl start rockpi-penta; then
        print_info "Service started successfully!"
        
        # Show service status
        sleep 2
        print_info "Service status:"
        systemctl status rockpi-penta --no-pager -l
    else
        print_error "Failed to start service"
        print_info "Check logs with: journalctl -u rockpi-penta -f"
    fi
else
    print_info "Service enabled but not started."
    print_info "To start manually: sudo systemctl start rockpi-penta"
fi

# Cleanup
rm -rf "$TEMP_DIR"

print_info "Installation completed!"
echo
print_info "Useful commands:"
print_info "  Start service:   sudo systemctl start rockpi-penta"
print_info "  Stop service:    sudo systemctl stop rockpi-penta"
print_info "  Service status:  sudo systemctl status rockpi-penta"
print_info "  View logs:       sudo journalctl -u rockpi-penta -f"
print_info "  Edit config:     sudo nano /etc/rockpi-penta.conf"
print_info "  Edit hardware:   sudo nano /etc/rockpi-penta.env"
echo
print_warning "If you experience I2C issues, you may need to reboot your system." 