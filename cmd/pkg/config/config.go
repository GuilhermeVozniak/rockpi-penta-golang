package config

import (
	"bufio"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"
)

// Config holds all configuration for the application
type Config struct {
	// Fan configuration
	Fan FanConfig `yaml:"fan"`
	// Key configuration
	Key KeyConfig `yaml:"key"`
	// Time configuration
	Time TimeConfig `yaml:"time"`
	// Slider configuration
	Slider SliderConfig `yaml:"slider"`
	// OLED configuration
	OLED OLEDConfig `yaml:"oled"`
	// Hardware configuration from environment
	Hardware HardwareConfig `yaml:"hardware"`
}

type FanConfig struct {
	LV0 float64 `yaml:"lv0"`
	LV1 float64 `yaml:"lv1"`
	LV2 float64 `yaml:"lv2"`
	LV3 float64 `yaml:"lv3"`
}

type KeyConfig struct {
	Click string `yaml:"click"`
	Twice string `yaml:"twice"`
	Press string `yaml:"press"`
}

type TimeConfig struct {
	Twice float64 `yaml:"twice"`
	Press float64 `yaml:"press"`
}

type SliderConfig struct {
	Auto bool    `yaml:"auto"`
	Time float64 `yaml:"time"`
}

type OLEDConfig struct {
	Rotate bool `yaml:"rotate"`
	FTemp  bool `yaml:"f_temp"`
}

type HardwareConfig struct {
	SDA         string `yaml:"sda"`
	SCL         string `yaml:"scl"`
	OLEDReset   string `yaml:"oled_reset"`
	ButtonChip  string `yaml:"button_chip"`
	ButtonLine  string `yaml:"button_line"`
	FanChip     string `yaml:"fan_chip"`
	FanLine     string `yaml:"fan_line"`
	HardwarePWM string `yaml:"hardware_pwm"`
	PWMChip     string `yaml:"pwm_chip"`
}

// Load loads configuration from files and environment
func Load() (*Config, error) {
	cfg := &Config{}

	// Set default values
	cfg.setDefaults()

	// Load from config file
	if err := cfg.loadFromFile("/etc/rockpi-penta.conf"); err != nil {
		// Config file is optional, just log the error
		fmt.Printf("Warning: Could not load config file: %v\n", err)
	}

	// Load hardware configuration from environment
	if err := cfg.loadHardwareFromEnv(); err != nil {
		return nil, fmt.Errorf("failed to load hardware config: %w", err)
	}

	return cfg, nil
}

func (c *Config) setDefaults() {
	c.Fan = FanConfig{
		LV0: 35.0,
		LV1: 40.0,
		LV2: 45.0,
		LV3: 50.0,
	}
	c.Key = KeyConfig{
		Click: "slider",
		Twice: "switch",
		Press: "none",
	}
	c.Time = TimeConfig{
		Twice: 0.7,
		Press: 1.8,
	}
	c.Slider = SliderConfig{
		Auto: true,
		Time: 10.0,
	}
	c.OLED = OLEDConfig{
		Rotate: false,
		FTemp:  false,
	}
}

func (c *Config) loadFromFile(filename string) error {
	file, err := os.Open(filename)
	if err != nil {
		return err
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	currentSection := ""

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		if strings.HasPrefix(line, "[") && strings.HasSuffix(line, "]") {
			currentSection = strings.Trim(line, "[]")
			continue
		}

		if strings.Contains(line, "=") {
			parts := strings.SplitN(line, "=", 2)
			if len(parts) != 2 {
				continue
			}
			key := strings.TrimSpace(parts[0])
			value := strings.TrimSpace(parts[1])

			if err := c.setConfigValue(currentSection, key, value); err != nil {
				fmt.Printf("Warning: Could not set config value %s.%s=%s: %v\n", currentSection, key, value, err)
			}
		}
	}

	return scanner.Err()
}

func (c *Config) setConfigValue(section, key, value string) error {
	switch section {
	case "fan":
		return c.setFanValue(key, value)
	case "key":
		return c.setKeyValue(key, value)
	case "time":
		return c.setTimeValue(key, value)
	case "slider":
		return c.setSliderValue(key, value)
	case "oled":
		return c.setOLEDValue(key, value)
	}
	return fmt.Errorf("unknown section: %s", section)
}

func (c *Config) setFanValue(key, value string) error {
	val, err := strconv.ParseFloat(value, 64)
	if err != nil {
		return err
	}
	switch key {
	case "lv0":
		c.Fan.LV0 = val
	case "lv1":
		c.Fan.LV1 = val
	case "lv2":
		c.Fan.LV2 = val
	case "lv3":
		c.Fan.LV3 = val
	default:
		return fmt.Errorf("unknown fan key: %s", key)
	}
	return nil
}

func (c *Config) setKeyValue(key, value string) error {
	switch key {
	case "click":
		c.Key.Click = value
	case "twice":
		c.Key.Twice = value
	case "press":
		c.Key.Press = value
	default:
		return fmt.Errorf("unknown key key: %s", key)
	}
	return nil
}

func (c *Config) setTimeValue(key, value string) error {
	val, err := strconv.ParseFloat(value, 64)
	if err != nil {
		return err
	}
	switch key {
	case "twice":
		c.Time.Twice = val
	case "press":
		c.Time.Press = val
	default:
		return fmt.Errorf("unknown time key: %s", key)
	}
	return nil
}

func (c *Config) setSliderValue(key, value string) error {
	switch key {
	case "auto":
		val, err := strconv.ParseBool(value)
		if err != nil {
			return err
		}
		c.Slider.Auto = val
	case "time":
		val, err := strconv.ParseFloat(value, 64)
		if err != nil {
			return err
		}
		c.Slider.Time = val
	default:
		return fmt.Errorf("unknown slider key: %s", key)
	}
	return nil
}

func (c *Config) setOLEDValue(key, value string) error {
	switch key {
	case "rotate":
		val, err := strconv.ParseBool(value)
		if err != nil {
			return err
		}
		c.OLED.Rotate = val
	case "f-temp":
		val, err := strconv.ParseBool(value)
		if err != nil {
			return err
		}
		c.OLED.FTemp = val
	default:
		return fmt.Errorf("unknown oled key: %s", key)
	}
	return nil
}

func (c *Config) loadHardwareFromEnv() error {
	// Try to load from environment file first
	envFile := "/etc/rockpi-penta.env"
	if _, err := os.Stat(envFile); err == nil {
		if err := c.loadEnvFile(envFile); err != nil {
			return fmt.Errorf("failed to load environment file: %w", err)
		}
	} else {
		// Fallback to detecting board type and setting defaults
		if err := c.detectAndSetHardwareDefaults(); err != nil {
			return fmt.Errorf("failed to detect hardware: %w", err)
		}
	}

	// Override with actual environment variables if set
	c.loadFromEnvVars()

	return nil
}

func (c *Config) loadEnvFile(filename string) error {
	file, err := os.Open(filename)
	if err != nil {
		return err
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		if strings.Contains(line, "=") {
			parts := strings.SplitN(line, "=", 2)
			if len(parts) == 2 {
				key := strings.TrimSpace(parts[0])
				value := strings.TrimSpace(parts[1])
				c.setHardwareValue(key, value)
			}
		}
	}

	return scanner.Err()
}

func (c *Config) loadFromEnvVars() {
	envVars := map[string]*string{
		"SDA":          &c.Hardware.SDA,
		"SCL":          &c.Hardware.SCL,
		"OLED_RESET":   &c.Hardware.OLEDReset,
		"BUTTON_CHIP":  &c.Hardware.ButtonChip,
		"BUTTON_LINE":  &c.Hardware.ButtonLine,
		"FAN_CHIP":     &c.Hardware.FanChip,
		"FAN_LINE":     &c.Hardware.FanLine,
		"HARDWARE_PWM": &c.Hardware.HardwarePWM,
		"PWMCHIP":      &c.Hardware.PWMChip,
	}

	for envVar, configField := range envVars {
		if value := os.Getenv(envVar); value != "" {
			*configField = value
		}
	}
}

func (c *Config) setHardwareValue(key, value string) {
	switch key {
	case "SDA":
		c.Hardware.SDA = value
	case "SCL":
		c.Hardware.SCL = value
	case "OLED_RESET":
		c.Hardware.OLEDReset = value
	case "BUTTON_CHIP":
		c.Hardware.ButtonChip = value
	case "BUTTON_LINE":
		c.Hardware.ButtonLine = value
	case "FAN_CHIP":
		c.Hardware.FanChip = value
	case "FAN_LINE":
		c.Hardware.FanLine = value
	case "HARDWARE_PWM":
		c.Hardware.HardwarePWM = value
	case "PWMCHIP":
		c.Hardware.PWMChip = value
	}
}

func (c *Config) detectAndSetHardwareDefaults() error {
	// Read device tree model to detect board type
	model, err := c.readDeviceTreeModel()
	if err != nil {
		return fmt.Errorf("failed to read device tree model: %w", err)
	}

	// Set hardware defaults based on board type
	switch {
	case strings.Contains(model, "Raspberry Pi 5"):
		c.Hardware = HardwareConfig{
			SDA:         "SDA",
			SCL:         "SCL",
			OLEDReset:   "D23",
			ButtonChip:  "4",
			ButtonLine:  "17",
			FanChip:     "4",
			FanLine:     "27",
			HardwarePWM: "0",
		}
	case strings.Contains(model, "Raspberry Pi 4"):
		c.Hardware = HardwareConfig{
			SDA:         "SDA",
			SCL:         "SCL",
			OLEDReset:   "D23",
			ButtonChip:  "0",
			ButtonLine:  "17",
			FanChip:     "0",
			FanLine:     "27",
			HardwarePWM: "0",
		}
	default:
		// Default to Raspberry Pi 4 configuration for unknown boards
		c.Hardware = HardwareConfig{
			SDA:         "SDA",
			SCL:         "SCL",
			OLEDReset:   "D23",
			ButtonChip:  "0",
			ButtonLine:  "17",
			FanChip:     "0",
			FanLine:     "27",
			HardwarePWM: "0",
		}
		fmt.Printf("Warning: Unknown board type '%s', using default Raspberry Pi 4 configuration\n", model)
	}

	return nil
}

func (c *Config) readDeviceTreeModel() (string, error) {
	// Try multiple possible locations for device tree model
	locations := []string{
		"/proc/device-tree/model",
		"/sys/firmware/devicetree/base/model",
	}

	for _, location := range locations {
		if data, err := os.ReadFile(location); err == nil {
			// Remove null bytes and trim
			model := strings.TrimSpace(strings.ReplaceAll(string(data), "\x00", ""))
			if model != "" {
				return model, nil
			}
		}
	}

	// Fallback: try to detect from /proc/cpuinfo
	if data, err := os.ReadFile("/proc/cpuinfo"); err == nil {
		lines := strings.Split(string(data), "\n")
		for _, line := range lines {
			if strings.HasPrefix(line, "Model") {
				if parts := strings.SplitN(line, ":", 2); len(parts) == 2 {
					return strings.TrimSpace(parts[1]), nil
				}
			}
		}
	}

	return "", fmt.Errorf("could not detect device model")
}

// GetFanDutyCycle returns the fan duty cycle based on temperature
func (c *Config) GetFanDutyCycle(temp float64) float64 {
	if temp >= c.Fan.LV3 {
		return 0.0 // 100% speed (inverted logic)
	} else if temp >= c.Fan.LV2 {
		return 0.25 // 75% speed
	} else if temp >= c.Fan.LV1 {
		return 0.5 // 50% speed
	} else if temp >= c.Fan.LV0 {
		return 0.75 // 25% speed
	}
	return 0.999 // Fan off
}

// GetSliderInterval returns the slider interval as a time.Duration
func (c *Config) GetSliderInterval() time.Duration {
	return time.Duration(c.Slider.Time * float64(time.Second))
}

// GetDoubleClickTimeout returns the double click timeout as a time.Duration
func (c *Config) GetDoubleClickTimeout() time.Duration {
	return time.Duration(c.Time.Twice * float64(time.Second))
}

// GetLongPressTimeout returns the long press timeout as a time.Duration
func (c *Config) GetLongPressTimeout() time.Duration {
	return time.Duration(c.Time.Press * float64(time.Second))
}
