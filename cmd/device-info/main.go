package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/GuilhermeVozniak/rockpi-penta-golang/pkg/config"
)

func main() {
	var (
		showEnvVars = flag.Bool("env", false, "Show environment variables that should be set")
		showExport  = flag.Bool("export", false, "Show export commands for detected environment variables")
		verify      = flag.Bool("verify", false, "Verify hardware access with current configuration")
		verbose     = flag.Bool("v", false, "Verbose output")
	)
	flag.Parse()

	// Perform device detection
	device := config.DetectDevice()

	if *showExport {
		// Show export commands
		fmt.Println("# Add these to your shell environment or /etc/rockpi-penta.env:")
		envVars := device.GetRecommendedEnvVars()
		for key, value := range envVars {
			fmt.Printf("export %s=%s\n", key, value)
		}
		return
	}

	if *showEnvVars {
		// Show just the environment variables
		envVars := device.GetRecommendedEnvVars()
		for key, value := range envVars {
			fmt.Printf("%s=%s\n", key, value)
		}
		return
	}

	if *verify {
		// Verify current configuration
		fmt.Println("=== Hardware Verification ===")

		// Load current config
		cfg := config.Load()
		hwCfg := config.HWConfig

		fmt.Printf("Current Configuration:\n")
		fmt.Printf("  BUTTON_CHIP=%s, BUTTON_LINE=%s\n", hwCfg.ButtonChip, hwCfg.ButtonLine)
		fmt.Printf("  FAN_CHIP=%s, FAN_LINE=%s\n", hwCfg.FanChip, hwCfg.FanLine)
		fmt.Printf("  HARDWARE_PWM=%t\n", hwCfg.HardwarePWM)
		fmt.Printf("  I2C_BUS=%s\n", os.Getenv("I2C_BUS"))
		fmt.Println()

		// Test hardware access
		access := device.VerifyHardwareAccess()
		fmt.Println("Hardware Access Test:")
		allGood := true
		for component, accessible := range access {
			status := "❌ FAIL"
			if accessible {
				status = "✅ OK"
			} else {
				allGood = false
			}
			fmt.Printf("  %s: %s\n", component, status)
		}

		if allGood {
			fmt.Println("\n✅ All hardware components are accessible!")
		} else {
			fmt.Println("\n⚠️  Some hardware components are not accessible.")
			fmt.Println("   This might be normal if the hardware is not connected.")
		}

		// Compare with detected values
		detected := device.GetRecommendedEnvVars()
		fmt.Println("\nConfiguration Comparison:")
		differences := 0

		if hwCfg.ButtonChip != detected["BUTTON_CHIP"] {
			fmt.Printf("  BUTTON_CHIP: current=%s, detected=%s ⚠️\n", hwCfg.ButtonChip, detected["BUTTON_CHIP"])
			differences++
		}
		if hwCfg.ButtonLine != detected["BUTTON_LINE"] {
			fmt.Printf("  BUTTON_LINE: current=%s, detected=%s ⚠️\n", hwCfg.ButtonLine, detected["BUTTON_LINE"])
			differences++
		}
		if hwCfg.FanChip != detected["FAN_CHIP"] {
			fmt.Printf("  FAN_CHIP: current=%s, detected=%s ⚠️\n", hwCfg.FanChip, detected["FAN_CHIP"])
			differences++
		}
		if hwCfg.FanLine != detected["FAN_LINE"] {
			fmt.Printf("  FAN_LINE: current=%s, detected=%s ⚠️\n", hwCfg.FanLine, detected["FAN_LINE"])
			differences++
		}
		hwPwmStr := "0"
		if hwCfg.HardwarePWM {
			hwPwmStr = "1"
		}
		if hwPwmStr != detected["HARDWARE_PWM"] {
			fmt.Printf("  HARDWARE_PWM: current=%s, detected=%s ⚠️\n", hwPwmStr, detected["HARDWARE_PWM"])
			differences++
		}

		if differences == 0 {
			fmt.Println("  ✅ Configuration matches detected values")
		} else {
			fmt.Printf("  ⚠️  %d configuration differences found\n", differences)
			fmt.Println("     Consider updating your environment variables")
		}

		_ = cfg // Use cfg to avoid unused variable warning
		return
	}

	// Default: show full detection report
	device.PrintDetectionReport()

	if *verbose {
		fmt.Println("=== Additional System Information ===")

		// Show current environment variables
		fmt.Println("\nCurrent Environment Variables:")
		envVars := []string{"BUTTON_CHIP", "BUTTON_LINE", "FAN_CHIP", "FAN_LINE", "HARDWARE_PWM", "I2C_BUS", "SDA", "SCL", "OLED_RESET"}
		for _, envVar := range envVars {
			if value := os.Getenv(envVar); value != "" {
				fmt.Printf("  %s=%s\n", envVar, value)
			} else {
				fmt.Printf("  %s=(not set)\n", envVar)
			}
		}

		// Show GPIO information if available
		fmt.Println("\nGPIO Information:")
		gpioInfo := config.ParseGPIOFromKernel()
		if len(gpioInfo) > 0 {
			for key, value := range gpioInfo {
				fmt.Printf("  %s: %s\n", key, value)
			}
		} else {
			fmt.Println("  (GPIO debug information not available - requires root)")
		}
	}
}
