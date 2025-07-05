package fan

import (
	"context"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strconv"
	"sync"
	"time"

	"github.com/radxa/rocki-penta-golang/pkg/config"
	"github.com/warthog618/gpiod"
)

// Controller manages the fan control
type Controller struct {
	cfg        *config.Config
	pwm        *PWMController
	gpio       *GPIOController
	enabled    bool
	mu         sync.RWMutex
	tempCache  temperatureCache
	dutyCache  float64
	lastDuty   float64
}

type temperatureCache struct {
	temp float64
	time time.Time
}

// PWMController handles hardware PWM control
type PWMController struct {
	chipPath   string
	pwmPath    string
	periodNs   int64
	enabled    bool
}

// GPIOController handles GPIO-based PWM simulation
type GPIOController struct {
	line     *gpiod.Line
	chip     *gpiod.Chip
	period   time.Duration
	dutyHigh time.Duration
	dutyLow  time.Duration
	running  bool
	stop     chan struct{}
}

// New creates a new fan controller
func New(cfg *config.Config) (*Controller, error) {
	ctrl := &Controller{
		cfg:     cfg,
		enabled: true,
	}

	// Initialize PWM or GPIO controller based on hardware configuration
	if cfg.Hardware.HardwarePWM == "1" {
		pwmCtrl, err := NewPWMController(cfg.Hardware.PWMChip)
		if err != nil {
			return nil, fmt.Errorf("failed to initialize PWM controller: %w", err)
		}
		ctrl.pwm = pwmCtrl
	} else {
		gpioCtrl, err := NewGPIOController(cfg.Hardware.FanChip, cfg.Hardware.FanLine)
		if err != nil {
			return nil, fmt.Errorf("failed to initialize GPIO controller: %w", err)
		}
		ctrl.gpio = gpioCtrl
	}

	return ctrl, nil
}

// NewPWMController creates a new PWM controller
func NewPWMController(chip string) (*PWMController, error) {
	var chipPath string
	
	// Handle both numeric and string chip names
	if _, err := strconv.Atoi(chip); err == nil {
		chipPath = fmt.Sprintf("/sys/class/pwm/pwmchip%s", chip)
	} else {
		chipPath = fmt.Sprintf("/sys/class/pwm/%s", chip)
	}

	pwmPath := filepath.Join(chipPath, "pwm0")
	exportPath := filepath.Join(chipPath, "export")

	// Export PWM channel if not already exported
	if _, err := os.Stat(pwmPath); os.IsNotExist(err) {
		if err := os.WriteFile(exportPath, []byte("0"), 0644); err != nil {
			log.Printf("Warning: Could not export PWM channel: %v", err)
		}
	}

	ctrl := &PWMController{
		chipPath: chipPath,
		pwmPath:  pwmPath,
		periodNs: 40000, // 40 microseconds
	}

	// Set period
	if err := ctrl.setPeriod(ctrl.periodNs); err != nil {
		return nil, fmt.Errorf("failed to set PWM period: %w", err)
	}

	// Enable PWM
	if err := ctrl.setEnabled(true); err != nil {
		return nil, fmt.Errorf("failed to enable PWM: %w", err)
	}

	return ctrl, nil
}

// NewGPIOController creates a new GPIO controller
func NewGPIOController(chipName, lineName string) (*GPIOController, error) {
	chipNum, err := strconv.Atoi(chipName)
	if err != nil {
		return nil, fmt.Errorf("invalid chip name: %s", chipName)
	}

	lineNum, err := strconv.Atoi(lineName)
	if err != nil {
		return nil, fmt.Errorf("invalid line name: %s", lineName)
	}

	chip, err := gpiod.NewChip(fmt.Sprintf("gpiochip%d", chipNum))
	if err != nil {
		return nil, fmt.Errorf("failed to open GPIO chip: %w", err)
	}

	line, err := chip.RequestLine(lineNum, gpiod.AsOutput(0))
	if err != nil {
		chip.Close()
		return nil, fmt.Errorf("failed to request GPIO line: %w", err)
	}

	ctrl := &GPIOController{
		line:   line,
		chip:   chip,
		period: 25 * time.Millisecond, // 25ms period
		stop:   make(chan struct{}),
	}

	return ctrl, nil
}

// setPeriod sets the PWM period
func (p *PWMController) setPeriod(ns int64) error {
	periodPath := filepath.Join(p.pwmPath, "period")
	return os.WriteFile(periodPath, []byte(fmt.Sprintf("%d", ns)), 0644)
}

// setEnabled enables/disables the PWM
func (p *PWMController) setEnabled(enabled bool) error {
	enablePath := filepath.Join(p.pwmPath, "enable")
	var value string
	if enabled {
		value = "1"
	} else {
		value = "0"
	}
	p.enabled = enabled
	return os.WriteFile(enablePath, []byte(value), 0644)
}

// setDutyCycle sets the PWM duty cycle (0.0 to 1.0)
func (p *PWMController) setDutyCycle(duty float64) error {
	if duty < 0 {
		duty = 0
	} else if duty > 1 {
		duty = 1
	}
	
	dutyNs := int64(float64(p.periodNs) * duty)
	dutyCyclePath := filepath.Join(p.pwmPath, "duty_cycle")
	return os.WriteFile(dutyCyclePath, []byte(fmt.Sprintf("%d", dutyNs)), 0644)
}

// setDutyCycle sets the GPIO PWM duty cycle (0.0 to 1.0)
func (g *GPIOController) setDutyCycle(duty float64) error {
	if duty < 0 {
		duty = 0
	} else if duty > 1 {
		duty = 1
	}

	g.dutyHigh = time.Duration(float64(g.period) * duty)
	g.dutyLow = g.period - g.dutyHigh

	// Start PWM if not already running
	if !g.running {
		g.running = true
		go g.runPWM()
	}

	return nil
}

// runPWM runs the GPIO PWM simulation
func (g *GPIOController) runPWM() {
	for g.running {
		select {
		case <-g.stop:
			g.running = false
			return
		default:
			if g.dutyHigh > 0 {
				g.line.SetValue(1)
				time.Sleep(g.dutyHigh)
			}
			if g.dutyLow > 0 {
				g.line.SetValue(0)
				time.Sleep(g.dutyLow)
			}
		}
	}
}

// Close closes the GPIO controller
func (g *GPIOController) Close() error {
	if g.running {
		g.running = false
		close(g.stop)
	}
	if g.line != nil {
		g.line.Close()
	}
	if g.chip != nil {
		return g.chip.Close()
	}
	return nil
}

// Close closes the PWM controller
func (p *PWMController) Close() error {
	return p.setEnabled(false)
}

// readTemperature reads the CPU temperature
func (c *Controller) readTemperature() (float64, error) {
	data, err := os.ReadFile("/sys/class/thermal/thermal_zone0/temp")
	if err != nil {
		return 0, fmt.Errorf("failed to read temperature: %w", err)
	}

	tempMilli, err := strconv.ParseInt(string(data[:len(data)-1]), 10, 64)
	if err != nil {
		return 0, fmt.Errorf("failed to parse temperature: %w", err)
	}

	return float64(tempMilli) / 1000.0, nil
}

// getTemperature gets the current temperature with caching
func (c *Controller) getTemperature() (float64, error) {
	c.mu.RLock()
	if time.Since(c.tempCache.time) < 60*time.Second {
		temp := c.tempCache.temp
		c.mu.RUnlock()
		return temp, nil
	}
	c.mu.RUnlock()

	temp, err := c.readTemperature()
	if err != nil {
		return 0, err
	}

	c.mu.Lock()
	c.tempCache.temp = temp
	c.tempCache.time = time.Now()
	c.mu.Unlock()

	return temp, nil
}

// setDutyCycle sets the fan duty cycle
func (c *Controller) setDutyCycle(duty float64) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if duty == c.lastDuty {
		return nil // No change needed
	}

	c.lastDuty = duty

	if c.pwm != nil {
		return c.pwm.setDutyCycle(duty)
	} else if c.gpio != nil {
		return c.gpio.setDutyCycle(duty)
	}

	return fmt.Errorf("no PWM or GPIO controller available")
}

// Run runs the fan control loop
func (c *Controller) Run(ctx context.Context) {
	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			c.mu.RLock()
			enabled := c.enabled
			c.mu.RUnlock()

			if !enabled {
				// Fan is disabled, set to off
				if err := c.setDutyCycle(0.999); err != nil {
					log.Printf("Error setting fan duty cycle: %v", err)
				}
				continue
			}

			temp, err := c.getTemperature()
			if err != nil {
				log.Printf("Error reading temperature: %v", err)
				continue
			}

			duty := c.cfg.GetFanDutyCycle(temp)
			if err := c.setDutyCycle(duty); err != nil {
				log.Printf("Error setting fan duty cycle: %v", err)
			}
		}
	}
}

// Toggle toggles the fan on/off
func (c *Controller) Toggle() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.enabled = !c.enabled
	log.Printf("Fan %s", map[bool]string{true: "enabled", false: "disabled"}[c.enabled])
}

// Close closes the fan controller
func (c *Controller) Close() error {
	if c.pwm != nil {
		return c.pwm.Close()
	}
	if c.gpio != nil {
		return c.gpio.Close()
	}
	return nil
} 