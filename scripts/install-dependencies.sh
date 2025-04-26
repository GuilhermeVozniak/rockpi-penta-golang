#!/bin/bash
set -e

echo "Installing RockPi Penta dependencies and configuration..."

# Process command line arguments
FORCE_OVERRIDE=false
if [[ "$1" == "--force" || "$1" == "-f" ]]; then
    FORCE_OVERRIDE=true
    echo "Force override mode activated - will replace existing configurations"
fi

# Check if running as root
if [ "$(id -u)" -ne 0 ]; then
    echo "This script must be run as root. Please use sudo."
    exit 1
fi

# If not in force mode, ask about overriding configurations
if [ "$FORCE_OVERRIDE" = false ]; then
    read -p "Do you want to override any existing configurations? (y/n): " -n 1 -r
    echo
    if [[ $REPLY =~ ^[Yy]$ ]]; then
        FORCE_OVERRIDE=true
        echo "Will override existing configurations."
    else
        echo "Will preserve existing configurations where present."
    fi
fi

# Install Go 1.24.2 if not already installed or if version doesn't match
GO_VERSION="1.24.2"  # Full version for downloading
GO_MOD_VERSION="1.24"  # Version format for go.mod
INSTALL_GO=false

if ! command -v go version &> /dev/null; then
    echo "Go not found, will install version $GO_VERSION..."
    INSTALL_GO=true
elif [ "$FORCE_OVERRIDE" = true ]; then
    echo "Force override: Will reinstall Go $GO_VERSION..."
    INSTALL_GO=true
else
    CURRENT_GO_VERSION=$(go version | awk '{print $3}' | sed 's/go//')
    # Remove patch version for comparison with GO_MOD_VERSION
    CURRENT_GO_MAJOR_MINOR=$(echo "$CURRENT_GO_VERSION" | cut -d. -f1-2)
    
    if [[ "$CURRENT_GO_MAJOR_MINOR" != "$GO_MOD_VERSION" ]]; then
        echo "Go version $CURRENT_GO_VERSION detected, but version $GO_VERSION is required."
        echo "Will install Go $GO_VERSION..."
        INSTALL_GO=true
    else
        echo "Go $CURRENT_GO_MAJOR_MINOR is already installed and compatible with required version $GO_MOD_VERSION."
    fi
fi

if [ "$INSTALL_GO" = true ]; then
    echo "Installing Go $GO_VERSION..."
    
    # Determine architecture
    ARCH=$(uname -m)
    if [ "$ARCH" = "x86_64" ]; then
        GO_ARCH="amd64"
    elif [ "$ARCH" = "aarch64" ] || [ "$ARCH" = "arm64" ]; then
        GO_ARCH="arm64"
    elif [[ "$ARCH" == arm* ]]; then
        GO_ARCH="armv6l"
    else
        echo "Unsupported architecture: $ARCH"
        exit 1
    fi
    
    # Setup temporary directory
    TMP_DIR=$(mktemp -d)
    cd "$TMP_DIR"
    
    # Download and install Go
    GO_PACKAGE="go${GO_VERSION}.linux-${GO_ARCH}.tar.gz"
    GO_URL="https://go.dev/dl/${GO_PACKAGE}"
    
    echo "Downloading Go from $GO_URL..."
    if ! curl -LO "$GO_URL"; then
        echo "Failed to download Go. Please check your internet connection."
        exit 1
    fi
    
    echo "Extracting Go to /usr/local..."
    rm -rf /usr/local/go
    tar -C /usr/local -xzf "$GO_PACKAGE"
    
    # Set up environment if needed
    echo 'export PATH=$PATH:/usr/local/go/bin' > /etc/profile.d/go.sh
    chmod +x /etc/profile.d/go.sh
    
    # Add to current session's PATH
    export PATH=$PATH:/usr/local/go/bin
    
    # Clean up
    cd - > /dev/null
    rm -rf "$TMP_DIR"
    
    echo "Go $GO_VERSION installed successfully."
    echo "Note: In go.mod, the version is specified as $GO_MOD_VERSION (without patch version)."
else
    echo "Skipping Go installation as it's already installed with a compatible version."
fi

# Check and install required system packages
echo "Checking required system packages..."
PACKAGES_TO_INSTALL=""

# Check for i2c-tools
if ! dpkg -l | grep -q i2c-tools || [ "$FORCE_OVERRIDE" = true ]; then
    PACKAGES_TO_INSTALL="$PACKAGES_TO_INSTALL i2c-tools"
fi

# Check for curl
if ! command -v curl &> /dev/null || [ "$FORCE_OVERRIDE" = true ]; then
    PACKAGES_TO_INSTALL="$PACKAGES_TO_INSTALL curl"
fi

# Install any missing packages
if [ -n "$PACKAGES_TO_INSTALL" ]; then
    echo "Installing packages:$PACKAGES_TO_INSTALL"
    apt-get update
    apt-get install -y $PACKAGES_TO_INSTALL
else
    echo "All required system packages are already installed."
fi

# Enable I2C programmatically instead of requiring raspi-config
echo "Enabling I2C interface..."
I2C_ENABLED=false

# Always update modules when in force mode
if ! grep -q "^i2c-dev" /etc/modules || [ "$FORCE_OVERRIDE" = true ]; then
    if [ "$FORCE_OVERRIDE" = true ] && grep -q "^i2c-dev" /etc/modules; then
        echo "Force override: Updating i2c-dev in /etc/modules..."
        sed -i '/^i2c-dev/d' /etc/modules
    else
        echo "Adding i2c-dev to /etc/modules..."
    fi
    echo "i2c-dev" >> /etc/modules
    I2C_ENABLED=true
else
    echo "i2c-dev already in /etc/modules."
fi

# Enable I2C in config.txt if it exists (Raspberry Pi specific)
if [ -f "/boot/config.txt" ]; then
    if ! grep -q "^dtparam=i2c_arm=on" /boot/config.txt || [ "$FORCE_OVERRIDE" = true ]; then
        if [ "$FORCE_OVERRIDE" = true ] && grep -q "^dtparam=i2c_arm=on" /boot/config.txt; then
            echo "Force override: Updating I2C in /boot/config.txt..."
            sed -i '/^dtparam=i2c_arm=on/d' /boot/config.txt
        else
            echo "Enabling I2C in /boot/config.txt..."
        fi
        echo "dtparam=i2c_arm=on" >> /boot/config.txt
        I2C_ENABLED=true
    else
        echo "I2C already enabled in /boot/config.txt."
    fi
fi

# Load module immediately without requiring reboot
if [ "$I2C_ENABLED" = true ]; then
    echo "Loading I2C kernel module..."
    modprobe i2c-dev
    echo "I2C interface has been enabled."
else
    echo "I2C interface was already enabled."
fi

# Create config directory if it doesn't exist
mkdir -p /etc

# Create default configuration file
CONFIG_FILE="/etc/rockpi-penta.conf"
if [ ! -f "$CONFIG_FILE" ] || [ "$FORCE_OVERRIDE" = true ]; then
    if [ -f "$CONFIG_FILE" ] && [ "$FORCE_OVERRIDE" = true ]; then
        echo "Force override: Replacing existing configuration file at $CONFIG_FILE..."
        rm -f "$CONFIG_FILE"
    else
        echo "Creating default configuration file at $CONFIG_FILE..."
    fi
    
    cat > "$CONFIG_FILE" << 'EOF'
[fan]
# When the temperature is above lv0 (35°C), the fan at 25% power,
# and lv1 at 50% power, lv2 at 75% power, lv3 at 100% power.
# When the temperature is below lv0, the fan is turned off.
lv0 = 35
lv1 = 40
lv2 = 45
lv3 = 50

[key]
# You can customize the function of the key, currently available functions are
# slider: oled display next page
# switch: fan turn on/off switch
# reboot, poweroff
click = slider
twice = switch
press = none

[time]
# twice: maximum time between double clicking (seconds)
# press: long press time (seconds)
twice = 0.7
press = 1.8

[slider]
# Whether the oled auto display next page and the time interval (seconds)
auto = true
time = 10

[oled]
# Whether rotate the text of oled 180 degrees, whether use Fahrenheit
rotate = false
f-temp = false
EOF
    echo "Configuration file created successfully."
else
    echo "Configuration file already exists at $CONFIG_FILE."
    echo "Run with --force to override existing configuration."
fi

# Create systemd service file
SERVICE_FILE="/etc/systemd/system/rockpi-penta.service"
if [ ! -f "$SERVICE_FILE" ] || [ "$FORCE_OVERRIDE" = true ]; then
    if [ -f "$SERVICE_FILE" ] && [ "$FORCE_OVERRIDE" = true ]; then
        echo "Force override: Replacing existing service file at $SERVICE_FILE..."
        rm -f "$SERVICE_FILE"
    else
        echo "Creating systemd service file..."
    fi
    
    cat > "$SERVICE_FILE" << 'EOF'
[Unit]
Description=RockPi Penta Service
After=network.target

[Service]
Type=simple
ExecStart=/usr/local/bin/rockpi-penta-service
Environment="HARDWARE_PWM=0"
Environment="FAN_CHIP=gpiochip0"
Environment="FAN_LINE=18"
Environment="BUTTON_CHIP=gpiochip0"
Environment="BUTTON_LINE=17"
Environment="OLED_RESET=27"
Restart=on-failure
RestartSec=10
KillMode=process

[Install]
WantedBy=multi-user.target
EOF
    echo "Systemd service file created successfully."
else
    echo "Systemd service file already exists at $SERVICE_FILE."
    echo "Run with --force to override existing service file."
fi

# If we made changes to the service file, reload systemd
if [ -f "$SERVICE_FILE" ] && [ "$FORCE_OVERRIDE" = true ]; then
    echo "Reloading systemd daemon..."
    systemctl daemon-reload
fi

echo "Dependencies and configuration setup complete!"
echo "Next steps:"
echo "1. Run './scripts/build.sh' to build the application"
echo "2. Enable the service with 'sudo systemctl enable rockpi-penta'"
echo "3. Start the service with 'sudo systemctl start rockpi-penta'" 