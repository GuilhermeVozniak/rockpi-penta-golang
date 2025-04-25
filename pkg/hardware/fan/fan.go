package fan

import (
	"fmt"
	"log"
	"os"
	"strconv"
	"strings"
	"time"

	"rockpi-penta-golang/pkg/config"
	"rockpi-penta-golang/pkg/sysinfo"

	"periph.io/x/conn/v3/driver/driverreg"
	"periph.io/x/conn/v3/gpio"
	"periph.io/x/conn/v3/gpio/gpioreg"
)

const (
	fanCheckInterval = 1 * time.Second
	fanTempCacheTime = 60 * time.Second
	fanSysfsPwmPath  = "/sys/class/pwm"
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

	dutyNs := int64(float64(h.periodNs) * duty)
	dutyPath := fmt.Sprintf("%s/duty_cycle", h.pinPath)

	if err := os.WriteFile(dutyPath, []byte(strconv.FormatInt(dutyNs, 10)), 0644); err != nil {
		return fmt.Errorf("failed to set PWM duty cycle on %s: %w", h.pinPath, err)
	}
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

// --- Software PWM (GPIO) ---

type softwarePWMFan struct {
	pin      gpio.PinIO
	stopChan chan struct{}
	dutyChan chan float64
	period   time.Duration
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

	s := &softwarePWMFan{
		pin:      p,
		stopChan: make(chan struct{}),
		dutyChan: make(chan float64, 1), // Buffered channel for duty cycle updates
		period:   period,
	}

	go s.runPWM() // Start the PWM goroutine
	log.Printf("Initialized Software PWM fan controller on pin %s", p.Name())
	return s, nil
}

// runPWM simulates PWM signal in a goroutine.
func (s *softwarePWMFan) runPWM() {
	currentDuty := 0.0
	ticker := time.NewTicker(s.period)
	defer ticker.Stop()

	for {
		select {
		case duty := <-s.dutyChan:
			currentDuty = duty
			if currentDuty < 0.01 {
				currentDuty = 0
			}
			if currentDuty > 0.99 {
				currentDuty = 1.0
			}
			log.Printf("Software PWM duty set to %.2f", currentDuty)
		case <-ticker.C:
			if currentDuty <= 0 { // Ensure pin is low if duty is 0
				s.pin.Out(gpio.Low)
				continue
			}
			if currentDuty >= 1.0 { // Ensure pin is high if duty is 1
				s.pin.Out(gpio.High)
				continue
			}
			// Perform PWM cycle
			onDuration := time.Duration(float64(s.period) * currentDuty)
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

func (s *softwarePWMFan) SetDutyCycle(duty float64) error {
	s.dutyChan <- duty
	return nil
}

func (s *softwarePWMFan) Cleanup() error {
	log.Printf("Cleaning up Software PWM fan controller on pin %s", s.pin.Name())
	close(s.stopChan)
	// Small delay to allow runPWM to finish
	time.Sleep(s.period + 50*time.Millisecond)
	return s.pin.Out(gpio.Low) // Ensure pin is left low
}

// --- Fan Control Logic ---

var (
	lastTempRead time.Time
	lastTemp     float64
	lastDuty     float64 = -1 // Start with invalid duty to force initial set
)

// getFanDutyCycle calculates the target duty cycle based on temperature and config.
// Duty cycle values match the Python lv2dc mapping reversed:
// lv0 (e.g. 35C): 0.75
// lv1 (e.g. 40C): 0.50
// lv2 (e.g. 45C): 0.25
// lv3 (e.g. 50C): 0.00 (Full speed)
// Below lv0: 0.999 (interpreted as ~off by pwm control)
func getFanDutyCycle(conf *config.Config) float64 {
	// Use cached temperature if recent
	if time.Since(lastTempRead) < fanTempCacheTime {
		// Use cached temp
	} else {
		// Read current temperature (ignore errors for now, keep last temp)
		// Assume GetCPUTemp returns temp in C
		tempStr, err := sysinfo.GetCPUTemp(false) // Get temp in Celsius
		if err == nil {
			// Parse C temp: "CPU Temp: 45.1°C"
			parts := strings.Fields(tempStr)
			if len(parts) >= 3 {
				tempValStr := strings.TrimSuffix(parts[2], "°C")
				temp, parseErr := strconv.ParseFloat(tempValStr, 64)
				if parseErr == nil {
					lastTemp = temp
					lastTempRead = time.Now()
					log.Printf("Read CPU Temp: %.1f°C", lastTemp)
				} else {
					log.Printf("Warning: Could not parse CPU temp value '%s': %v", tempValStr, parseErr)
				}
			} else {
				log.Printf("Warning: Could not parse CPU temp string '%s'", tempStr)
			}
		} else {
			log.Printf("Warning: Failed to read CPU temp: %v", err)
		}
	}

	// Check if fan is manually disabled
	if !conf.Run.Load() {
		log.Println("Fan manually disabled, setting duty cycle to OFF (0.999)")
		return 0.999 // Use 0.999 for off, as in Python version
	}

	t := lastTemp
	var duty float64

	switch {
	case t >= conf.Fan.Lv3:
		duty = 0.0 // Full speed
	case t >= conf.Fan.Lv2:
		duty = 0.25
	case t >= conf.Fan.Lv1:
		duty = 0.50
	case t >= conf.Fan.Lv0:
		duty = 0.75
	default:
		duty = 0.999 // Off
	}

	log.Printf("Temp %.1f°C -> Duty Cycle %.3f", t, duty)
	return duty
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
			// Only update fan if duty cycle has changed
			if targetDuty != lastDuty {
				if err := fan.SetDutyCycle(targetDuty); err != nil {
					log.Printf("Error setting fan duty cycle to %.3f: %v", targetDuty, err)
				} else {
					log.Printf("Fan duty cycle set to %.3f", targetDuty)
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
