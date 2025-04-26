#!/bin/bash
set -e

echo "Building RockPi Penta Go service..."

# Create the scripts directory if it doesn't exist
mkdir -p "$(dirname "$0")"

# First, check if dependencies are installed
if [ "$(id -u)" -eq 0 ]; then
    echo "Checking dependencies..."
    ./scripts/install-dependencies.sh
else
    # Check if Go is installed without running the full dependency script
    if ! command -v go &> /dev/null && [ ! -x /usr/local/go/bin/go ]; then
        echo "Error: Go is not installed. Please run 'sudo ./scripts/install-dependencies.sh' first."
        exit 1
    fi
fi

# Check for Go in common installation paths
if command -v go &> /dev/null; then
    echo "Found Go in PATH"
elif [ -x /usr/local/go/bin/go ]; then
    echo "Found Go in /usr/local/go/bin, adding to PATH"
    export PATH=$PATH:/usr/local/go/bin
else
    echo "Error: Go is not installed. Please run 'sudo ./scripts/install-dependencies.sh' first."
    exit 1
fi

# Verify Go is now in path
if ! command -v go &> /dev/null; then
    echo "Error: Go is still not found in PATH after attempting to add it."
    exit 1
fi

# Display Go version
go_version=$(go version)
echo "Using $go_version"

# Check if in the correct directory
if [ ! -d "cmd/rockpi-penta-service" ]; then
    echo "Error: Please run this script from the root of the rockpi-penta-golang repository."
    exit 1
fi

# Create a directory for the output
mkdir -p build

# Build the application
echo "Compiling Go application..."
go build -o build/rockpi-penta-service ./cmd/rockpi-penta-service

if [ $? -eq 0 ]; then
    echo "Build successful! Binary created at build/rockpi-penta-service"
    
    # Ask to install if run as root
    if [ "$(id -u)" -eq 0 ]; then
        read -p "Do you want to install the service to /usr/local/bin? (y/n) " -n 1 -r
        echo
        if [[ $REPLY =~ ^[Yy]$ ]]; then
            echo "Installing binary to /usr/local/bin..."
            
            # Check if service is running and stop it temporarily
            SERVICE_WAS_RUNNING=false
            if systemctl is-active --quiet rockpi-penta; then
                echo "Service is currently running. Stopping it temporarily..."
                systemctl stop rockpi-penta
                SERVICE_WAS_RUNNING=true
            fi
            
            # Wait a moment for the process to fully stop
            sleep 1
            
            # Copy the binary
            cp build/rockpi-penta-service /usr/local/bin/
            chmod +x /usr/local/bin/rockpi-penta-service
            
            # Restart service if it was running
            if [ "$SERVICE_WAS_RUNNING" = true ]; then
                echo "Restarting the service..."
                systemctl start rockpi-penta
                echo "Service restarted successfully."
            fi
            
            echo "Installation complete!"
            echo "To start the service, run: sudo systemctl start rockpi-penta"
            echo "To enable at boot: sudo systemctl enable rockpi-penta"
        else
            echo "Skipping installation. You can manually copy the binary later."
        fi
    else
        echo "Note: To install the service, run this script as root: 'sudo ./scripts/build.sh'"
    fi
else
    echo "Build failed. Please check the errors above."
    exit 1
fi 