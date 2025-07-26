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
- Internet connection for downloading dependencies

## Quick Installation

We provide multiple installation methods. Choose the one that best fits your needs:

### Method 1: Using base-linux-setup (Recommended)

This method uses our enhanced [base-linux-setup](https://github.com/GuilhermeVozniak/base-linux-setup) tool that provides intelligent dependency management with conditional checks and proper dependency ordering.

```bash
# Clone the repository
git clone https://github.com/GuilhermeVozniak/rockpi-penta-golang.git
cd rockpi-penta-golang

# Run the enhanced installation script
chmod +x scripts/install-with-base-linux-setup.sh
sudo ./scripts/install-with-base-linux-setup.sh
```

This script will:
- Automatically download the appropriate base-linux-setup binary for your architecture
- Use the RockPi Penta configuration with intelligent dependency resolution
- Check if components are already installed before attempting installation
- Install Go 1.24.2 only if needed (checks existing version)
- Enable I2C interface only if not already enabled
- Create configuration files only if they don't exist
- Execute tasks in proper dependency order

**Features of the enhanced installation:**
- ✅ **Conditional Execution**: Only installs what's missing
- ✅ **Dependency Management**: Ensures tasks run in the correct order
- ✅ **Architecture Detection**: Downloads the right binary for your system
- ✅ **Idempotent**: Safe to run multiple times
- ✅ **Smart Checks**: Verifies existing installations before proceeding

### Method 2: Traditional Installation

If you prefer the traditional approach or want more control over the process:

#### 1. Clone the repository

```bash
git clone https://github.com/GuilhermeVozniak/rockpi-penta-golang.git
cd rockpi-penta-golang
```

#### 2. Install dependencies and create config files

This script will:

- Install Go 1.24.2 (which provides Go 1.24)
- Install required system packages
- Enable I2C interface
- Create default configuration file at `/etc/rockpi-penta.conf`
- Set up systemd service file

```bash
sudo ./scripts/install-dependencies.sh
```

#### 3. Build and install the service

This script will:

- Compile the Go application
- Optionally install the binary to /usr/local/bin

```bash
# Build only
./scripts/build.sh

# Build and install (requires sudo)
sudo ./scripts/build.sh
```

### Final Steps (Both Methods)

After running either installation method:

```bash
# Build the application
./scripts/build.sh

# Enable and start the service
sudo systemctl daemon-reload
sudo systemctl enable rockpi-penta
sudo systemctl start rockpi-penta
```

That's it! The service will now run automatically at boot.

## Configuration File

The installation creates a configuration file at `/etc/rockpi-penta.conf` that you can customize:

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

## Verification

You can verify the installation by checking:

```bash
# Check Go installation
go version

# Check I2C module
lsmod | grep i2c_dev

# Check service status
sudo systemctl status rockpi-penta

# View service logs
sudo journalctl -u rockpi-penta -f
```

## Advanced Configuration

### Custom base-linux-setup Configuration

If you want to modify the installation process, you can edit the `scripts/rockpi-penta-setup.json` file. This file contains:

- **Conditional Tasks**: Each task can have a `condition` field with a shell command that determines if the task should run
- **Dependencies**: Tasks can depend on other tasks using the `depends_on` field
- **Task Types**: Support for commands, scripts, files, and services

Example task with condition and dependency:
```json
{
  "name": "Install Go 1.24.2",
  "type": "script",
  "condition": "! command -v go >/dev/null 2>&1 || [ \"$(go version | awk '{print $3}' | sed 's/go//' | cut -d. -f1-2 2>/dev/null || echo '0.0')\" != \"1.24\" ]",
  "depends_on": ["Update System Packages"],
  "script": "# Go installation script here"
}
```

### Using Your Own base-linux-setup Binary

If you have your own build of base-linux-setup, you can use it directly:

```bash
# Using your own binary
your-base-linux-setup --config scripts/rockpi-penta-setup.json
```

## Troubleshooting

### Installation Issues

If the enhanced installation fails:

1. **Check internet connection**: The script downloads the base-linux-setup binary
2. **Verify architecture support**: Ensure your system architecture is supported
3. **Check permissions**: Some tasks require sudo privileges
4. **Review logs**: Check the output for specific error messages

### Service Issues

- **Check service status**: `sudo systemctl status rockpi-penta`
- **View logs**: `sudo journalctl -u rockpi-penta`
- **Verify I2C**: Run `i2cdetect -y 1` to check if OLED display is detected

### I2C Bus Issues

If you see errors like `failed to open I2C bus /dev/i2c-1: i2creg: no bus found` in your logs, your system might be using different I2C bus numbers than the default. Use the included I2C setup script to automatically configure the correct bus:

```bash
sudo ./scripts/setup-i2c.sh
```

This script will:

1. Detect all available I2C buses on your system
2. Let you choose which bus to use or try all buses automatically
3. Update the service file with the correct environment variables
4. Provide instructions for restarting the service

After running this script, restart the service:

```bash
sudo systemctl daemon-reload
sudo systemctl restart rockpi-penta
```

### Other Common Issues

- **Test fan**: Set `HARDWARE_PWM=0` and check if GPIO pins are accessible
- **Permissions**: Make sure the service runs as root
- **Missing hardware**: Ensure all hardware components are properly connected

If I2C is not working, you may need to reboot after enabling it.

## License

[Same as original RockPi Penta project]
