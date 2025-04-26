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

# Ask about removing i2c-tools but keep curl
read -p "Remove i2c-tools package? (y/n): " -n 1 -r
echo
if [[ $REPLY =~ ^[Yy]$ ]]; then
    echo "Removing i2c-tools package..."
    apt-get remove -y i2c-tools 2>/dev/null || true
    echo "i2c-tools package removed."
else
    echo "Keeping i2c-tools package."
fi

# Ask about disabling I2C
read -p "Disable I2C interface? (y/n): " -n 1 -r
echo
if [[ $REPLY =~ ^[Yy]$ ]]; then
    echo "Disabling I2C interface..."
    I2C_DISABLED=false
    
    # Remove from /etc/modules
    if grep -q "^i2c-dev" /etc/modules; then
        echo "Removing i2c-dev from /etc/modules..."
        sed -i '/^i2c-dev/d' /etc/modules
        I2C_DISABLED=true
    else
        echo "i2c-dev not found in /etc/modules."
    fi
    
    # Remove from /boot/config.txt if it exists
    if [ -f "/boot/config.txt" ] && grep -q "^dtparam=i2c_arm=on" /boot/config.txt; then
        echo "Disabling I2C in /boot/config.txt..."
        sed -i '/^dtparam=i2c_arm=on/d' /boot/config.txt
        I2C_DISABLED=true
    else
        echo "I2C configuration not found in /boot/config.txt or file doesn't exist."
    fi
    
    # Unload the module immediately
    if lsmod | grep -q "i2c_dev"; then
        echo "Unloading I2C kernel module..."
        rmmod i2c_dev 2>/dev/null || true
    fi
    
    if [ "$I2C_DISABLED" = true ]; then
        echo "I2C interface has been disabled."
    else
        echo "No I2C configuration was found to disable."
    fi
else
    echo "Keeping I2C interface enabled."
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