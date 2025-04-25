#!/bin/bash
set -e

echo "Installing RockPi Penta dependencies and configuration..."

# Check if running as root
if [ "$(id -u)" -ne 0 ]; then
    echo "This script must be run as root. Please use sudo."
    exit 1
fi

# Install Go if not already installed
if ! command -v go &> /dev/null; then
    echo "Go not found, installing..."
    apt-get update
    apt-get install -y golang
fi

# Install required system packages
echo "Installing required system packages..."
apt-get update
apt-get install -y i2c-tools

# Enable I2C if not already enabled
if ! grep -q "^i2c-dev" /etc/modules; then
    echo "Enabling I2C..."
    echo "i2c-dev" >> /etc/modules
    if ! grep -q "^dtparam=i2c_arm=on" /boot/config.txt; then
        echo "dtparam=i2c_arm=on" >> /boot/config.txt
    fi
    echo "I2C has been enabled. A reboot will be required."
fi

# Create config directory if it doesn't exist
mkdir -p /etc

# Create default configuration file
CONFIG_FILE="/etc/rockpi-penta.conf"
if [ ! -f "$CONFIG_FILE" ]; then
    echo "Creating default configuration file at $CONFIG_FILE..."
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
fi

# Create systemd service file
SERVICE_FILE="/etc/systemd/system/rockpi-penta.service"
if [ ! -f "$SERVICE_FILE" ]; then
    echo "Creating systemd service file..."
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
fi

echo "Dependencies and configuration setup complete!"
echo "Next steps:"
echo "1. Run './scripts/build.sh' to build the application"
echo "2. After building, run 'sudo systemctl daemon-reload'"
echo "3. Enable the service with 'sudo systemctl enable rockpi-penta'"
echo "4. Start the service with 'sudo systemctl start rockpi-penta'"
echo "Note: A reboot may be required for I2C changes to take effect." 