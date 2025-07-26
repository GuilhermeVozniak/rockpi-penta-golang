package config

import (
	"bufio"
	"fmt"
	"log"
	"os"
	"regexp"
	"strings"
)

// DeviceInfo holds information about the detected device
type DeviceInfo struct {
	BoardType      string
	Model          string
	ButtonChip     string
	ButtonLine     string
	FanChip        string
	FanLine        string
	HardwarePWM    bool
	I2CBus         string
	GPIOChipPath   string
	Confidence     int // 0-100, how confident we are in the detection
	DetectionNotes []string
}

// DetectDevice attempts to identify the current hardware platform
func DetectDevice() *DeviceInfo {
	device := &DeviceInfo{
		Confidence:     0,
		DetectionNotes: []string{},
	}

	// Try multiple detection methods
	detectFromCPUInfo(device)
	detectFromDeviceTree(device)
	detectFromGPIOChips(device)
	detectFromI2CBuses(device)

	// Set final configuration based on detection
	setDeviceConfiguration(device)

	log.Printf("Device detected: %s (confidence: %d%%)", device.BoardType, device.Confidence)
	for _, note := range device.DetectionNotes {
		log.Printf("Detection: %s", note)
	}

	return device
}

// detectFromCPUInfo reads /proc/cpuinfo to identify the board
func detectFromCPUInfo(device *DeviceInfo) {
	file, err := os.Open("/proc/cpuinfo")
	if err != nil {
		device.DetectionNotes = append(device.DetectionNotes, "Could not read /proc/cpuinfo")
		return
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.ToLower(scanner.Text())

		// Look for Raspberry Pi indicators
		if strings.Contains(line, "raspberry pi") {
			if strings.Contains(line, "model") {
				device.Model = strings.TrimSpace(scanner.Text())
				if strings.Contains(line, "pi 5") {
					device.BoardType = "raspberry-pi-5"
					device.Confidence = 90
				} else if strings.Contains(line, "pi 4") {
					device.BoardType = "raspberry-pi-4"
					device.Confidence = 90
				} else if strings.Contains(line, "pi 3") {
					device.BoardType = "raspberry-pi-3"
					device.Confidence = 85
				}
				device.DetectionNotes = append(device.DetectionNotes, "Detected from /proc/cpuinfo: "+device.Model)
			}
		}

		// Look for Rock Pi indicators
		if strings.Contains(line, "rockchip") || strings.Contains(line, "rk3399") || strings.Contains(line, "rk3588") {
			if strings.Contains(line, "rk3588") {
				device.BoardType = "rock-pi-5"
				device.Confidence = 85
			} else if strings.Contains(line, "rk3399") {
				device.BoardType = "rock-pi-4"
				device.Confidence = 85
			}
			device.DetectionNotes = append(device.DetectionNotes, "Detected Rockchip SoC from /proc/cpuinfo")
		}
	}
}

// detectFromDeviceTree reads device tree information
func detectFromDeviceTree(device *DeviceInfo) {
	// Try to read device tree model
	if model, err := os.ReadFile("/proc/device-tree/model"); err == nil {
		modelStr := strings.TrimSpace(string(model))
		device.Model = modelStr

		modelLower := strings.ToLower(modelStr)

		// Raspberry Pi detection
		if strings.Contains(modelLower, "raspberry pi 5") {
			device.BoardType = "raspberry-pi-5"
			device.Confidence = 95
		} else if strings.Contains(modelLower, "raspberry pi 4") {
			device.BoardType = "raspberry-pi-4"
			device.Confidence = 95
		} else if strings.Contains(modelLower, "raspberry pi 3") {
			device.BoardType = "raspberry-pi-3"
			device.Confidence = 95
		}

		// Rock Pi detection - more specific
		if strings.Contains(modelLower, "rock 5a") || strings.Contains(modelLower, "rock5a") {
			device.BoardType = "rock-5a"
			device.Confidence = 95
		} else if strings.Contains(modelLower, "rock 5") || strings.Contains(modelLower, "rock5") {
			device.BoardType = "rock-pi-5"
			device.Confidence = 90
		} else if strings.Contains(modelLower, "rock 4") || strings.Contains(modelLower, "rock4") {
			device.BoardType = "rock-pi-4"
			device.Confidence = 90
		} else if strings.Contains(modelLower, "rock 3c") || strings.Contains(modelLower, "rock3c") {
			device.BoardType = "rock-3c"
			device.Confidence = 90
		} else if strings.Contains(modelLower, "rock 3") || strings.Contains(modelLower, "rock3") {
			device.BoardType = "rock-pi-3"
			device.Confidence = 90
		}

		device.DetectionNotes = append(device.DetectionNotes, "Device tree model: "+modelStr)
	}

	// Check compatible string
	if compatible, err := os.ReadFile("/proc/device-tree/compatible"); err == nil {
		compatStr := string(compatible)
		compatLower := strings.ToLower(compatStr)

		// Raspberry Pi compatible strings
		if strings.Contains(compatStr, "raspberrypi,5") {
			device.BoardType = "raspberry-pi-5"
			device.Confidence = 95
		} else if strings.Contains(compatStr, "raspberrypi,4") {
			device.BoardType = "raspberry-pi-4"
			device.Confidence = 95
		} else if strings.Contains(compatStr, "raspberrypi,3") {
			device.BoardType = "raspberry-pi-3"
			device.Confidence = 95
		}

		// Rockchip SoC detection
		if strings.Contains(compatStr, "rockchip,rk3588") {
			if strings.Contains(compatLower, "rock-5a") {
				device.BoardType = "rock-5a"
				device.Confidence = 95
			} else {
				device.BoardType = "rock-pi-5"
				device.Confidence = 90
			}
		} else if strings.Contains(compatStr, "rockchip,rk3399") {
			device.BoardType = "rock-pi-4"
			device.Confidence = 90
		} else if strings.Contains(compatStr, "rockchip,rk3566") {
			if strings.Contains(compatLower, "rock-3c") {
				device.BoardType = "rock-3c"
				device.Confidence = 90
			} else {
				device.BoardType = "rock-pi-3"
				device.Confidence = 85
			}
		}

		device.DetectionNotes = append(device.DetectionNotes, "Device tree compatible: "+strings.ReplaceAll(compatStr, "\x00", ", "))
	}
}

// detectFromGPIOChips scans available GPIO chips
func detectFromGPIOChips(device *DeviceInfo) {
	chipDirs := []string{"/sys/class/gpio", "/dev"}

	for _, dir := range chipDirs {
		entries, err := os.ReadDir(dir)
		if err != nil {
			continue
		}

		var chips []string
		for _, entry := range entries {
			name := entry.Name()
			if strings.HasPrefix(name, "gpiochip") {
				chips = append(chips, name)
			}
		}

		if len(chips) > 0 {
			device.DetectionNotes = append(device.DetectionNotes, fmt.Sprintf("Found GPIO chips: %v", chips))

			// Raspberry Pi typically has gpiochip0 and gpiochip4
			// Rock Pi typically has gpiochip0, gpiochip1, etc.
			if containsChip(chips, "gpiochip4") && len(chips) <= 3 {
				if device.BoardType == "" {
					device.BoardType = "raspberry-pi-generic"
					device.Confidence = 60
				}
			} else if containsChip(chips, "gpiochip1") && len(chips) > 3 {
				if device.BoardType == "" {
					device.BoardType = "rock-pi-generic"
					device.Confidence = 60
				}
			}
		}
	}
}

// detectFromI2CBuses scans available I2C buses
func detectFromI2CBuses(device *DeviceInfo) {
	i2cDevs := []string{}

	entries, err := os.ReadDir("/dev")
	if err == nil {
		for _, entry := range entries {
			name := entry.Name()
			if strings.HasPrefix(name, "i2c-") {
				i2cDevs = append(i2cDevs, "/dev/"+name)
			}
		}
	}

	if len(i2cDevs) > 0 {
		device.DetectionNotes = append(device.DetectionNotes, fmt.Sprintf("Found I2C buses: %v", i2cDevs))

		// Set preferred I2C bus based on common configurations
		if containsString(i2cDevs, "/dev/i2c-1") {
			device.I2CBus = "/dev/i2c-1" // Common for Raspberry Pi
		} else if containsString(i2cDevs, "/dev/i2c-13") {
			device.I2CBus = "/dev/i2c-13" // Common for some ARM boards
		} else if len(i2cDevs) > 0 {
			device.I2CBus = i2cDevs[0] // Use first available
		}
	}
}

// setDeviceConfiguration sets the final GPIO configuration based on detected board type
func setDeviceConfiguration(device *DeviceInfo) {
	switch device.BoardType {
	// Raspberry Pi configurations
	case "raspberry-pi-5":
		device.ButtonChip = "4"
		device.ButtonLine = "17"
		device.FanChip = "4"
		device.FanLine = "27"
		device.HardwarePWM = false
		device.GPIOChipPath = "/dev/gpiochip4"
		if device.I2CBus == "" {
			device.I2CBus = "/dev/i2c-1"
		}

	case "raspberry-pi-4":
		device.ButtonChip = "0" // RPI4 uses gpiochip0 according to Python implementation
		device.ButtonLine = "17"
		device.FanChip = "0"
		device.FanLine = "27"
		device.HardwarePWM = false
		device.GPIOChipPath = "/dev/gpiochip0"
		if device.I2CBus == "" {
			device.I2CBus = "/dev/i2c-1"
		}

	case "raspberry-pi-3", "raspberry-pi-generic":
		device.ButtonChip = "0"
		device.ButtonLine = "17"
		device.FanChip = "0"
		device.FanLine = "27"
		device.HardwarePWM = false
		device.GPIOChipPath = "/dev/gpiochip0"
		if device.I2CBus == "" {
			device.I2CBus = "/dev/i2c-1"
		}

	// Rock Pi configurations
	case "rock-pi-5", "rock-5a":
		device.ButtonChip = "4"
		device.ButtonLine = "11"
		device.HardwarePWM = true
		device.GPIOChipPath = "/dev/gpiochip4"
		if device.I2CBus == "" {
			device.I2CBus = "/dev/i2c-8" // I2C8 for Rock 5A
		}
		// Rock 5A uses PWM chip 14 (or 1 for Armbian)
		device.DetectionNotes = append(device.DetectionNotes, "Rock 5A: PWM chip may vary (14 or 1) - check /sys/class/pwm/")

	case "rock-pi-4":
		device.ButtonChip = "4"
		device.ButtonLine = "18"
		device.HardwarePWM = true
		device.GPIOChipPath = "/dev/gpiochip4"
		if device.I2CBus == "" {
			device.I2CBus = "/dev/i2c-7" // I2C7 for Rock Pi 4
		}
		// Rock Pi 4 uses PWM chip 1 (or 0 for Armbian)
		device.DetectionNotes = append(device.DetectionNotes, "Rock Pi 4: PWM chip may vary (1 or 0) - check /sys/class/pwm/")

	case "rock-pi-3":
		device.ButtonChip = "3"
		device.ButtonLine = "20"
		device.HardwarePWM = true
		device.GPIOChipPath = "/dev/gpiochip3"
		if device.I2CBus == "" {
			device.I2CBus = "/dev/i2c-3" // I2C3 for Rock Pi 3
		}
		device.DetectionNotes = append(device.DetectionNotes, "Rock Pi 3: PWM chip 15")

	case "rock-3c":
		device.ButtonChip = "3"
		device.ButtonLine = "1"
		device.FanChip = "3"
		device.FanLine = "2"
		device.HardwarePWM = false // Rock 3C uses software PWM
		device.GPIOChipPath = "/dev/gpiochip3"
		if device.I2CBus == "" {
			device.I2CBus = "/dev/i2c-1" // Uses GPIO pins for I2C
		}
		device.DetectionNotes = append(device.DetectionNotes, "Rock 3C: Uses software PWM and GPIO I2C")

	case "rock-pi-generic":
		// Generic Rock Pi fallback
		device.ButtonChip = "4"
		device.ButtonLine = "18"
		device.HardwarePWM = true
		device.GPIOChipPath = "/dev/gpiochip4"
		if device.I2CBus == "" {
			device.I2CBus = "/dev/i2c-7"
		}
		device.DetectionNotes = append(device.DetectionNotes, "Generic Rock Pi configuration - may need manual adjustment")

	default:
		// Fallback to Raspberry Pi 5 defaults (most common current setup)
		device.BoardType = "unknown-fallback-rpi5"
		device.ButtonChip = "4"
		device.ButtonLine = "17"
		device.FanChip = "4"
		device.FanLine = "27"
		device.HardwarePWM = false
		device.GPIOChipPath = "/dev/gpiochip4"
		if device.I2CBus == "" {
			device.I2CBus = "/dev/i2c-1"
		}
		device.Confidence = 30
		device.DetectionNotes = append(device.DetectionNotes, "Unknown board, using Raspberry Pi 5 defaults")
	}
}

// VerifyHardwareAccess tests if the detected hardware configuration is accessible
func (d *DeviceInfo) VerifyHardwareAccess() map[string]bool {
	results := make(map[string]bool)

	// Test I2C bus access
	if d.I2CBus != "" {
		if _, err := os.Stat(d.I2CBus); err == nil {
			results["i2c_bus"] = true
		} else {
			results["i2c_bus"] = false
		}
	}

	// Test GPIO chip access
	if d.GPIOChipPath != "" {
		if _, err := os.Stat(d.GPIOChipPath); err == nil {
			results["gpio_chip"] = true
		} else {
			results["gpio_chip"] = false
		}
	}

	// Test PWM access (if hardware PWM is expected)
	if d.HardwarePWM {
		pwmPath := fmt.Sprintf("/sys/class/pwm/pwmchip%s", d.FanChip)
		if _, err := os.Stat(pwmPath); err == nil {
			results["hardware_pwm"] = true
		} else {
			results["hardware_pwm"] = false
		}
	} else {
		results["hardware_pwm"] = true // Not required
	}

	return results
}

// GetRecommendedEnvVars returns the environment variables that should be set
func (d *DeviceInfo) GetRecommendedEnvVars() map[string]string {
	vars := map[string]string{
		"BUTTON_CHIP":  d.ButtonChip,
		"BUTTON_LINE":  d.ButtonLine,
		"HARDWARE_PWM": boolToString(d.HardwarePWM),
		"I2C_BUS":      d.I2CBus,
		"SDA":          "SDA",
		"SCL":          "SCL",
		"OLED_RESET":   "D23",
	}

	// Add FAN configuration based on board type
	if d.HardwarePWM {
		// For Rock Pi boards using hardware PWM, use PWMCHIP
		pwmChip := d.getPWMChip()
		if pwmChip != "" {
			vars["PWMCHIP"] = pwmChip
		}
	} else {
		// For Raspberry Pi boards using software PWM, use FAN_CHIP/FAN_LINE
		vars["FAN_CHIP"] = d.FanChip
		vars["FAN_LINE"] = d.FanLine
	}

	return vars
}

// getPWMChip returns the appropriate PWM chip for Rock Pi boards
func (d *DeviceInfo) getPWMChip() string {
	switch d.BoardType {
	case "rock-5a":
		return "14" // Default, may be "1" for Armbian
	case "rock-pi-4":
		return "1" // Default, may be "0" for Armbian
	case "rock-pi-3":
		return "15"
	case "rock-3c":
		return "" // Uses software PWM
	default:
		return "1" // Generic fallback
	}
}

// PrintDetectionReport prints a detailed detection report
func (d *DeviceInfo) PrintDetectionReport() {
	fmt.Println("=== Device Detection Report ===")
	fmt.Printf("Board Type: %s\n", d.BoardType)
	if d.Model != "" {
		fmt.Printf("Model: %s\n", d.Model)
	}
	fmt.Printf("Confidence: %d%%\n", d.Confidence)
	fmt.Println()

	fmt.Println("Detected Configuration:")
	envVars := d.GetRecommendedEnvVars()
	for key, value := range envVars {
		fmt.Printf("  %s=%s\n", key, value)
	}
	fmt.Println()

	fmt.Println("Hardware Access Test:")
	access := d.VerifyHardwareAccess()
	for component, accessible := range access {
		status := "❌ FAIL"
		if accessible {
			status = "✅ OK"
		}
		fmt.Printf("  %s: %s\n", component, status)
	}
	fmt.Println()

	if len(d.DetectionNotes) > 0 {
		fmt.Println("Detection Notes:")
		for _, note := range d.DetectionNotes {
			fmt.Printf("  - %s\n", note)
		}
		fmt.Println()
	}

	if d.Confidence < 80 {
		fmt.Println("⚠️  Warning: Low confidence detection. Consider manually setting environment variables.")
	}
}

// Helper functions
func containsChip(chips []string, target string) bool {
	for _, chip := range chips {
		if chip == target {
			return true
		}
	}
	return false
}

func containsString(slice []string, target string) bool {
	for _, item := range slice {
		if item == target {
			return true
		}
	}
	return false
}

func boolToString(b bool) string {
	if b {
		return "1"
	}
	return "0"
}

// ParseGPIOFromKernel attempts to extract GPIO information from kernel modules
func ParseGPIOFromKernel() map[string]string {
	info := make(map[string]string)

	// Try to read from /sys/kernel/debug/gpio (requires root)
	if data, err := os.ReadFile("/sys/kernel/debug/gpio"); err == nil {
		lines := strings.Split(string(data), "\n")
		currentChip := ""
		chipRegex := regexp.MustCompile(`gpiochip(\d+):`)

		for _, line := range lines {
			if matches := chipRegex.FindStringSubmatch(line); len(matches) > 1 {
				currentChip = matches[1]
			}
			if currentChip != "" && strings.Contains(line, "gpio-") {
				// Extract GPIO line information
				parts := strings.Fields(line)
				if len(parts) > 0 {
					info[fmt.Sprintf("chip_%s", currentChip)] = line
				}
			}
		}
	}

	return info
}
