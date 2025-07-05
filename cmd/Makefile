# ROCK Pi Penta SATA HAT Controller (Go) - Makefile

.PHONY: build build-all test clean install help

# Default target
.DEFAULT_GOAL := help

# Build for Raspberry Pi (ARM64 and ARM32)
build:
	@./scripts/build.sh

# Build for all supported architectures
build-all:
	@./scripts/build.sh build-all

# Run tests
test:
	@./scripts/build.sh test

# Clean build artifacts
clean:
	@./scripts/build.sh clean

# Install locally (requires sudo)
install:
	@./scripts/build.sh install

# Make scripts executable
scripts-executable:
	@chmod +x scripts/*.sh

# Show help
help:
	@echo "ROCK Pi Penta SATA HAT Controller (Go) - Build System"
	@echo ""
	@echo "Usage:"
	@echo "  make build       Build for Raspberry Pi (ARM64 and ARM32)"
	@echo "  make build-all   Build for all supported architectures"
	@echo "  make test        Run tests"
	@echo "  make clean       Clean build artifacts"
	@echo "  make install     Install locally (requires sudo)"
	@echo "  make help        Show this help message"
	@echo ""
	@echo "Environment variables:"
	@echo "  VERSION          Override version string"
	@echo "  CGO_ENABLED      Override CGO setting (default: 0)"
	@echo ""
	@echo "Examples:"
	@echo "  make build                    # Build for Raspberry Pi"
	@echo "  make build-all               # Build for all architectures"
	@echo "  VERSION=v1.0.0 make build    # Build with custom version" 