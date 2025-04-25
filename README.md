# RockPi Penta Golang

Go implementation of the RockPi Penta service for monitoring and controlling the SATA HAT.

Repository: [https://github.com/GuilhermeVozniak/rockpi-penta-golang](https://github.com/GuilhermeVozniak/rockpi-penta-golang)

## Features

- **CPU & System Monitoring**: Display CPU temperature, load, memory usage, and uptime.
- **Disk Monitoring**: Display usage information for all attached SATA disks.
- **Fan Control**: Automatic PWM fan control based on CPU temperature thresholds.
- **OLED Display**: Information display with auto sliding pages and button-based navigation.
- **Button Interface**: Configurable actions for single click, double click, and long press.

## Prerequisites

- Raspberry Pi with RockPi Penta SATA HAT
- Raspberry Pi OS (or similar Linux distribution)
- Root/sudo access

## Quick Installation

We've simplified the installation process with two scripts that handle everything for you:

### 1. Clone the repository

```bash
git clone https://github.com/GuilhermeVozniak/rockpi-penta-golang.git
cd rockpi-penta-golang
```

### 2. Install dependencies and create config files

This script will:

- Install Go 1.24.2 (which provides Go 1.24)
- Install required system packages
- Enable I2C interface
- Create default configuration file at `/etc/rockpi-penta.conf`
- Set up systemd service file

```bash
sudo ./scripts/install-dependencies.sh
```

### 3. Build and install the service

This script will:

- Compile the Go application
- Optionally install the binary to /usr/local/bin

```bash
# Build only
./scripts/build.sh

# Build and install (requires sudo)
sudo ./scripts/build.sh
```

### 4. Start the service

```bash
sudo systemctl daemon-reload
sudo systemctl enable rockpi-penta
sudo systemctl start rockpi-penta
```

That's it! The service will now run automatically at boot.

## Uninstallation

To uninstall the RockPi Penta service and its components, use the provided uninstall script:

```bash
sudo ./scripts/uninstall.sh
```

The script will:

- Stop and disable the service
- Remove the systemd service file
- Remove the binary from /usr/local/bin
- Optionally remove the configuration file (with confirmation)
- Optionally remove Go installation (with confirmation)
- Optionally remove local build artifacts (with confirmation)

To completely remove all dependencies installed by the setup process, you can also run:

```bash
sudo ./scripts/uninstall-dependencies.sh
```

This additional script will:

- Remove Go from /usr/local/go (optional)
- Remove system packages like i2c-tools and curl (optional)
- Disable I2C modules in system configuration (optional)
- Clean up unused dependencies with apt autoremove (optional)

Each step in the dependency uninstallation process requires confirmation, allowing you to selectively remove only what you want.

## Configuration

The default configuration file is created at `/etc/rockpi-penta.conf`. You can edit this file to customize settings:

```ini
[fan]
# Temperature thresholds (°C) for fan speed control
lv0 = 35
lv1 = 40
lv2 = 45
lv3 = 50

[key]
# Button actions: slider, switch, reboot, poweroff, none
click = slider
twice = switch
press = none

[time]
# Time settings for button detection (seconds)
twice = 0.7
press = 1.8

[slider]
# OLED automatic page sliding settings
auto = true
time = 10

[oled]
# OLED display settings
rotate = false
f-temp = false
```

## Environment Variables

The default environment variables are set in the systemd service file. If you need to change these, edit `/etc/systemd/system/rockpi-penta.service`:

```bash
# Fan control - choose GPIO or hardware PWM
Environment="HARDWARE_PWM=0"
Environment="FAN_CHIP=gpiochip0"
Environment="FAN_LINE=18"

# Button control
Environment="BUTTON_CHIP=gpiochip0"
Environment="BUTTON_LINE=17"

# OLED display reset pin (optional)
Environment="OLED_RESET=27"
```

After editing, reload and restart the service:

```bash
sudo systemctl daemon-reload
sudo systemctl restart rockpi-penta
```

## GPIO Pin Configuration

The default configuration uses the following GPIO pins:

- Fan: GPIO 18 (Pin 12) for software PWM
- Button: GPIO 17 (Pin 11) for user input
- OLED Reset: GPIO 27 (Pin 13) for display reset

Adjust the environment variables to match your specific hardware setup.

## Manual Installation

If you prefer to set things up manually, follow these steps:

1. Install Go (version 1.24)
2. Enable I2C interface via `raspi-config`
3. Build the application: `go build -o rockpi-penta-service ./cmd/rockpi-penta-service`
4. Create a configuration file at `/etc/rockpi-penta.conf` (use example above)
5. Set up environment variables and run the service

## Configuration Details

### Fan Speed Control

- Below `lv0`: Fan is off
- Between `lv0` and `lv1`: Fan at 25% speed
- Between `lv1` and `lv2`: Fan at 50% speed
- Between `lv2` and `lv3`: Fan at 75% speed
- Above `lv3`: Fan at 100% speed

### Button Actions

- `slider`: Switch to the next OLED page
- `switch`: Toggle fan on/off
- `reboot`: Reboot the system
- `poweroff`: Power off the system
- `none`: No action

## Troubleshooting

- **Check service status**: `sudo systemctl status rockpi-penta`
- **View logs**: `sudo journalctl -u rockpi-penta`
- **Verify I2C**: Run `i2cdetect -y 1` to check if OLED display is detected
- **Test fan**: Set `HARDWARE_PWM=0` and check if GPIO pins are accessible
- **Permissions**: Make sure the service runs as root

If I2C is not working, you may need to reboot after enabling it.

## License

[Same as original RockPi Penta project]
