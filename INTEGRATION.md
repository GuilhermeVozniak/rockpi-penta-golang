# RockPi Penta & base-linux-setup Integration

This document describes the enhanced integration between the RockPi Penta Golang project and the [base-linux-setup](https://github.com/GuilhermeVozniak/base-linux-setup) tool.

## Overview

The integration allows the RockPi Penta project to use the released version of base-linux-setup for intelligent dependency installation with advanced features like:

- **Conditional Execution**: Tasks only run when needed
- **Dependency Management**: Automatic task ordering based on dependencies
- **Idempotent Operations**: Safe to run multiple times
- **Architecture Detection**: Automatic binary selection for different platforms

## Files Added/Modified

### New Files

1. **`scripts/rockpi-penta-setup.json`** - Configuration file for base-linux-setup
2. **`scripts/install-with-base-linux-setup.sh`** - Enhanced installation script
3. **`INTEGRATION.md`** - This documentation file

### Modified Files

1. **`README.md`** - Updated with new installation methods and documentation

## Enhanced JSON Configuration

The `scripts/rockpi-penta-setup.json` file uses the enhanced JSON schema with these new features:

### Conditional Tasks

Tasks can include a `condition` field with a shell command that determines if the task should run:

```json
{
  "name": "Install Go 1.24.2",
  "condition": "! command -v go >/dev/null 2>&1 || [ \"$(go version | awk '{print $3}' | sed 's/go//' | cut -d. -f1-2 2>/dev/null || echo '0.0')\" != \"1.24\" ]",
  "type": "script",
  "script": "# Go installation script"
}
```

This task only runs if:
- Go is not installed, OR
- The installed Go version is not 1.24

### Task Dependencies

Tasks can depend on other tasks using the `depends_on` field:

```json
{
  "name": "Install Go 1.24.2",
  "depends_on": ["Update System Packages"],
  "type": "script",
  "script": "# Go installation script"
}
```

This ensures "Update System Packages" runs before "Install Go 1.24.2".

### Complete Task Flow

The configuration defines this dependency chain:

```
Update System Packages
├── Install Go 1.24.2
└── Install System Dependencies
    └── Enable I2C Interface
        └── Create RockPi Penta Configuration
            └── Create Systemd Service
                └── Reload Systemd
                    └── Verify Installation
```

## Enhanced base-linux-setup Features

### New CLI Flag

Added `--config` flag to load external configuration files:

```bash
base-linux-setup --config /path/to/config.json
```

### Enhanced Task Struct

```go
type Task struct {
    Name        string   `json:"name"`
    Description string   `json:"description"`
    Type        string   `json:"type"`
    Commands    []string `json:"commands"`
    Script      string   `json:"script"`
    Elevated    bool     `json:"elevated"`
    Optional    bool     `json:"optional"`
    Condition   string   `json:"condition,omitempty"`   // NEW: Shell command to check if task should run
    DependsOn   []string `json:"depends_on,omitempty"` // NEW: Task dependencies
}
```

### Dependency Resolution

The tool automatically:
1. Validates all dependencies exist
2. Detects circular dependencies
3. Sorts tasks using topological sort
4. Executes tasks in proper order

### Condition Checking

Before executing each task, the tool:
1. Runs the condition command (if specified)
2. Skips the task if condition returns non-zero exit code
3. Continues with normal execution if condition passes

## Installation Methods Comparison

### Method 1: Enhanced Installation (Recommended)

```bash
chmod +x scripts/install-with-base-linux-setup.sh
sudo ./scripts/install-with-base-linux-setup.sh
```

**Advantages:**
- ✅ Automatic architecture detection
- ✅ Downloads appropriate binary for your system
- ✅ Intelligent condition checking
- ✅ Proper dependency ordering
- ✅ Idempotent (safe to re-run)
- ✅ Uses latest released version of base-linux-setup

**Process:**
1. Detects system architecture (x86_64, arm64, armv7l, etc.)
2. Downloads appropriate base-linux-setup binary from GitHub releases
3. Loads RockPi configuration with enhanced features
4. Executes tasks with dependency resolution

### Method 2: Traditional Installation

```bash
sudo ./scripts/install-dependencies.sh
```

**Characteristics:**
- ⚠️ No condition checking (always installs)
- ⚠️ No dependency management
- ⚠️ May reinstall existing components
- ✅ Direct control over process
- ✅ No external downloads required

## Smart Condition Examples

### Go Version Check
```bash
! command -v go >/dev/null 2>&1 || [ "$(go version | awk '{print $3}' | sed 's/go//' | cut -d. -f1-2 2>/dev/null || echo '0.0')" != "1.24" ]
```

This condition returns true (task should run) if:
- Go command is not found, OR
- Go version is not 1.24.x

### I2C Module Check
```bash
! lsmod | grep -q i2c_dev
```

This condition returns true if the i2c_dev module is not loaded.

### File Existence Check
```bash
[ ! -f "/etc/rockpi-penta.conf" ]
```

This condition returns true if the configuration file doesn't exist.

### Package Installation Check
```bash
! dpkg -l | grep -q i2c-tools
```

This condition returns true if the i2c-tools package is not installed.

## Architecture Support

The installation script automatically detects and downloads the appropriate binary:

| Architecture | Binary |
|--------------|--------|
| x86_64 (Intel/AMD 64-bit) | `base-linux-setup-linux-amd64` |
| aarch64/arm64 (ARM 64-bit) | `base-linux-setup-linux-arm64` |
| armv7l/armv6l (ARM 32-bit) | `base-linux-setup-linux-arm` |
| macOS Intel | `base-linux-setup-darwin-amd64` |
| macOS Apple Silicon | `base-linux-setup-darwin-arm64` |

## Benefits of Integration

### For Users
1. **Faster Setup**: Only installs missing components
2. **Reliable**: Proper dependency ordering prevents failures
3. **Consistent**: Same experience across different systems
4. **Safe**: Idempotent operations prevent conflicts

### For Developers
1. **Maintainable**: Configuration is declarative JSON
2. **Extensible**: Easy to add new tasks with dependencies
3. **Testable**: Conditions can be tested independently
4. **Reusable**: base-linux-setup can be used by other projects

### For System Administrators
1. **Auditable**: Clear dependency chains and conditions
2. **Controllable**: Can modify configuration as needed
3. **Repeatable**: Consistent results across environments
4. **Debuggable**: Clear logging and error messages

## Future Enhancements

### Potential Improvements
1. **Rollback Support**: Ability to undo changes
2. **Dry Run Mode**: Preview what would be installed
3. **Custom Condition Scripts**: More complex condition logic
4. **Parallel Execution**: Run independent tasks in parallel
5. **Progress Reporting**: Better feedback during installation

### Configuration Extensions
1. **Pre/Post Hooks**: Scripts to run before/after tasks
2. **Environment Variables**: Task-specific environment setup
3. **Timeout Handling**: Maximum execution time for tasks
4. **Retry Logic**: Automatic retry on temporary failures

## Testing

To test the configuration without making changes:

```bash
# Test configuration parsing
base-linux-setup --config scripts/rockpi-penta-setup.json detect

# View task information
base-linux-setup --config scripts/rockpi-penta-setup.json list-presets
```

## Troubleshooting

### Common Issues

1. **Download Failures**: Check internet connection and GitHub access
2. **Permission Errors**: Ensure script is run with appropriate privileges
3. **Architecture Errors**: Verify your system architecture is supported
4. **Dependency Failures**: Check that all task names in `depends_on` exist

### Debug Mode

For verbose output, check the base-linux-setup logs during execution.

## Contributing

To modify the installation process:

1. Edit `scripts/rockpi-penta-setup.json`
2. Test the configuration with base-linux-setup
3. Update documentation if needed
4. Test on target systems

## License

This integration maintains the same license as both projects (MIT License). 