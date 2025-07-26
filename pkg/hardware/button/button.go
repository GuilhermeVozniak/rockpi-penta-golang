package button

import (
	"fmt"
	"log"
	"regexp"
	"strings"
	"sync"
	"time"

	"periph.io/x/conn/v3/gpio"
	"periph.io/x/conn/v3/gpio/gpioreg"
	"periph.io/x/host/v3"

	"github.com/GuilhermeVozniak/rockpi-penta-golang/pkg/config"
)

type Controller struct {
	pin         gpio.PinIn
	running     bool
	stopCh      chan struct{}
	eventCh     chan string
	mutex       sync.RWMutex
	patterns    map[string]*regexp.Regexp
	bufferSize  int
	waitPeriod  int
	pressPeriod int
}

var (
	instance *Controller
	once     sync.Once
)

// GetInstance returns the singleton button controller
func GetInstance() *Controller {
	once.Do(func() {
		instance = &Controller{
			eventCh: make(chan string, 10),
			stopCh:  make(chan struct{}),
		}
	})
	return instance
}

// Initialize sets up the button control
func (c *Controller) Initialize() error {
	c.mutex.Lock()
	defer c.mutex.Unlock()

	hwConfig := config.HWConfig
	if hwConfig == nil {
		return fmt.Errorf("hardware configuration not loaded")
	}

	// Initialize periph.io
	if _, err := host.Init(); err != nil {
		return fmt.Errorf("failed to initialize periph.io: %v", err)
	}

	// Convert chip and line to GPIO pin name
	pinName := fmt.Sprintf("GPIO%s_%s", hwConfig.ButtonChip, hwConfig.ButtonLine)
	pin := gpioreg.ByName(pinName)
	if pin == nil {
		// Try alternative naming
		pinName = fmt.Sprintf("GPIO%s", hwConfig.ButtonLine)
		pin = gpioreg.ByName(pinName)
		if pin == nil {
			return fmt.Errorf("failed to find GPIO pin %s or %s",
				fmt.Sprintf("GPIO%s_%s", hwConfig.ButtonChip, hwConfig.ButtonLine), pinName)
		}
	}

	// Configure as input with pull-up
	if err := pin.In(gpio.PullUp, gpio.BothEdges); err != nil {
		return fmt.Errorf("failed to configure GPIO pin as input: %v", err)
	}

	c.pin = pin

	// Setup timing and patterns based on config
	cfg := config.GlobalConfig
	c.waitPeriod = int(cfg.Time.Twice * 10)  // Convert to 100ms units
	c.pressPeriod = int(cfg.Time.Press * 10) // Convert to 100ms units
	c.bufferSize = c.pressPeriod

	// Create regex patterns for button events
	c.patterns = map[string]*regexp.Regexp{
		"click": regexp.MustCompile(fmt.Sprintf(`1+0+1{%d,}`, c.waitPeriod)),
		"twice": regexp.MustCompile(`1+0+1+0+1{3,}`),
		"press": regexp.MustCompile(fmt.Sprintf(`1+0{%d,}`, c.pressPeriod)),
	}

	log.Printf("Button controller initialized on GPIO%s_%s", hwConfig.ButtonChip, hwConfig.ButtonLine)
	return nil
}

// Start begins the button monitoring
func (c *Controller) Start() error {
	c.mutex.Lock()
	defer c.mutex.Unlock()

	if c.running {
		return fmt.Errorf("button controller already running")
	}

	if c.pin == nil {
		if err := c.Initialize(); err != nil {
			return fmt.Errorf("failed to initialize button control: %v", err)
		}
	}

	c.running = true
	c.stopCh = make(chan struct{})

	go c.monitorLoop()
	log.Println("Button controller started")
	return nil
}

// Stop stops the button monitoring
func (c *Controller) Stop() {
	c.mutex.Lock()
	defer c.mutex.Unlock()

	if !c.running {
		return
	}

	c.running = false
	close(c.stopCh)
	log.Println("Button controller stopped")
}

// GetEventChannel returns the channel for button events
func (c *Controller) GetEventChannel() <-chan string {
	return c.eventCh
}

// monitorLoop is the main button monitoring loop
func (c *Controller) monitorLoop() {
	ticker := time.NewTicker(100 * time.Millisecond) // 100ms polling
	defer ticker.Stop()

	buffer := make([]string, 0, c.bufferSize)

	for {
		select {
		case <-c.stopCh:
			return
		case <-ticker.C:
			// Read current pin state
			level := gpio.High
			if c.pin.Read() == gpio.Low {
				level = gpio.Low
			}

			// Convert to string (1 for high, 0 for low)
			var stateStr string
			if level == gpio.High {
				stateStr = "1"
			} else {
				stateStr = "0"
			}

			// Add to buffer
			buffer = append(buffer, stateStr)

			// Keep buffer size manageable
			if len(buffer) > c.bufferSize {
				buffer = buffer[1:]
			}

			// Check for patterns if buffer has enough data
			if len(buffer) >= 10 { // Minimum buffer size for pattern matching
				bufferStr := strings.Join(buffer, "")
				event := c.matchPattern(bufferStr)
				if event != "" {
					select {
					case c.eventCh <- event:
						// Clear buffer after detecting an event
						buffer = buffer[:0]
					default:
						// Channel full, skip this event
					}
				}
			}
		}
	}
}

// matchPattern checks if the buffer matches any button event pattern
func (c *Controller) matchPattern(buffer string) string {
	// Check patterns in order of priority
	for _, event := range []string{"press", "twice", "click"} {
		if pattern, exists := c.patterns[event]; exists {
			if pattern.MatchString(buffer) {
				log.Printf("Button event detected: %s", event)
				return event
			}
		}
	}
	return ""
}

// IsRunning returns whether the button controller is running
func (c *Controller) IsRunning() bool {
	c.mutex.RLock()
	defer c.mutex.RUnlock()
	return c.running
}

// SetPin allows setting a custom pin for testing
func (c *Controller) SetPin(pin gpio.PinIn) {
	c.mutex.Lock()
	defer c.mutex.Unlock()
	c.pin = pin
}

// UpdateConfig updates the button timing configuration
func (c *Controller) UpdateConfig() {
	c.mutex.Lock()
	defer c.mutex.Unlock()

	cfg := config.GlobalConfig
	c.waitPeriod = int(cfg.Time.Twice * 10)
	c.pressPeriod = int(cfg.Time.Press * 10)
	c.bufferSize = c.pressPeriod

	// Recreate patterns with new timing
	c.patterns = map[string]*regexp.Regexp{
		"click": regexp.MustCompile(fmt.Sprintf(`1+0+1{%d,}`, c.waitPeriod)),
		"twice": regexp.MustCompile(`1+0+1+0+1{3,}`),
		"press": regexp.MustCompile(fmt.Sprintf(`1+0{%d,}`, c.pressPeriod)),
	}

	log.Printf("Button timing updated: twice=%.1fs, press=%.1fs",
		cfg.Time.Twice, cfg.Time.Press)
}
