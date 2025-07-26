package config

import (
	"fmt"
	"log"
	"os"
	"strconv"
	"sync"
	"sync/atomic"

	"gopkg.in/ini.v1"
)

// Config holds all configuration values
type Config struct {
	Fan    FanConfig    `ini:"fan"`
	Key    KeyConfig    `ini:"key"`
	Time   TimeConfig   `ini:"time"`
	Slider SliderConfig `ini:"slider"`
	OLED   OLEDConfig   `ini:"oled"`

	// Runtime state
	RunState    *int32
	SliderIndex *int32
	DiskDevices []string
	diskMutex   sync.RWMutex
}

type FanConfig struct {
	Lv0 float64 `ini:"lv0"`
	Lv1 float64 `ini:"lv1"`
	Lv2 float64 `ini:"lv2"`
	Lv3 float64 `ini:"lv3"`
}

type KeyConfig struct {
	Click string `ini:"click"`
	Twice string `ini:"twice"`
	Press string `ini:"press"`
}

type TimeConfig struct {
	Twice float64 `ini:"twice"`
	Press float64 `ini:"press"`
}

type SliderConfig struct {
	Auto bool    `ini:"auto"`
	Time float64 `ini:"time"`
}

type OLEDConfig struct {
	Rotate bool `ini:"rotate"`
	FTemp  bool `ini:"f-temp"`
}

// Hardware environment configuration
type HardwareConfig struct {
	SDA         string
	SCL         string
	OLEDReset   string
	ButtonChip  string
	ButtonLine  string
	FanChip     string
	FanLine     string
	HardwarePWM bool
}

var (
	GlobalConfig *Config
	HWConfig     *HardwareConfig
	once         sync.Once
)

// Load reads configuration from /etc/rockpi-penta.conf
func Load() *Config {
	once.Do(func() {
		GlobalConfig = &Config{
			RunState:    new(int32),
			SliderIndex: new(int32),
		}
		atomic.StoreInt32(GlobalConfig.RunState, 1)
		atomic.StoreInt32(GlobalConfig.SliderIndex, -1)

		// Set defaults
		setDefaults(GlobalConfig)

		// Try to load from file
		if err := loadFromFile(GlobalConfig); err != nil {
			log.Printf("Warning: Could not load config file, using defaults: %v", err)
		}

		// Load hardware config from environment
		HWConfig = loadHardwareConfig()
	})
	return GlobalConfig
}

func setDefaults(c *Config) {
	c.Fan = FanConfig{
		Lv0: 35,
		Lv1: 40,
		Lv2: 45,
		Lv3: 50,
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
		Time: 10,
	}
	c.OLED = OLEDConfig{
		Rotate: false,
		FTemp:  false,
	}
}

func loadFromFile(c *Config) error {
	cfg, err := ini.Load("/etc/rockpi-penta.conf")
	if err != nil {
		return err
	}

	return cfg.MapTo(c)
}

func loadHardwareConfig() *HardwareConfig {
	// First try to detect device automatically
	var defaults map[string]string

	if shouldAutoDetect() {
		log.Println("Auto-detecting hardware configuration...")
		device := DetectDevice()
		defaults = device.GetRecommendedEnvVars()

		// Print detection summary
		log.Printf("Detected board: %s (confidence: %d%%)", device.BoardType, device.Confidence)

		// Verify hardware access
		access := device.VerifyHardwareAccess()
		allGood := true
		for component, accessible := range access {
			if !accessible {
				log.Printf("Warning: %s not accessible", component)
				allGood = false
			}
		}

		if allGood && device.Confidence >= 80 {
			log.Println("Hardware auto-detection successful")
		} else {
			log.Println("Hardware auto-detection completed with warnings")
		}
	} else {
		// Use default values if auto-detection is disabled
		defaults = map[string]string{
			"SDA":          "SDA",
			"SCL":          "SCL",
			"OLED_RESET":   "D23",
			"BUTTON_CHIP":  "4",
			"BUTTON_LINE":  "17",
			"FAN_CHIP":     "4",
			"FAN_LINE":     "27",
			"HARDWARE_PWM": "0",
			"I2C_BUS":      "/dev/i2c-1",
		}
	}

	hw := &HardwareConfig{
		SDA:         getEnvDefaultWithFallback("SDA", defaults["SDA"]),
		SCL:         getEnvDefaultWithFallback("SCL", defaults["SCL"]),
		OLEDReset:   getEnvDefaultWithFallback("OLED_RESET", defaults["OLED_RESET"]),
		ButtonChip:  getEnvDefaultWithFallback("BUTTON_CHIP", defaults["BUTTON_CHIP"]),
		ButtonLine:  getEnvDefaultWithFallback("BUTTON_LINE", defaults["BUTTON_LINE"]),
		FanChip:     getEnvDefaultWithFallback("FAN_CHIP", defaults["FAN_CHIP"]),
		FanLine:     getEnvDefaultWithFallback("FAN_LINE", defaults["FAN_LINE"]),
		HardwarePWM: getEnvDefaultBoolWithFallback("HARDWARE_PWM", defaults["HARDWARE_PWM"] == "1"),
	}

	// Set I2C_BUS environment variable if not set and we have a detected value
	if os.Getenv("I2C_BUS") == "" && defaults["I2C_BUS"] != "" {
		os.Setenv("I2C_BUS", defaults["I2C_BUS"])
		log.Printf("Set I2C_BUS=%s based on device detection", defaults["I2C_BUS"])
	}

	return hw
}

func getEnvDefault(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

func getEnvDefaultBool(key string, defaultValue bool) bool {
	if value := os.Getenv(key); value != "" {
		if b, err := strconv.ParseBool(value); err == nil {
			return b
		}
		// Try parsing as int (0/1)
		if i, err := strconv.Atoi(value); err == nil {
			return i != 0
		}
	}
	return defaultValue
}

func getEnvDefaultWithFallback(key, fallbackValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallbackValue
}

func getEnvDefaultBoolWithFallback(key string, fallbackValue bool) bool {
	if value := os.Getenv(key); value != "" {
		if b, err := strconv.ParseBool(value); err == nil {
			return b
		}
		// Try parsing as int (0/1)
		if i, err := strconv.Atoi(value); err == nil {
			return i != 0
		}
	}
	return fallbackValue
}

func shouldAutoDetect() bool {
	// Check if auto-detection is explicitly disabled
	if disabled := os.Getenv("DISABLE_AUTO_DETECT"); disabled != "" {
		if b, err := strconv.ParseBool(disabled); err == nil && b {
			return false
		}
		if i, err := strconv.Atoi(disabled); err == nil && i != 0 {
			return false
		}
	}

	// Auto-detect by default, but not if we're in a container or CI environment
	if os.Getenv("CONTAINER") != "" || os.Getenv("CI") != "" {
		return false
	}

	return true
}

// IsRunning returns the current run state
func (c *Config) IsRunning() bool {
	return atomic.LoadInt32(c.RunState) != 0
}

// SetRunning sets the run state
func (c *Config) SetRunning(running bool) {
	val := int32(0)
	if running {
		val = 1
	}
	atomic.StoreInt32(c.RunState, val)
}

// ToggleRunning toggles the run state
func (c *Config) ToggleRunning() bool {
	for {
		old := atomic.LoadInt32(c.RunState)
		new := int32(0)
		if old == 0 {
			new = 1
		}
		if atomic.CompareAndSwapInt32(c.RunState, old, new) {
			return new != 0
		}
	}
}

// GetSliderIndex returns the current slider index
func (c *Config) GetSliderIndex() int32 {
	return atomic.LoadInt32(c.SliderIndex)
}

// IncrementSliderIndex increments and returns the new slider index
func (c *Config) IncrementSliderIndex() int32 {
	return atomic.AddInt32(c.SliderIndex, 1)
}

// SetDiskDevices sets the disk devices list
func (c *Config) SetDiskDevices(devices []string) {
	c.diskMutex.Lock()
	defer c.diskMutex.Unlock()
	c.DiskDevices = make([]string, len(devices))
	copy(c.DiskDevices, devices)
}

// GetDiskDevices returns a copy of the disk devices list
func (c *Config) GetDiskDevices() []string {
	c.diskMutex.RLock()
	defer c.diskMutex.RUnlock()
	devices := make([]string, len(c.DiskDevices))
	copy(devices, c.DiskDevices)
	return devices
}

// GetFanDutyCycle calculates the fan duty cycle based on temperature
func (c *Config) GetFanDutyCycle(temp float64) float64 {
	if !c.IsRunning() {
		return 0.999 // Off state
	}

	// Temperature thresholds to duty cycle mapping (from Python lv2dc)
	if temp >= c.Fan.Lv3 {
		return 0.0 // 100% power
	}
	if temp >= c.Fan.Lv2 {
		return 0.25 // 75% power
	}
	if temp >= c.Fan.Lv1 {
		return 0.5 // 50% power
	}
	if temp >= c.Fan.Lv0 {
		return 0.75 // 25% power
	}
	return 0.999 // Off
}

// GetKeyAction returns the action for a given key event
func (c *Config) GetKeyAction(key string) string {
	switch key {
	case "click":
		return c.Key.Click
	case "twice":
		return c.Key.Twice
	case "press":
		return c.Key.Press
	default:
		return "none"
	}
}

// String returns a string representation of the configuration
func (c *Config) String() string {
	return fmt.Sprintf("Config{Fan: %+v, Key: %+v, Time: %+v, Slider: %+v, OLED: %+v, Running: %v}",
		c.Fan, c.Key, c.Time, c.Slider, c.OLED, c.IsRunning())
}
