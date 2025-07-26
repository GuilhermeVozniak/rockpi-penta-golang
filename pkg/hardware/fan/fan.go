package fan

import (
	"fmt"
	"log"
	"os"
	"strconv"
	"sync"
	"time"

	"periph.io/x/conn/v3/gpio"
	"periph.io/x/conn/v3/gpio/gpioreg"
	"periph.io/x/host/v3"

	"github.com/GuilhermeVozniak/rockpi-penta-golang/pkg/config"
	"github.com/GuilhermeVozniak/rockpi-penta-golang/pkg/sysinfo"
)

type Controller struct {
	pwm       PWMInterface
	lastDuty  float64
	lastTemp  float64
	tempCache time.Time
	running   bool
	stopCh    chan struct{}
	mutex     sync.RWMutex
}

type PWMInterface interface {
	SetDutyCycle(duty float64) error
	Close() error
}

// HardwarePWM represents hardware PWM control
type HardwarePWM struct {
	chipPath string
	period   time.Duration
}

// SoftwarePWM represents software PWM control using GPIO
type SoftwarePWM struct {
	pin     gpio.PinOut
	period  time.Duration
	duty    float64
	stopCh  chan struct{}
	running bool
	mutex   sync.RWMutex
}

var (
	instance *Controller
	once     sync.Once
)

// GetInstance returns the singleton fan controller
func GetInstance() *Controller {
	once.Do(func() {
		instance = &Controller{
			lastDuty: -1,
			stopCh:   make(chan struct{}),
		}
	})
	return instance
}

// Initialize sets up the fan control based on hardware configuration
func (c *Controller) Initialize() error {
	c.mutex.Lock()
	defer c.mutex.Unlock()

	if c.pwm != nil {
		c.pwm.Close()
	}

	hwConfig := config.HWConfig
	if hwConfig == nil {
		return fmt.Errorf("hardware configuration not loaded")
	}

	var err error
	if hwConfig.HardwarePWM {
		c.pwm, err = c.initHardwarePWM(hwConfig.FanChip)
	} else {
		c.pwm, err = c.initSoftwarePWM(hwConfig.FanChip, hwConfig.FanLine)
	}

	return err
}

func (c *Controller) initHardwarePWM(chipStr string) (*HardwarePWM, error) {
	chipPath := fmt.Sprintf("/sys/class/pwm/pwmchip%s/pwm0/", chipStr)

	// Try to export PWM
	exportPath := fmt.Sprintf("/sys/class/pwm/pwmchip%s/export", chipStr)
	if err := os.WriteFile(exportPath, []byte("0"), 0644); err != nil {
		// Ignore error if already exported
		log.Printf("Warning: PWM export error (may already be exported): %v", err)
	}

	pwm := &HardwarePWM{
		chipPath: chipPath,
		period:   40 * time.Microsecond, // 25kHz frequency
	}

	// Set period
	periodPath := chipPath + "period"
	periodNs := pwm.period.Nanoseconds()
	if err := os.WriteFile(periodPath, []byte(strconv.FormatInt(periodNs, 10)), 0644); err != nil {
		return nil, fmt.Errorf("failed to set PWM period: %v", err)
	}

	// Enable PWM
	enablePath := chipPath + "enable"
	if err := os.WriteFile(enablePath, []byte("1"), 0644); err != nil {
		return nil, fmt.Errorf("failed to enable PWM: %v", err)
	}

	log.Printf("Hardware PWM initialized on chip %s", chipStr)
	return pwm, nil
}

func (c *Controller) initSoftwarePWM(chipStr, lineStr string) (*SoftwarePWM, error) {
	// Initialize periph.io
	if _, err := host.Init(); err != nil {
		return nil, fmt.Errorf("failed to initialize periph.io: %v", err)
	}

	// Convert chip and line to GPIO pin name
	pinName := fmt.Sprintf("GPIO%s_%s", chipStr, lineStr)
	pin := gpioreg.ByName(pinName)
	if pin == nil {
		// Try alternative naming
		pinName = fmt.Sprintf("GPIO%s", lineStr)
		pin = gpioreg.ByName(pinName)
		if pin == nil {
			return nil, fmt.Errorf("failed to find GPIO pin %s or %s", fmt.Sprintf("GPIO%s_%s", chipStr, lineStr), pinName)
		}
	}

	// Configure as output
	if err := pin.Out(gpio.Low); err != nil {
		return nil, fmt.Errorf("failed to configure GPIO pin as output: %v", err)
	}

	swPWM := &SoftwarePWM{
		pin:    pin,
		period: 25 * time.Millisecond, // 40Hz frequency for software PWM
		stopCh: make(chan struct{}),
	}

	// Start PWM goroutine
	go swPWM.runPWM()

	log.Printf("Software PWM initialized on GPIO%s_%s", chipStr, lineStr)
	return swPWM, nil
}

// SetDutyCycle for HardwarePWM
func (h *HardwarePWM) SetDutyCycle(duty float64) error {
	dutyPath := h.chipPath + "duty_cycle"
	dutyNs := int64(float64(h.period.Nanoseconds()) * duty)
	return os.WriteFile(dutyPath, []byte(strconv.FormatInt(dutyNs, 10)), 0644)
}

// Close for HardwarePWM
func (h *HardwarePWM) Close() error {
	// Disable PWM
	enablePath := h.chipPath + "enable"
	return os.WriteFile(enablePath, []byte("0"), 0644)
}

// SetDutyCycle for SoftwarePWM
func (s *SoftwarePWM) SetDutyCycle(duty float64) error {
	s.mutex.Lock()
	defer s.mutex.Unlock()
	s.duty = duty
	return nil
}

// Close for SoftwarePWM
func (s *SoftwarePWM) Close() error {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	if s.running {
		close(s.stopCh)
		s.running = false
	}
	return nil
}

// runPWM runs the software PWM loop
func (s *SoftwarePWM) runPWM() {
	s.mutex.Lock()
	s.running = true
	s.mutex.Unlock()

	ticker := time.NewTicker(s.period)
	defer ticker.Stop()

	for {
		select {
		case <-s.stopCh:
			return
		case <-ticker.C:
			s.mutex.RLock()
			duty := s.duty
			s.mutex.RUnlock()

			if duty <= 0.001 {
				// Fully off
				s.pin.Out(gpio.Low)
				continue
			}
			if duty >= 0.999 {
				// Fully on
				s.pin.Out(gpio.High)
				continue
			}

			// PWM cycle
			onTime := time.Duration(float64(s.period.Nanoseconds()) * duty)
			offTime := s.period - onTime

			s.pin.Out(gpio.High)
			time.Sleep(onTime)
			s.pin.Out(gpio.Low)
			time.Sleep(offTime)
		}
	}
}

// Start begins the fan control loop
func (c *Controller) Start() error {
	c.mutex.Lock()
	defer c.mutex.Unlock()

	if c.running {
		return fmt.Errorf("fan controller already running")
	}

	if c.pwm == nil {
		if err := c.Initialize(); err != nil {
			return fmt.Errorf("failed to initialize fan control: %v", err)
		}
	}

	c.running = true
	c.stopCh = make(chan struct{})

	go c.controlLoop()
	log.Println("Fan controller started")
	return nil
}

// Stop stops the fan control loop
func (c *Controller) Stop() {
	c.mutex.Lock()
	defer c.mutex.Unlock()

	if !c.running {
		return
	}

	c.running = false
	close(c.stopCh)

	if c.pwm != nil {
		c.pwm.Close()
	}

	log.Println("Fan controller stopped")
}

// controlLoop is the main fan control loop
func (c *Controller) controlLoop() {
	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()

	sysInfo := sysinfo.GetInstance()

	for {
		select {
		case <-c.stopCh:
			return
		case <-ticker.C:
			c.updateFanSpeed(sysInfo)
		}
	}
}

// updateFanSpeed updates the fan speed based on temperature
func (c *Controller) updateFanSpeed(sysInfo *sysinfo.SystemInfo) {
	now := time.Now()

	// Update temperature cache every 60 seconds
	var temp float64
	if now.Sub(c.tempCache) > 60*time.Second {
		if err := sysInfo.Update(); err != nil {
			log.Printf("Failed to update system info: %v", err)
			return
		}
		temp = sysInfo.CPUTemp
		c.tempCache = now
		c.lastTemp = temp
	} else {
		temp = c.lastTemp
	}

	// Calculate duty cycle based on temperature
	duty := config.GlobalConfig.GetFanDutyCycle(temp)

	// Only update if duty cycle changed
	if duty != c.lastDuty {
		if err := c.pwm.SetDutyCycle(duty); err != nil {
			log.Printf("Failed to set fan duty cycle: %v", err)
		} else {
			log.Printf("Fan duty cycle set to %.1f%% (temp: %.1f°C)", (1.0-duty)*100, temp)
			c.lastDuty = duty
		}
	}
}

// GetTemperature returns the last cached CPU temperature
func (c *Controller) GetTemperature() float64 {
	c.mutex.RLock()
	defer c.mutex.RUnlock()
	return c.lastTemp
}

// IsRunning returns whether the fan controller is running
func (c *Controller) IsRunning() bool {
	c.mutex.RLock()
	defer c.mutex.RUnlock()
	return c.running
}
