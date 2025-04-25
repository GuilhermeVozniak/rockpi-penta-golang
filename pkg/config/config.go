package config

import (
	"bufio"
	"fmt"
	"os"
	"strconv"
	"strings"
)

// Config holds the application configuration.
type Config struct {
	Fan struct {
		Lv0 float64 // Temperature threshold for level 0
		Lv1 float64 // Temperature threshold for level 1
		Lv2 float64 // Temperature threshold for level 2
		Lv3 float64 // Temperature threshold for level 3
	}
	Key struct {
		Click string // Action for single click
		Twice string // Action for double click
		Press string // Action for long press
	}
	Time struct {
		Twice float64 // Time window for double click (seconds)
		Press float64 // Time threshold for long press (seconds)
	}
	Slider struct {
		Auto bool    // Enable auto-sliding pages on OLED
		Time float64 // Interval for auto-sliding (seconds)
	}
	OLED struct {
		Rotate bool // Rotate OLED display 180 degrees
		FTemp  bool // Use Fahrenheit for temperature display
	}
	// Internal state, not directly from config file
	Idx AtomicInt  // Current OLED page index
	Run AtomicBool // Fan enabled state
}

const defaultConfigPath = "/etc/rockpi-penta.conf"

// loadDefaults sets the default configuration values.
func loadDefaults() *Config {
	return &Config{
		Fan: struct {
			Lv0 float64
			Lv1 float64
			Lv2 float64
			Lv3 float64
		}{Lv0: 35, Lv1: 40, Lv2: 45, Lv3: 50},
		Key: struct {
			Click string
			Twice string
			Press string
		}{Click: "slider", Twice: "switch", Press: "none"},
		Time: struct {
			Twice float64
			Press float64
		}{Twice: 0.7, Press: 1.8},
		Slider: struct {
			Auto bool
			Time float64
		}{Auto: true, Time: 10},
		OLED: struct {
			Rotate bool
			FTemp  bool
		}{Rotate: false, FTemp: false},
		Idx: *NewAtomicInt(-1),
		Run: *NewAtomicBool(true),
	}
}

// parseConfig parses the configuration file line by line.
// It's a simplified parser assuming "key = value" format within sections like [fan].
func parseConfig(path string) (*Config, error) {
	conf := loadDefaults() // Start with defaults

	file, err := os.Open(path)
	if err != nil {
		if os.IsNotExist(err) {
			fmt.Printf("Config file %s not found, using defaults.\n", path)
			return conf, nil // Return defaults if file doesn't exist
		}
		return nil, fmt.Errorf("error opening config file %s: %w", path, err)
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	currentSection := ""
	lineNumber := 0

	for scanner.Scan() {
		lineNumber++
		line := strings.TrimSpace(scanner.Text())

		// Skip comments and empty lines
		if len(line) == 0 || line[0] == '#' || line[0] == ';' {
			continue
		}

		// Check for section headers
		if strings.HasPrefix(line, "[") && strings.HasSuffix(line, "]") {
			currentSection = strings.ToLower(strings.TrimSpace(line[1 : len(line)-1]))
			continue
		}

		// Parse key-value pairs
		parts := strings.SplitN(line, "=", 2)
		if len(parts) != 2 {
			fmt.Printf("Warning: Skipping malformed line %d in %s: %s\n", lineNumber, path, line)
			continue
		}
		key := strings.ToLower(strings.TrimSpace(parts[0]))
		value := strings.TrimSpace(parts[1])

		// Assign values based on section and key
		var parseErr error
		switch currentSection {
		case "fan":
			var fValue float64
			fValue, parseErr = strconv.ParseFloat(value, 64)
			if parseErr == nil {
				switch key {
				case "lv0":
					conf.Fan.Lv0 = fValue
				case "lv1":
					conf.Fan.Lv1 = fValue
				case "lv2":
					conf.Fan.Lv2 = fValue
				case "lv3":
					conf.Fan.Lv3 = fValue
				default:
					fmt.Printf("Warning: Unknown key '%s' in section '[%s]' at line %d\n", key, currentSection, lineNumber)
				}
			}
		case "key":
			switch key {
			case "click":
				conf.Key.Click = value
			case "twice":
				conf.Key.Twice = value
			case "press":
				conf.Key.Press = value
			default:
				fmt.Printf("Warning: Unknown key '%s' in section '[%s]' at line %d\n", key, currentSection, lineNumber)
			}
		case "time":
			var fValue float64
			fValue, parseErr = strconv.ParseFloat(value, 64)
			if parseErr == nil {
				switch key {
				case "twice":
					conf.Time.Twice = fValue
				case "press":
					conf.Time.Press = fValue
				default:
					fmt.Printf("Warning: Unknown key '%s' in section '[%s]' at line %d\n", key, currentSection, lineNumber)
				}
			}
		case "slider":
			switch key {
			case "auto":
				conf.Slider.Auto, parseErr = strconv.ParseBool(value)
			case "time":
				conf.Slider.Time, parseErr = strconv.ParseFloat(value, 64)
			default:
				fmt.Printf("Warning: Unknown key '%s' in section '[%s]' at line %d\n", key, currentSection, lineNumber)
			}
		case "oled":
			switch key {
			case "rotate":
				conf.OLED.Rotate, parseErr = strconv.ParseBool(value)
			case "f-temp": // Match Python config key
				conf.OLED.FTemp, parseErr = strconv.ParseBool(value)
			default:
				fmt.Printf("Warning: Unknown key '%s' in section '[%s]' at line %d\n", key, currentSection, lineNumber)
			}
		default:
			// Only warn about keys if they are within a known section structure pattern
			if currentSection != "" {
				fmt.Printf("Warning: Unknown section '[%s]' at line %d\n", currentSection, lineNumber)
			}
		}

		if parseErr != nil {
			fmt.Printf("Warning: Error parsing value for '%s' in section '[%s]' at line %d: %v\n", key, currentSection, lineNumber, parseErr)
		}
	}

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("error reading config file %s: %w", path, err)
	}

	fmt.Printf("Configuration loaded successfully from %s\n", path)
	return conf, nil
}

// LoadConfig loads configuration, trying the default path first, then falling back to defaults.
func LoadConfig() (*Config, error) {
	// Try loading from the default path
	conf, err := parseConfig(defaultConfigPath)
	if err != nil {
		fmt.Printf("Error loading config from %s: %v. Using defaults.\n", defaultConfigPath, err)
		// If parsing failed (not just file not found), return the error
		if !os.IsNotExist(err) {
			// Return default config along with the parsing error
			return loadDefaults(), fmt.Errorf("config parsing error: %w", err)
		}
		// If file not found, parseConfig already printed a message and returned defaults
	}
	// If err was nil or os.IsNotExist, conf holds either parsed or default config
	return conf, nil
}
