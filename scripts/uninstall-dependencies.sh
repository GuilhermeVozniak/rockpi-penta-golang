#!/bin/bash
set -e

echo "RockPi Penta Golang Dependencies Uninstaller"
echo "============================================="

# Check if running as root
if [ "$(id -u)" -ne 0 ]; then
    echo "This script must be run as root. Please use sudo."
    exit 1
fi

echo "This script will remove dependencies installed by install-dependencies.sh."
echo "It is recommended to run uninstall.sh first to remove the service."
echo

read -p "Continue with uninstalling dependencies? (y/n): " -n 1 -r
echo
if [[ ! $REPLY =~ ^[Yy]$ ]]; then
    echo "Operation cancelled."
    exit 0
fi

# Uninstall Go
if [ -d "/usr/local/go" ]; then
    read -p "Remove Go from /usr/local/go? (y/n): " -n 1 -r
    echo
    if [[ $REPLY =~ ^[Yy]$ ]]; then
        echo "Removing Go installation..."
        rm -rf /usr/local/go
        
        # Remove Go environment settings
        if [ -f "/etc/profile.d/go.sh" ]; then
            echo "Removing Go environment settings..."
            rm -f /etc/profile.d/go.sh
        fi
        
        echo "Go installation removed."
    else
        echo "Keeping Go installation."
    fi
else
    echo "Go installation not found in /usr/local/go."
fi

# Ask about removing packages
read -p "Remove installed system packages (i2c-tools, curl)? (y/n): " -n 1 -r
echo
if [[ $REPLY =~ ^[Yy]$ ]]; then
    echo "Removing packages..."
    apt-get remove -y i2c-tools curl 2>/dev/null || true
    echo "Packages removed."
else
    echo "Keeping system packages."
fi

# Ask about disabling I2C
read -p "Disable I2C modules (requires reboot)? (y/n): " -n 1 -r
echo
if [[ $REPLY =~ ^[Yy]$ ]]; then
    echo "Disabling I2C modules..."
    
    # Remove from /etc/modules
    if grep -q "^i2c-dev" /etc/modules; then
        sed -i '/^i2c-dev/d' /etc/modules
        echo "Removed i2c-dev from /etc/modules."
    fi
    
    # Remove from /boot/config.txt
    if [ -f "/boot/config.txt" ] && grep -q "^dtparam=i2c_arm=on" /boot/config.txt; then
        sed -i '/^dtparam=i2c_arm=on/d' /boot/config.txt
        echo "Removed I2C configuration from /boot/config.txt."
    fi
    
    echo "I2C modules disabled. A reboot is required for changes to take effect."
else
    echo "Keeping I2C modules enabled."
fi

# Ask about running apt autoremove to clean up unused dependencies
read -p "Run apt autoremove to clean up unused dependencies? (y/n): " -n 1 -r
echo
if [[ $REPLY =~ ^[Yy]$ ]]; then
    echo "Running apt autoremove..."
    apt-get autoremove -y
    echo "Unused dependencies removed."
else
    echo "Skipping autoremove."
fi

echo
echo "Dependencies uninstallation completed."
echo "Note: Some changes may require a reboot to take full effect." 