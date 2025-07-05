#!/bin/bash

# ROCK Pi Penta SATA HAT Controller (Go) - Installation Script
# This script provides cross-distribution compatibility for Raspberry Pi OS and Kali Linux

set -e

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

# Configuration
BINARY_NAME="rocki-penta"
INSTALL_DIR="/usr/local/bin"
CONFIG_DIR="/etc"
SYSTEMD_DIR="/etc/systemd/system"
SERVICE_NAME="rocki-penta.service"

# Function to print colored output
log_info() {
    echo -e "${GREEN}[INFO]${NC} $1"
}

log_warn() {
    echo -e "${YELLOW}[WARN]${NC} $1"
}

log_error() {
    echo -e "${RED}[ERROR]${NC} $1"
}

# Function to check if running as root
check_root() {
    if [[ $EUID -ne 0 ]]; then
        log_error "This script must be run as root"
        exit 1
    fi
}

# Function to detect the operating system
detect_os() {
    if [[ -f /etc/os-release ]]; then
        . /etc/os-release
        OS=$ID
        VERSION=$VERSION_ID
    else
        log_error "Cannot detect operating system"
        exit 1
    fi
    
    log_info "Detected OS: $OS $VERSION"
}

# Function to detect board type
detect_board() {
    local model=""
    
    # Try multiple locations for device tree model
    for location in "/proc/device-tree/model" "/sys/firmware/devicetree/base/model"; do
        if [[ -f "$location" ]]; then
            model=$(tr -d '\0' < "$location" 2>/dev/null || echo "")
            if [[ -n "$model" ]]; then
                break
            fi
        fi
    done
    
    # Fallback to /proc/cpuinfo
    if [[ -z "$model" ]]; then
        model=$(grep "^Model" /proc/cpuinfo | cut -d':' -f2 | xargs 2>/dev/null || echo "")
    fi
    
    if [[ -z "$model" ]]; then
        log_error "Could not detect board type"
        exit 1
    fi
    
    log_info "Detected board: $model"
    echo "$model"
}

# Function to install system dependencies
install_dependencies() {
    log_info "Installing system dependencies..."
    
    case "$OS" in
        "raspbian"|"debian")
            apt-get update
            apt-get install -y i2c-tools
            ;;
        "kali")
            apt-get update
            apt-get install -y i2c-tools
            ;;
        "ubuntu")
            apt-get update
            apt-get install -y i2c-tools
            ;;
        *)
            log_warn "Unknown OS: $OS. Attempting to install i2c-tools..."
            if command -v apt-get &> /dev/null; then
                apt-get update
                apt-get install -y i2c-tools
            elif command -v yum &> /dev/null; then
                yum install -y i2c-tools
            elif command -v pacman &> /dev/null; then
                pacman -S --noconfirm i2c-tools
            else
                log_error "Cannot install dependencies on this system"
                exit 1
            fi
            ;;
    esac
}

# Function to enable I2C
enable_i2c() {
    log_info "Enabling I2C..."
    
    # Method 1: Try raspi-config (works on Raspberry Pi OS)
    if command -v raspi-config &> /dev/null; then
        log_info "Using raspi-config to enable I2C"
        raspi-config nonint do_i2c 0 || log_warn "raspi-config failed, trying alternative method"
    fi
    
    # Method 2: Direct configuration for broader compatibility
    local boot_config="/boot/config.txt"
    local firmware_config="/boot/firmware/config.txt"
    
    # Check different locations for config.txt
    for config_file in "$boot_config" "$firmware_config"; do
        if [[ -f "$config_file" ]]; then
            log_info "Configuring I2C in $config_file"
            
            # Enable I2C
            if ! grep -q "^dtparam=i2c_arm=on" "$config_file"; then
                echo "dtparam=i2c_arm=on" >> "$config_file"
                log_info "Added dtparam=i2c_arm=on to $config_file"
            fi
            
            # Load I2C module
            if ! grep -q "^i2c-dev" /etc/modules; then
                echo "i2c-dev" >> /etc/modules
                log_info "Added i2c-dev to /etc/modules"
            fi
            
            break
        fi
    done
    
    # Method 3: Modprobe for immediate effect
    modprobe i2c-dev 2>/dev/null || log_warn "Could not load i2c-dev module"
}

# Function to create hardware environment file
create_hardware_env() {
    local board_model="$1"
    local env_file="$CONFIG_DIR/rocki-penta.env"
    
    log_info "Creating hardware environment file: $env_file"
    
    case "$board_model" in
        *"Raspberry Pi 5"*)
            cat > "$env_file" << 'EOF'
SDA=SDA
SCL=SCL
OLED_RESET=D23
BUTTON_CHIP=4
BUTTON_LINE=17
FAN_CHIP=4
FAN_LINE=27
HARDWARE_PWM=0
EOF
            ;;
        *"Raspberry Pi 4"*)
            cat > "$env_file" << 'EOF'
SDA=SDA
SCL=SCL
OLED_RESET=D23
BUTTON_CHIP=0
BUTTON_LINE=17
FAN_CHIP=0
FAN_LINE=27
HARDWARE_PWM=0
EOF
            ;;
        *)
            log_warn "Unknown board type, using Raspberry Pi 4 defaults"
            cat > "$env_file" << 'EOF'
SDA=SDA
SCL=SCL
OLED_RESET=D23
BUTTON_CHIP=0
BUTTON_LINE=17
FAN_CHIP=0
FAN_LINE=27
HARDWARE_PWM=0
EOF
            ;;
    esac
    
    log_info "Hardware environment file created successfully"
}

# Function to install the binary
install_binary() {
    local binary_path="$1"
    
    if [[ ! -f "$binary_path" ]]; then
        log_error "Binary not found: $binary_path"
        exit 1
    fi
    
    log_info "Installing binary to $INSTALL_DIR/$BINARY_NAME"
    cp "$binary_path" "$INSTALL_DIR/$BINARY_NAME"
    chmod +x "$INSTALL_DIR/$BINARY_NAME"
    
    log_info "Binary installed successfully"
}

# Function to install configuration
install_config() {
    local config_path="$1"
    local target_config="$CONFIG_DIR/rockpi-penta.conf"
    
    if [[ -f "$config_path" ]]; then
        log_info "Installing configuration to $target_config"
        cp "$config_path" "$target_config"
        log_info "Configuration installed successfully"
    else
        log_warn "Configuration file not found: $config_path"
        log_info "Creating default configuration"
        cat > "$target_config" << 'EOF'
[fan]
lv0 = 35
lv1 = 40
lv2 = 45
lv3 = 50

[key]
click = slider
twice = switch
press = none

[time]
twice = 0.7
press = 1.8

[slider]
auto = true
time = 10

[oled]
rotate = false
f-temp = false
EOF
    fi
}

# Function to install systemd service
install_service() {
    local service_path="$1"
    local target_service="$SYSTEMD_DIR/$SERVICE_NAME"
    
    if [[ -f "$service_path" ]]; then
        log_info "Installing systemd service to $target_service"
        cp "$service_path" "$target_service"
    else
        log_warn "Service file not found: $service_path"
        log_info "Creating default systemd service"
        cat > "$target_service" << 'EOF'
[Unit]
Description=ROCK Pi Penta SATA HAT Controller (Go)
After=network.target

[Service]
Type=simple
ExecStart=/usr/local/bin/rocki-penta
KillSignal=SIGINT
EnvironmentFile=-/etc/rocki-penta.env
Restart=on-failure
RestartSec=5
User=root
Group=root
WorkingDirectory=/usr/local/bin
StandardOutput=journal
StandardError=journal

[Install]
WantedBy=multi-user.target
EOF
    fi
    
    # Reload systemd and enable service
    systemctl daemon-reload
    systemctl enable "$SERVICE_NAME"
    
    log_info "Systemd service installed and enabled"
}

# Function to check if reboot is needed
check_reboot_needed() {
    local needs_reboot=false
    
    # Check if I2C configuration was changed
    if [[ -f /boot/config.txt ]] || [[ -f /boot/firmware/config.txt ]]; then
        needs_reboot=true
    fi
    
    if [[ "$needs_reboot" == "true" ]]; then
        log_warn "System configuration has been modified."
        log_warn "A reboot is recommended for changes to take effect."
        echo
        read -p "Do you want to reboot now? (y/N): " -n 1 -r
        echo
        if [[ $REPLY =~ ^[Yy]$ ]]; then
            log_info "Rebooting system..."
            reboot
        else
            log_info "Please reboot manually when convenient."
        fi
    fi
}

# Main installation function
main() {
    log_info "Starting ROCK Pi Penta SATA HAT Controller (Go) installation..."
    
    # Check if running as root
    check_root
    
    # Detect OS and board
    detect_os
    local board_model=$(detect_board)
    
    # Install dependencies
    install_dependencies
    
    # Enable I2C
    enable_i2c
    
    # Create hardware environment file
    create_hardware_env "$board_model"
    
    # Install files
    install_binary "${1:-./rocki-penta}"
    install_config "${2:-./configs/rockpi-penta.conf}"
    install_service "${3:-./configs/systemd/rocki-penta.service}"
    
    # Check if reboot is needed
    check_reboot_needed
    
    log_info "Installation completed successfully!"
    log_info "You can start the service with: systemctl start rocki-penta"
    log_info "Or reboot to start automatically."
}

# Handle command line arguments
if [[ $# -eq 0 ]]; then
    main
else
    main "$@"
fi 