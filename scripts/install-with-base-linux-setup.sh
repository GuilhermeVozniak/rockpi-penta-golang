#!/bin/bash
set -e

echo "Setting up RockPi Penta dependencies using base-linux-setup..."

# Configuration
BASE_LINUX_SETUP_VERSION="v1.3.0"
BASE_LINUX_SETUP_BINARY=""
CONFIG_FILE="scripts/rockpi-penta-setup.json"

# Detect architecture for binary selection
ARCH=$(uname -m)
KERNEL=$(uname -s | tr '[:upper:]' '[:lower:]')

case "$KERNEL" in
    "linux")
        case "$ARCH" in
            "x86_64")
                BASE_LINUX_SETUP_BINARY="base-linux-setup-linux-amd64"
                ;;
            "aarch64"|"arm64")
                BASE_LINUX_SETUP_BINARY="base-linux-setup-linux-arm64"
                ;;
            "armv7l"|"armv6l"|arm*)
                BASE_LINUX_SETUP_BINARY="base-linux-setup-linux-arm"
                ;;
            *)
                echo "Unsupported architecture: $ARCH"
                exit 1
                ;;
        esac
        ;;
    "darwin")
        case "$ARCH" in
            "x86_64")
                BASE_LINUX_SETUP_BINARY="base-linux-setup-darwin-amd64"
                ;;
            "arm64")
                BASE_LINUX_SETUP_BINARY="base-linux-setup-darwin-arm64"
                ;;
            *)
                echo "Unsupported macOS architecture: $ARCH"
                exit 1
                ;;
        esac
        ;;
    *)
        echo "Unsupported kernel: $KERNEL"
        exit 1
        ;;
esac

echo "Detected: $KERNEL/$ARCH -> $BASE_LINUX_SETUP_BINARY"

# Create temporary directory for base-linux-setup
TEMP_DIR=$(mktemp -d)
cd "$TEMP_DIR"

echo "Downloading base-linux-setup $BASE_LINUX_SETUP_VERSION..."

# Download the specific binary
DOWNLOAD_URL="https://github.com/GuilhermeVozniak/base-linux-setup/releases/download/${BASE_LINUX_SETUP_VERSION}/${BASE_LINUX_SETUP_BINARY}"

if command -v curl >/dev/null 2>&1; then
    curl -L -o base-linux-setup "$DOWNLOAD_URL"
elif command -v wget >/dev/null 2>&1; then
    wget -O base-linux-setup "$DOWNLOAD_URL"
else
    echo "Error: Neither curl nor wget is available. Please install one of them."
    exit 1
fi

# Make it executable
chmod +x base-linux-setup

echo "Downloaded base-linux-setup successfully."

# Go back to the original directory
cd - > /dev/null

# Check if config file exists
if [ ! -f "$CONFIG_FILE" ]; then
    echo "Error: Configuration file not found: $CONFIG_FILE"
    echo "Make sure you're running this script from the rockpi-penta-golang directory."
    exit 1
fi

echo "Using configuration file: $CONFIG_FILE"

# Run base-linux-setup with our configuration
echo "Running base-linux-setup with RockPi Penta configuration..."

if [ "$(id -u)" -eq 0 ]; then
    echo "Warning: Running as root. Some tasks may behave differently."
fi

# Execute base-linux-setup with our config
"${TEMP_DIR}/base-linux-setup" --config "$CONFIG_FILE"

# Cleanup
rm -rf "$TEMP_DIR"

echo ""
echo "✅ RockPi Penta dependency installation completed!"
echo ""
echo "Next steps:"
echo "1. If you see any messages about restarting your terminal, please do so to apply environment changes"
echo "2. Build the application: ./scripts/build.sh"
echo "3. Enable the service: sudo systemctl enable rockpi-penta"
echo "4. Start the service: sudo systemctl start rockpi-penta"
echo ""
echo "For more information, check the service status with:"
echo "  sudo systemctl status rockpi-penta"
echo ""
echo "View logs with:"
echo "  sudo journalctl -u rockpi-penta -f" 