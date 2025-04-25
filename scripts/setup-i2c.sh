#!/bin/bash
set -e

echo "RockPi Penta I2C Setup Helper"
echo "============================"

# Check if running as root
if [ "$(id -u)" -ne 0 ]; then
    echo "This script must be run as root. Please use sudo."
    exit 1
fi

# Find available I2C buses
echo "Detecting available I2C buses..."
I2C_BUSES=$(ls -la /dev/i2c* 2>/dev/null | awk '{print $NF}')

if [ -z "$I2C_BUSES" ]; then
    echo "No I2C buses found on the system."
    echo "Make sure I2C is enabled in raspi-config."
    exit 1
fi

echo "Found I2C buses:"
echo "$I2C_BUSES" | nl

# Create an array of buses
mapfile -t BUS_ARRAY <<< "$I2C_BUSES"

# Check if we have only one bus
if [ ${#BUS_ARRAY[@]} -eq 1 ]; then
    SELECTED_BUS="${BUS_ARRAY[0]}"
    echo "Only one I2C bus found: $SELECTED_BUS. Using it automatically."
else
    echo
    echo "Multiple I2C buses found. Which one would you like to use for the OLED display?"
    echo "You can also choose to try all buses automatically."
    echo
    echo "0) Try all buses automatically"
    echo "$I2C_BUSES" | nl
    echo
    
    read -p "Enter your choice (0-${#BUS_ARRAY[@]}): " CHOICE
    
    if [ "$CHOICE" -eq 0 ]; then
        SELECTED_BUS=""
        TRY_ALL=1
        echo "Will try all buses automatically."
    elif [ "$CHOICE" -ge 1 ] && [ "$CHOICE" -le ${#BUS_ARRAY[@]} ]; then
        SELECTED_BUS="${BUS_ARRAY[$CHOICE-1]}"
        TRY_ALL=0
        echo "Selected I2C bus: $SELECTED_BUS"
    else
        echo "Invalid choice. Exiting."
        exit 1
    fi
fi

# Update the service file
SERVICE_FILE="/etc/systemd/system/rockpi-penta.service"

if [ ! -f "$SERVICE_FILE" ]; then
    echo "Service file not found at $SERVICE_FILE"
    echo "Please run install-dependencies.sh first to create the service file."
    exit 1
fi

echo "Updating service file with I2C configuration..."

# Create a temporary file to modify the service
TMP_FILE=$(mktemp)

# Remove any existing OLED_I2C_BUS and OLED_TRY_ALL_BUSES entries
grep -v "Environment=\"OLED_I2C_BUS=" "$SERVICE_FILE" | grep -v "Environment=\"OLED_TRY_ALL_BUSES=" > "$TMP_FILE"

# Add the new configuration
if [ -n "$SELECTED_BUS" ]; then
    # Insert the selected bus just after the [Service] line
    sed -i "/\[Service\]/a Environment=\"OLED_I2C_BUS=$SELECTED_BUS\"" "$TMP_FILE"
    echo "Set OLED_I2C_BUS=$SELECTED_BUS in service file."
fi

if [ "$TRY_ALL" = "1" ]; then
    # Insert the try all flag just after the [Service] line or after the OLED_I2C_BUS line
    sed -i "/\[Service\]/a Environment=\"OLED_TRY_ALL_BUSES=1\"" "$TMP_FILE"
    echo "Set OLED_TRY_ALL_BUSES=1 in service file."
fi

# Move the modified file back
mv "$TMP_FILE" "$SERVICE_FILE"

# Apply permissions
chmod 644 "$SERVICE_FILE"

echo
echo "Service file updated successfully."
echo "You should now restart the service with:"
echo "  sudo systemctl daemon-reload"
echo "  sudo systemctl restart rockpi-penta"
echo
echo "If the service still fails, try running i2cdetect -y <bus_number> to check for available devices."
echo "For example: i2cdetect -y 13 (for /dev/i2c-13)" 