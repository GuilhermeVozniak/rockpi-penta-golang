#!/bin/bash
set -e

echo "RockPi Penta Golang Service Uninstaller"
echo "========================================"

# Check if running as root
if [ "$(id -u)" -ne 0 ]; then
    echo "This script must be run as root. Please use sudo."
    exit 1
fi

# Stop and disable the service
echo "Stopping and disabling RockPi Penta service..."
systemctl stop rockpi-penta 2>/dev/null || true
systemctl disable rockpi-penta 2>/dev/null || true
echo "Service stopped and disabled."

# Remove service file
SERVICE_FILE="/etc/systemd/system/rockpi-penta.service"
if [ -f "$SERVICE_FILE" ]; then
    echo "Removing systemd service file..."
    rm -f "$SERVICE_FILE"
    systemctl daemon-reload
    echo "Service file removed."
else
    echo "Service file not found, skipping."
fi

# Remove binary
BINARY_PATH="/usr/local/bin/rockpi-penta-service"
if [ -f "$BINARY_PATH" ]; then
    echo "Removing binary..."
    rm -f "$BINARY_PATH"
    echo "Binary removed."
else
    echo "Binary not found, skipping."
fi

# Ask about config file
CONFIG_FILE="/etc/rockpi-penta.conf"
if [ -f "$CONFIG_FILE" ]; then
    read -p "Do you want to remove the configuration file ($CONFIG_FILE)? (y/n): " -n 1 -r
    echo
    if [[ $REPLY =~ ^[Yy]$ ]]; then
        echo "Removing configuration file..."
        rm -f "$CONFIG_FILE"
        echo "Configuration file removed."
    else
        echo "Keeping configuration file."
    fi
else
    echo "Configuration file not found, skipping."
fi

# Ask about removing Go
if [ -d "/usr/local/go" ]; then
    read -p "Do you want to remove Go from /usr/local/go? (y/n): " -n 1 -r
    echo
    if [[ $REPLY =~ ^[Yy]$ ]]; then
        echo "Removing Go installation..."
        rm -rf /usr/local/go
        
        # Remove environment settings
        if [ -f "/etc/profile.d/go.sh" ]; then
            rm -f /etc/profile.d/go.sh
        fi
        
        echo "Go installation removed."
    else
        echo "Keeping Go installation."
    fi
fi

# Remove build artifacts from local directory if present
if [ -d "build" ] && [ -f "build/rockpi-penta-service" ]; then
    read -p "Do you want to remove local build artifacts? (y/n): " -n 1 -r
    echo
    if [[ $REPLY =~ ^[Yy]$ ]]; then
        echo "Removing local build artifacts..."
        rm -rf build
        echo "Build artifacts removed."
    else
        echo "Keeping local build artifacts."
    fi
fi

echo
echo "RockPi Penta Golang has been uninstalled."
echo "Note: If you enabled I2C interfaces during installation, those changes"
echo "were not reverted. You can manually disable them if needed."
echo "Thank you for using RockPi Penta Golang!" 