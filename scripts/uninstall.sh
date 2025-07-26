#!/bin/bash

# RockPi Penta Uninstall Script
# Removes all files, services, and configurations created during installation

set -e

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# Configuration
SERVICE_NAME="rockpi-penta"
BINARY_PATH="/usr/local/bin/rockpi-penta"
SERVICE_FILE="/etc/systemd/system/rockpi-penta.service"
CONFIG_FILE="/etc/rockpi-penta.conf"
ENV_FILE="/etc/rockpi-penta.env"

print_info() {
    echo -e "${BLUE}[INFO]${NC} $1"
}

print_success() {
    echo -e "${GREEN}[SUCCESS]${NC} $1"
}

print_warning() {
    echo -e "${YELLOW}[WARNING]${NC} $1"
}

print_error() {
    echo -e "${RED}[ERROR]${NC} $1"
}

print_header() {
    echo -e "${BLUE}==========================================${NC}"
    echo -e "${BLUE}    RockPi Penta Uninstall Script${NC}"
    echo -e "${BLUE}==========================================${NC}"
    echo
}

check_root() {
    if [[ $EUID -ne 0 ]]; then
        print_error "This script must be run as root (use sudo)"
        echo "Usage: sudo $0"
        exit 1
    fi
}

confirm_uninstall() {
    echo -e "${YELLOW}This will remove all RockPi Penta files and configurations:${NC}"
    echo "  • Service: $SERVICE_FILE"
    echo "  • Binary: $BINARY_PATH"
    echo "  • Config: $CONFIG_FILE"
    echo "  • Environment: $ENV_FILE"
    echo
    read -p "Are you sure you want to uninstall RockPi Penta? [y/N] " -n 1 -r
    echo
    if [[ ! $REPLY =~ ^[Yy]$ ]]; then
        print_info "Uninstall cancelled."
        exit 0
    fi
}

stop_and_disable_service() {
    print_info "Stopping and disabling RockPi Penta service..."
    
    # Check if service exists and is active
    if systemctl is-active --quiet "$SERVICE_NAME" 2>/dev/null; then
        print_info "Stopping service..."
        systemctl stop "$SERVICE_NAME" || print_warning "Failed to stop service"
        print_success "Service stopped"
    else
        print_info "Service is not running"
    fi
    
    # Check if service is enabled
    if systemctl is-enabled --quiet "$SERVICE_NAME" 2>/dev/null; then
        print_info "Disabling service..."
        systemctl disable "$SERVICE_NAME" || print_warning "Failed to disable service"
        print_success "Service disabled"
    else
        print_info "Service is not enabled"
    fi
    
    # Unmask if masked
    if systemctl is-masked --quiet "$SERVICE_NAME" 2>/dev/null; then
        print_info "Unmasking service..."
        systemctl unmask "$SERVICE_NAME" || print_warning "Failed to unmask service"
        print_success "Service unmasked"
    fi
}

remove_service_file() {
    if [[ -f "$SERVICE_FILE" ]]; then
        print_info "Removing service file: $SERVICE_FILE"
        rm -f "$SERVICE_FILE"
        print_success "Service file removed"
    else
        print_info "Service file not found: $SERVICE_FILE"
    fi
}

remove_binary() {
    if [[ -f "$BINARY_PATH" ]]; then
        print_info "Removing binary: $BINARY_PATH"
        rm -f "$BINARY_PATH"
        print_success "Binary removed"
    else
        print_info "Binary not found: $BINARY_PATH"
    fi
}

remove_config_files() {
    # Remove main config file
    if [[ -f "$CONFIG_FILE" ]]; then
        print_info "Removing configuration file: $CONFIG_FILE"
        rm -f "$CONFIG_FILE"
        print_success "Configuration file removed"
    else
        print_info "Configuration file not found: $CONFIG_FILE"
    fi
    
    # Remove environment file
    if [[ -f "$ENV_FILE" ]]; then
        print_info "Removing environment file: $ENV_FILE"
        rm -f "$ENV_FILE"
        print_success "Environment file removed"
    else
        print_info "Environment file not found: $ENV_FILE"
    fi
}

reload_systemd() {
    print_info "Reloading systemd daemon..."
    systemctl daemon-reload
    print_success "Systemd daemon reloaded"
}

cleanup_build_artifacts() {
    local script_dir=$(dirname "$(readlink -f "$0")")
    local project_root=$(dirname "$script_dir")
    local build_dir="$project_root/build"
    
    if [[ -d "$build_dir" ]]; then
        print_info "Removing build directory: $build_dir"
        rm -rf "$build_dir"
        print_success "Build directory removed"
    else
        print_info "Build directory not found: $build_dir"
    fi
}

verify_removal() {
    print_info "Verifying removal..."
    local all_clean=true
    
    # Check service
    if systemctl list-unit-files | grep -q "$SERVICE_NAME"; then
        print_warning "Service still exists in systemd"
        all_clean=false
    fi
    
    # Check files
    local files=("$SERVICE_FILE" "$BINARY_PATH" "$CONFIG_FILE" "$ENV_FILE")
    for file in "${files[@]}"; do
        if [[ -f "$file" ]]; then
            print_warning "File still exists: $file"
            all_clean=false
        fi
    done
    
    if $all_clean; then
        print_success "All RockPi Penta components successfully removed!"
    else
        print_warning "Some components may still exist - please check manually"
    fi
}

show_summary() {
    echo
    echo -e "${GREEN}==========================================${NC}"
    echo -e "${GREEN}         Uninstall Complete!${NC}"
    echo -e "${GREEN}==========================================${NC}"
    echo
    echo "The following components were removed:"
    echo "  ✓ RockPi Penta systemd service"
    echo "  ✓ Binary executable"
    echo "  ✓ Configuration files"
    echo "  ✓ Environment files"
    echo "  ✓ Build artifacts"
    echo
    echo "Your system has been restored to its previous state."
    echo "Thank you for using RockPi Penta!"
    echo
}

main() {
    print_header
    check_root
    confirm_uninstall
    
    echo
    print_info "Starting uninstall process..."
    
    stop_and_disable_service
    remove_service_file
    remove_binary
    remove_config_files
    reload_systemd
    cleanup_build_artifacts
    verify_removal
    
    show_summary
}

# Run main function
main "$@" 
