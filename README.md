# RockPi Penta Golang

A complete Go implementation of the RockPi Penta SATA HAT controller, providing fan control, OLED display management, and button interaction for SATA expansion boards.

## Features

- **🌡️ Temperature Monitoring**: Real-time CPU temperature monitoring with configurable thresholds
- **🌀 Smart Fan Control**: Automatic PWM fan control with multiple speed levels based on temperature  
- **📺 OLED Display**: 128x32 OLED display with multiple pages showing system information
- **🔘 Button Interface**: Configurable button actions (click, double-click, long press)
- **⚙️ Hardware Support**: Both hardware PWM and software PWM (GPIO) fan control
- **🔧 Configurable**: Easy configuration via INI file with hot-reload support
- **📊 System Info**: Display CPU load, memory usage, disk usage, uptime, and IP address
- **🔄 Auto-sliding**: Automatic page rotation on OLED display

## Quick Installation

### Method 1: Enhanced Installation (Recommended)

Uses our [base-linux-setup](https://github.com/GuilhermeVozniak/base-linux-setup) tool for intelligent dependency management:

```bash
# Clone the repository
git clone https://github.com/GuilhermeVozniak/rockpi-penta-golang.git
cd rockpi-penta-golang

# Run enhanced installation
sudo ./scripts/install-with-base-linux-setup.sh
```

**Benefits of enhanced installation:**
- ✅ **Conditional Execution**: Only installs missing dependencies
- ✅ **Architecture Detection**: Downloads correct binaries automatically  
- ✅ **Dependency Ordering**: Ensures tasks run in proper sequence
- ✅ **Idempotent**: Safe to run multiple times
- ✅ **Smart Checks**: Verifies existing installations before proceeding

### Method 2: Manual Installation

For users who prefer manual control:

```bash
# 1. Install system dependencies
sudo apt-get update
sudo apt-get install -y i2c-tools libi2c-dev build-essential git curl wget

# 2. Install Go 1.21+
# Download from https://golang.org/dl/ or use your package manager

# 3. Enable I2C interface
sudo modprobe i2c-dev
echo 'i2c-dev' | sudo tee -a /etc/modules
echo 'dtparam=i2c_arm=on' | sudo tee -a /boot/config.txt

# 4. Build and install
chmod +x scripts/build.sh
sudo ./scripts/build.sh install

# 5. Copy configuration files
sudo cp configs/rockpi-penta.conf /etc/
sudo cp configs/rockpi-penta.env /etc/
sudo cp configs/rockpi-penta.service /etc/systemd/system/

# 6. Enable and start service
sudo systemctl daemon-reload
sudo systemctl enable rockpi-penta
sudo systemctl start rockpi-penta
```

## Configuration

### Main Configuration (`/etc/rockpi-penta.conf`)

```ini
[fan]
# Temperature thresholds (°C) for fan speed control
lv0 = 35  # 25% power
lv1 = 40  # 50% power  
lv2 = 45  # 75% power
lv3 = 50  # 100% power

[key]
# Button actions: slider, switch, reboot, poweroff, none
click = slider    # Single click advances OLED page
twice = switch    # Double click toggles fan on/off
press = none      # Long press does nothing

[time]
# Button timing (seconds)
twice = 0.7  # Max time between double clicks
press = 1.8  # Long press duration

[slider]
# OLED auto-slide settings
auto = true  # Enable automatic page rotation
time = 10    # Seconds between pages

[oled]
# Display settings
rotate = false  # Rotate display 180 degrees
f-temp = false  # Use Fahrenheit instead of Celsius
```

### Hardware Configuration (`/etc/rockpi-penta.env`)

```bash
# I2C pins for OLED display
SDA=SDA
SCL=SCL
OLED_RESET=D23

# Button GPIO
BUTTON_CHIP=4
BUTTON_LINE=17

# Fan control GPIO
FAN_CHIP=4
FAN_LINE=27
HARDWARE_PWM=0  # 0=software PWM, 1=hardware PWM
```

## Hardware Compatibility

### Supported Boards

The system automatically detects and configures the following boards:

**Raspberry Pi Series:**
- Raspberry Pi 5 (with automatic GPIO configuration)
- Raspberry Pi 4 (with automatic GPIO configuration)  
- Raspberry Pi 3 (with automatic GPIO configuration)

**Rock Pi Series:**
- Rock 5A (RK3588) - with hardware PWM support
- Rock Pi 5 (RK3588) - with hardware PWM support
- Rock Pi 4 (RK3399) - with hardware PWM support
- Rock Pi 3 (RK3566) - with hardware PWM support
- Rock 3C (RK3566) - with software PWM

**Other ARM boards:** Generic fallback configurations available

### Pin Configurations

The system automatically configures GPIO pins based on detected hardware. Manual override is available via `/etc/rockpi-penta.env`:

**Raspberry Pi 5:**
```bash
BUTTON_CHIP=4
BUTTON_LINE=17
FAN_CHIP=4
FAN_LINE=27
HARDWARE_PWM=0
I2C_BUS=/dev/i2c-1
```

**Raspberry Pi 4:**
```bash
BUTTON_CHIP=0
BUTTON_LINE=17
FAN_CHIP=0
FAN_LINE=27
HARDWARE_PWM=0
I2C_BUS=/dev/i2c-1
```

**Rock 5A:**
```bash
BUTTON_CHIP=4
BUTTON_LINE=11
PWMCHIP=14
HARDWARE_PWM=1
I2C_BUS=/dev/i2c-8
```

**Rock Pi 4:**
```bash
BUTTON_CHIP=4
BUTTON_LINE=18
PWMCHIP=1
HARDWARE_PWM=1
I2C_BUS=/dev/i2c-7
```

**Rock Pi 3:**
```bash
BUTTON_CHIP=3
BUTTON_LINE=20
PWMCHIP=15
HARDWARE_PWM=1
I2C_BUS=/dev/i2c-3
```

**Rock 3C:**
```bash
BUTTON_CHIP=3
BUTTON_LINE=1
FAN_CHIP=3
FAN_LINE=2
HARDWARE_PWM=0
I2C_BUS=/dev/i2c-1
```

## Service Management

```bash
# Start/stop service
sudo systemctl start rockpi-penta
sudo systemctl stop rockpi-penta

# Enable/disable auto-start
sudo systemctl enable rockpi-penta
sudo systemctl disable rockpi-penta

# Check status
sudo systemctl status rockpi-penta

# View logs
sudo journalctl -u rockpi-penta -f

# Restart after config changes
sudo systemctl restart rockpi-penta
```

## OLED Display Pages

The OLED automatically cycles through three information pages:

1. **System Overview**: Uptime, CPU temperature, IP address
2. **Performance**: CPU load, memory usage  
3. **Storage**: Disk usage for root and attached SATA drives

Navigate manually using the button (single click by default).

## Button Actions

Configure button behavior in `/etc/rockpi-penta.conf`:

- **slider**: Advance to next OLED page
- **switch**: Toggle fan on/off
- **reboot**: Restart the system  
- **poweroff**: Shutdown the system
- **none**: No action

## Device Detection & Verification

The system includes automatic device detection to configure the correct GPIO pins and hardware settings for your board.

### Automatic Detection

The service will automatically detect your hardware platform and configure appropriate settings:

- **Raspberry Pi 5**: Uses gpiochip4, software PWM, I2C bus 1
- **Raspberry Pi 4**: Uses gpiochip0, software PWM, I2C bus 1  
- **Raspberry Pi 3**: Uses gpiochip0, software PWM, I2C bus 1
- **Rock 5A**: Uses gpiochip4, PWM chip 14, I2C bus 8
- **Rock Pi 5**: Uses gpiochip4, hardware PWM, I2C bus varies
- **Rock Pi 4**: Uses gpiochip4, PWM chip 1, I2C bus 7
- **Rock Pi 3**: Uses gpiochip3, PWM chip 15, I2C bus 3
- **Rock 3C**: Uses gpiochip3, software PWM, GPIO I2C
- **Unknown boards**: Falls back to Raspberry Pi 5 defaults

Auto-detection can be disabled by setting: `export DISABLE_AUTO_DETECT=1`

### Device Information Utility

Use the `rockpi-penta-device-info` command to verify your configuration:

```bash
# Show detected device and recommended configuration
sudo rockpi-penta-device-info

# Show only environment variables to set
sudo rockpi-penta-device-info -env

# Show export commands for shell
sudo rockpi-penta-device-info -export

# Verify current hardware access
sudo rockpi-penta-device-info -verify

# Verbose output with system details
sudo rockpi-penta-device-info -v
```

### Manual Configuration Override

If auto-detection doesn't work correctly, manually set environment variables in `/etc/rockpi-penta.env`:

```bash
# For Raspberry Pi 5
export BUTTON_CHIP=4
export BUTTON_LINE=17
export FAN_CHIP=4
export FAN_LINE=27
export HARDWARE_PWM=0
export I2C_BUS=/dev/i2c-1

# For Raspberry Pi 4/3  
export BUTTON_CHIP=0
export BUTTON_LINE=17
export FAN_CHIP=0
export FAN_LINE=27
export HARDWARE_PWM=0
export I2C_BUS=/dev/i2c-1

# For Rock 5A
export BUTTON_CHIP=4
export BUTTON_LINE=11
export PWMCHIP=14
export HARDWARE_PWM=1
export I2C_BUS=/dev/i2c-8

# For Rock Pi 4
export BUTTON_CHIP=4
export BUTTON_LINE=18
export PWMCHIP=1
export HARDWARE_PWM=1
export I2C_BUS=/dev/i2c-7

# For Rock Pi 3
export BUTTON_CHIP=3
export BUTTON_LINE=20
export PWMCHIP=15
export HARDWARE_PWM=1
export I2C_BUS=/dev/i2c-3

# For Rock 3C
export BUTTON_CHIP=3
export BUTTON_LINE=1
export FAN_CHIP=3
export FAN_LINE=2
export HARDWARE_PWM=0
export I2C_BUS=/dev/i2c-1
```

## Troubleshooting

### Service Won't Start

First, verify your hardware configuration:
```bash
# Check device detection and hardware access
sudo rockpi-penta-device-info -verify
```

Then check service logs for specific errors:
```bash
sudo journalctl -u rockpi-penta -n 50
```

Common issues:
- I2C not enabled: `sudo modprobe i2c-dev`
- Permissions: Service must run as root
- Hardware not connected: OLED/button failures are non-fatal
- Wrong GPIO pins: Use `rockpi-penta-device-info` to verify configuration

### I2C Issues

Verify I2C is working:
```bash
# Check I2C devices
sudo i2cdetect -y 1

# Should show device at address 0x3C (OLED)
```

If I2C detection fails:
```bash
# Enable I2C interface
sudo raspi-config  # Select Interface Options > I2C > Enable

# Or manually:
echo 'dtparam=i2c_arm=on' | sudo tee -a /boot/config.txt
sudo reboot
```

### Fan Not Working

1. Check PWM mode in `/etc/rockpi-penta.env`
2. Verify GPIO pins are correct for your board
3. Try switching between hardware (1) and software (0) PWM
4. Check physical connections

### Build Issues

Ensure you have Go 1.21+:
```bash
go version  # Should show 1.21 or later
```

Install missing dependencies:
```bash
sudo apt-get install build-essential libi2c-dev
```

## Development

### Project Structure
```
rockpi-penta-golang/
├── cmd/main.go                    # Main application entry point
├── pkg/
│   ├── config/                    # Configuration management
│   ├── hardware/
│   │   ├── fan/                   # Fan control (PWM/GPIO)
│   │   ├── oled/                  # OLED display management
│   │   └── button/                # Button input handling
│   └── sysinfo/                   # System information gathering
├── configs/                       # Configuration templates
├── scripts/                       # Build and installation scripts
└── README.md
```

### Building from Source

```bash
# Clone repository
git clone https://github.com/GuilhermeVozniak/rockpi-penta-golang.git
cd rockpi-penta-golang

# Install dependencies
go mod tidy

# Build
./scripts/build.sh

# Test (requires hardware or will show errors)
sudo ./build/rockpi-penta
```

### Contributing

1. Fork the repository
2. Create a feature branch: `git checkout -b feature-name`
3. Make your changes
4. Test on actual hardware if possible
5. Submit a pull request

## License

This project maintains compatibility with the original RockPi Penta Python implementation. Please refer to the original project for licensing terms.

## Acknowledgments

- Original Python implementation: [radxa/rockpi-penta](https://github.com/radxa/rockpi-penta)
- Hardware support via [periph.io](https://periph.io)
- Enhanced installation via [base-linux-setup](https://github.com/GuilhermeVozniak/base-linux-setup)

## Support

- **Issues**: [GitHub Issues](https://github.com/GuilhermeVozniak/rockpi-penta-golang/issues)
- **Hardware**: [Radxa Penta SATA HAT Documentation](https://docs.radxa.com/en/accessories/penta-sata-hat)
- **Original Project**: [RockPi Penta Python](https://github.com/radxa/rockpi-penta) 