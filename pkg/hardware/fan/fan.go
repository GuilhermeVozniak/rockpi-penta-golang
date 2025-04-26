package fan

import (
	"fmt"
	"log"
	"os"
	"strconv"
	"strings"
	"sync/atomic"
	"time"

	"rockpi-penta-golang/pkg/config"
	"rockpi-penta-golang/pkg/sysinfo"

	"periph.io/x/conn/v3/driver/driverreg"
	"periph.io/x/conn/v3/gpio"
	"periph.io/x/conn/v3/gpio/gpioreg"
)

const (
	fanCheckInterval      = 1 * time.Second
	fanTempCacheTime      = 60 * time.Second
	fanSysfsPwmPath       = "/sys/class/pwm"
	softwarePwmResolution = 100 // Higher values provide smoother PWM but use more CPU
)

// FanController defines the interface for controlling the fan.
type FanController interface {
	SetDutyCycle(duty float64) error // duty is 0.0 (off) to 1.0 (full)
	Cleanup() error
}

// --- Hardware PWM ---

type hardwarePWMFan struct {
	chipPath   string
	exportPath string
	pinPath    string
	periodNs   int64
	lastDuty   float64 // Cache last value to avoid unnecessary writes
}

// newHardwarePWMFan initializes hardware PWM via sysfs.
func newHardwarePWMFan(chipNumStr string, periodNs int64) (FanController, error) {
	chipPath := fmt.Sprintf("%s/pwmchip%s", fanSysfsPwmPath, chipNumStr)
	if _, err := os.Stat(chipPath); os.IsNotExist(err) {
		return nil, fmt.Errorf("pwmchip %s not found at %s", chipNumStr, chipPath)
	}

	h := &hardwarePWMFan{
		chipPath:   chipPath,
		exportPath: fmt.Sprintf("%s/export", chipPath),
		pinPath:    fmt.Sprintf("%s/pwm0", chipPath), // Assuming pwm0
		periodNs:   periodNs,
		lastDuty:   -1, // Invalid initial value to ensure first update
	}

	// Export pwm0 if not already exported
	if _, err := os.Stat(h.pinPath); os.IsNotExist(err) {
		if err := os.WriteFile(h.exportPath, []byte("0"), 0644); err != nil {
			// Ignore export errors if it already exists (might happen due to race conditions)
			if !strings.Contains(err.Error(), "device or resource busy") {
				return nil, fmt.Errorf("failed to export pwm0 on chip %s: %w", chipNumStr, err)
			}
			log.Println("PWM0 already exported or busy, continuing...")
		} else {
			log.Println("Exported PWM0 successfully.")
		}
		// Give sysfs a moment
		time.Sleep(100 * time.Millisecond)
	}

	// Set period
	if err := os.WriteFile(fmt.Sprintf("%s/period", h.pinPath), []byte(strconv.FormatInt(periodNs, 10)), 0644); err != nil {
		return nil, fmt.Errorf("failed to set PWM period on %s: %w", h.pinPath, err)
	}

	// Enable PWM
	if err := os.WriteFile(fmt.Sprintf("%s/enable", h.pinPath), []byte("1"), 0644); err != nil {
		return nil, fmt.Errorf("failed to enable PWM on %s: %w", h.pinPath, err)
	}

	log.Printf("Initialized Hardware PWM fan controller on %s", h.pinPath)
	return h, nil
}

func (h *hardwarePWMFan) SetDutyCycle(duty float64) error {
	if duty < 0 {
		duty = 0
	}
	if duty > 1 {
		duty = 1
	}

	// Avoid unnecessary writes if duty cycle hasn't changed significantly
	if h.lastDuty >= 0 && abs(duty-h.lastDuty) < 0.01 {
		return nil
	}

	dutyNs := int64(float64(h.periodNs) * duty)
	dutyPath := fmt.Sprintf("%s/duty_cycle", h.pinPath)

	if err := os.WriteFile(dutyPath, []byte(strconv.FormatInt(dutyNs, 10)), 0644); err != nil {
		return fmt.Errorf("failed to set PWM duty cycle on %s: %w", h.pinPath, err)
	}

	h.lastDuty = duty
	return nil
}

func (h *hardwarePWMFan) Cleanup() error {
	log.Printf("Cleaning up Hardware PWM fan controller on %s", h.pinPath)
	// Disable PWM
	if err := os.WriteFile(fmt.Sprintf("%s/enable", h.pinPath), []byte("0"), 0644); err != nil {
		log.Printf("Warning: failed to disable PWM on %s: %v", h.pinPath, err)
		// Continue cleanup even if disable fails
	}
	// Unexport pwm0 - best effort
	unexportPath := fmt.Sprintf("%s/unexport", h.chipPath)
	if err := os.WriteFile(unexportPath, []byte("0"), 0644); err != nil {
		log.Printf("Warning: failed to unexport pwm0 on chip %s: %v", h.chipPath, err)
	}
	return nil
}

// --- Optimized Software PWM (GPIO) ---

type softwarePWMFan struct {
	pin          gpio.PinIO
	stopChan     chan struct{}
	dutyChan     chan float64
	period       time.Duration
	currentDuty  atomic.Uint64 // Use atomic for safe concurrent access
	sleepEnabled bool          // Flag to determine if we should use more efficient sleep pattern
}

// newSoftwarePWMFan initializes GPIO pin for software PWM.
func newSoftwarePWMFan(chipName, lineNumStr string, period time.Duration) (FanController, error) {
	pinName := fmt.Sprintf("%s/%s", chipName, lineNumStr)
	p := gpioreg.ByName(pinName)
	if p == nil {
		p = gpioreg.ByName(lineNumStr)
		if p == nil {
			return nil, fmt.Errorf("failed to find GPIO pin for fan: %s or %s", pinName, lineNumStr)
		}
	}

	if err := p.Out(gpio.Low); err != nil { // Start low (off)
		return nil, fmt.Errorf("failed to set fan pin %s to output: %w", p.Name(), err)
	}

	// Check if OS has efficient sleep support for nanosleep
	// This is a simple heuristic - in practice, this should be true on Linux
	sleepEnabled := true

	s := &softwarePWMFan{
		pin:          p,
		stopChan:     make(chan struct{}),
		dutyChan:     make(chan float64, 1), // Buffered channel for duty cycle updates
		period:       period,
		sleepEnabled: sleepEnabled,
	}

	// Store initial duty cycle of 0
	s.currentDuty.Store(0)

	go s.runPWM() // Start the PWM goroutine
	log.Printf("Initialized Software PWM fan controller on pin %s", p.Name())
	return s, nil
}

// runPWM simulates PWM signal in a goroutine with optimized CPU usage.
func (s *softwarePWMFan) runPWM() {
	// For extremely low duty cycles (fan off) or extremely high (fan full)
	// we'll optimize by not toggling and just setting the pin state
	pinState := gpio.Low
	var ticker *time.Ticker

	if s.sleepEnabled {
		// Use more efficient approach for systems with good sleep support
		ticker = time.NewTicker(s.period / softwarePwmResolution)
		defer ticker.Stop()

		var counter int

		for {
			select {
			case duty := <-s.dutyChan:
				// Clamp duty cycle values
				if duty < 0.01 {
					duty = 0
				} else if duty > 0.99 {
					duty = 1.0
				}
				// Store new duty cycle atomically
				s.currentDuty.Store(uint64(duty * float64(softwarePwmResolution)))
				log.Printf("Software PWM duty set to %.2f", duty)

				// Special cases optimization
				if duty <= 0 || duty >= 1.0 {
					if duty <= 0 {
						pinState = gpio.Low
					} else {
						pinState = gpio.High
					}
					s.pin.Out(pinState) // Set pin state directly for special cases
				}

				// Reset counter on duty change
				counter = 0

			case <-ticker.C:
				duty := float64(s.currentDuty.Load()) / float64(softwarePwmResolution)

				// Skip PWM for special cases
				if duty <= 0 || duty >= 1.0 {
					continue
				}

				// Use counter-based PWM approach
				counter = (counter + 1) % softwarePwmResolution
				threshold := int(duty * float64(softwarePwmResolution))

				if counter < threshold {
					if pinState != gpio.High {
						s.pin.Out(gpio.High)
						pinState = gpio.High
					}
				} else {
					if pinState != gpio.Low {
						s.pin.Out(gpio.Low)
						pinState = gpio.Low
					}
				}

			case <-s.stopChan:
				log.Println("Stopping software PWM...")
				s.pin.Out(gpio.Low) // Ensure fan is off on stop
				return
			}
		}
	} else {
		// Fallback to original approach for systems with poor sleep behavior
		ticker = time.NewTicker(s.period)
		defer ticker.Stop()

		for {
			select {
			case duty := <-s.dutyChan:
				if duty < 0.01 {
					duty = 0
				}
				if duty > 0.99 {
					duty = 1.0
				}
				s.currentDuty.Store(uint64(duty * 1000000)) // Store with high precision
				log.Printf("Software PWM duty set to %.2f", duty)

			case <-ticker.C:
				duty := float64(s.currentDuty.Load()) / 1000000

				if duty <= 0 {
					s.pin.Out(gpio.Low)
					continue
				}
				if duty >= 1.0 {
					s.pin.Out(gpio.High)
					continue
				}

				// Perform PWM cycle
				onDuration := time.Duration(float64(s.period) * duty)
				s.pin.Out(gpio.High)
				time.Sleep(onDuration)
				s.pin.Out(gpio.Low)

			case <-s.stopChan:
				log.Println("Stopping software PWM...")
				s.pin.Out(gpio.Low) // Ensure fan is off on stop
				return
			}
		}
	}
}

func (s *softwarePWMFan) SetDutyCycle(duty float64) error {
	// Compare with current duty to avoid unnecessary updates
	currentDuty := float64(s.currentDuty.Load()) / 1000000
	if abs(duty-currentDuty) < 0.01 {
		return nil // No significant change, skip update
	}

	select {
	case s.dutyChan <- duty:
		// Successfully sent duty update
	default:
		// Channel buffer is full, replace the value
		select {
		case <-s.dutyChan: // Remove old value
		default:
			// Channel is now empty
		}
		s.dutyChan <- duty
	}
	return nil
}

func (s *softwarePWMFan) Cleanup() error {
	log.Printf("Cleaning up Software PWM fan controller on pin %s", s.pin.Name())
	close(s.stopChan)
	// Small delay to allow runPWM to finish
	time.Sleep(s.period + 50*time.Millisecond)
	return s.pin.Out(gpio.Low) // Ensure pin is left low
}

// Helper function for floating point absolute value
func abs(x float64) float64 {
	if x < 0 {
		return -x
	}
	return x
}

// --- Fan Control Logic ---

// TempCache holds temperature reading state to reduce system calls
type TempCache struct {
	Value     float64
	Timestamp time.Time
}

var (
	tempCacheData TempCache
	lastDuty      float64 = -1 // Start with invalid duty to force initial set
)

// Initialize the temperature cache
func init() {
	tempCacheData = TempCache{
		Value:     0,
		Timestamp: time.Time{},
	}
}

// getFanDutyCycle calculates the target duty cycle based on temperature and config.
// Duty cycle values match the Python lv2dc mapping reversed:
// lv0 (e.g. 35C): 0.75
// lv1 (e.g. 40C): 0.50
// lv2 (e.g. 45C): 0.25
// lv3 (e.g. 50C): 0.00 (Full speed)
// Below lv0: 0.999 (interpreted as ~off by pwm control)
func getFanDutyCycle(conf *config.Config) float64 {
	// Read hard drive temperatures using sysinfo package
	temp := getTemperatureForFanControl()

	// Check if fan is manually disabled
	if !conf.Run.Load() {
		return 0.999 // Use 0.999 for off, as in Python version
	}

	var duty float64
	switch {
	case temp >= conf.Fan.Lv3:
		duty = 0.0 // Full speed
	case temp >= conf.Fan.Lv2:
		duty = 0.25
	case temp >= conf.Fan.Lv1:
		duty = 0.50
	case temp >= conf.Fan.Lv0:
		duty = 0.75
	default:
		duty = 0.999 // Off
	}

	// Only log when duty changes or periodically for debug
	if abs(duty-lastDuty) >= 0.01 {
		log.Printf("Temperature %.1f°C -> Duty Cycle %.3f", temp, duty)
	}

	return duty
}

// getTemperatureForFanControl returns the temperature to use for fan control
// It tries to use the highest disk temperature, with CPU temperature as fallback
func getTemperatureForFanControl() float64 {
	// Use cached temperature if recent
	if time.Since(tempCacheData.Timestamp) < fanTempCacheTime {
		return tempCacheData.Value
	}

	// Try to get disk temperatures
	diskTemp, err := sysinfo.GetHighestDiskTemperature()
	if err == nil && diskTemp > 0 {
		log.Printf("Using highest disk temperature: %.1f°C for fan control", diskTemp)
		tempCacheData.Value = diskTemp
		tempCacheData.Timestamp = time.Now()
		return diskTemp
	}

	// Fallback to CPU temperature if disk temperature isn't available
	log.Printf("Could not get disk temperature (%v), falling back to CPU temperature", err)
	cpuTemp := readCPUTemperature()
	log.Printf("Using CPU temperature: %.1f°C for fan control", cpuTemp)
	return cpuTemp
}

// readCPUTemperature reads the CPU temperature
// This is used as a fallback when disk temperatures are not available
func readCPUTemperature() float64 {
	// Read current temperature
	tempStr, err := sysinfo.GetCPUTemp(false) // Get temp in Celsius
	if err != nil {
		log.Printf("Warning: Failed to read CPU temp: %v", err)
		if tempCacheData.Value > 0 {
			return tempCacheData.Value // Return last known value
		}
		return 40.0 // Reasonable default
	}

	// Parse C temp: "CPU Temp: 45.1°C"
	parts := strings.Fields(tempStr)
	if len(parts) < 3 {
		log.Printf("Warning: Could not parse CPU temp string '%s'", tempStr)
		if tempCacheData.Value > 0 {
			return tempCacheData.Value
		}
		return 40.0 // Reasonable default
	}

	tempValStr := strings.TrimSuffix(parts[2], "°C")
	temp, err := strconv.ParseFloat(tempValStr, 64)
	if err != nil {
		log.Printf("Warning: Could not parse CPU temp value '%s': %v", tempValStr, err)
		if tempCacheData.Value > 0 {
			return tempCacheData.Value
		}
		return 40.0 // Reasonable default
	}

	// Update cache
	tempCacheData.Value = temp
	tempCacheData.Timestamp = time.Now()
	return temp
}

// controlFanLoop runs the main fan control logic.
func controlFanLoop(conf *config.Config, fan FanController, stopChan chan struct{}) {
	log.Println("Starting fan control loop...")
	ticker := time.NewTicker(fanCheckInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			targetDuty := getFanDutyCycle(conf)
			// Only update fan if duty cycle has changed significantly
			if abs(targetDuty-lastDuty) >= 0.01 {
				if err := fan.SetDutyCycle(targetDuty); err != nil {
					log.Printf("Error setting fan duty cycle to %.3f: %v", targetDuty, err)
				} else {
					if targetDuty <= 0.01 {
						log.Printf("Fan set to full speed (duty cycle %.3f)", targetDuty)
					} else if targetDuty >= 0.99 {
						log.Printf("Fan turned off (duty cycle %.3f)", targetDuty)
					} else {
						log.Printf("Fan duty cycle set to %.3f", targetDuty)
					}
					lastDuty = targetDuty
				}
			}
		case <-stopChan:
			log.Println("Stopping fan control loop...")
			if err := fan.Cleanup(); err != nil {
				log.Printf("Error during fan cleanup: %v", err)
			}
			return
		}
	}
}

// StartFanControl initializes and starts the fan controller.
func StartFanControl(conf *config.Config) (chan struct{}, error) {
	var fan FanController
	var err error

	useHardwarePWM := os.Getenv("HARDWARE_PWM") == "1"

	if useHardwarePWM {
		pwmChip := os.Getenv("PWMCHIP")
		if pwmChip == "" {
			return nil, fmt.Errorf("HARDWARE_PWM is 1 but PWMCHIP environment variable is not set")
		}
		// Python used period_us(40) -> 40,000 ns
		fan, err = newHardwarePWMFan(pwmChip, 40000)
		if err != nil {
			return nil, fmt.Errorf("failed to initialize hardware PWM fan: %w", err)
		}
	} else {
		fanChip := os.Getenv("FAN_CHIP")
		fanLine := os.Getenv("FAN_LINE")
		if fanChip == "" || fanLine == "" {
			return nil, fmt.Errorf("HARDWARE_PWM is not 1, but FAN_CHIP or FAN_LINE environment variables are not set")
		}
		// Python used Gpio(0.025) -> 25ms period
		fan, err = newSoftwarePWMFan(fanChip, fanLine, 25*time.Millisecond)
		if err != nil {
			// Initialize periph driver registry if needed
			if _, initErr := driverreg.Init(); initErr != nil {
				log.Printf("Warning: Failed to init periph driver registry: %v", initErr)
			}
			return nil, fmt.Errorf("failed to initialize software PWM fan: %w", err)
		}
	}

	stopChan := make(chan struct{})
	go controlFanLoop(conf, fan, stopChan)

	return stopChan, nil
}
