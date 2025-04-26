package oled

import (
	"errors"
	"fmt"
	"image"
	"image/color"
	"log"
	"os"
	"strings"
	"sync"
	"time"

	"rockpi-penta-golang/pkg/config"
	"rockpi-penta-golang/pkg/hardware/button"
	"rockpi-penta-golang/pkg/sysinfo"

	"golang.org/x/image/font"
	"golang.org/x/image/font/basicfont"
	"golang.org/x/image/math/fixed"

	"periph.io/x/conn/v3/gpio"
	"periph.io/x/conn/v3/gpio/gpioreg"
	"periph.io/x/conn/v3/i2c/i2creg"
	"periph.io/x/devices/v3/ssd1306"
	"periph.io/x/devices/v3/ssd1306/image1bit"
	"periph.io/x/host/v3"
)

const (
	oledWidth         = 128
	oledHeight        = 32
	oledDefaultI2CBus = "/dev/i2c-1" // Default, will be overridden by OLED_I2C_BUS env var if present
)

// OLEDDisplay handles drawing operations on the SSD1306.
type OLEDDisplay struct {
	dev        *ssd1306.Dev
	img        *image1bit.VerticalLSB
	lineHeight int
	face       font.Face
	mu         sync.Mutex // Protect concurrent access to the display
	conf       *config.Config
}

// newOLEDDisplay initializes the I2C connection and the display driver.
func newOLEDDisplay(conf *config.Config) (*OLEDDisplay, error) {
	// Initialize host drivers first
	if _, err := host.Init(); err != nil {
		return nil, fmt.Errorf("failed to initialize periph host: %w", err)
	}

	// Get I2C bus name from environment variable or use default
	i2cBusName := os.Getenv("OLED_I2C_BUS")
	if i2cBusName == "" {
		i2cBusName = oledDefaultI2CBus
		log.Printf("OLED_I2C_BUS environment variable not set, using default: %s", i2cBusName)
	} else {
		log.Printf("Using I2C bus from environment variable: %s", i2cBusName)
	}

	// Check if we should try all available I2C buses if the specified one fails
	tryAllBuses := os.Getenv("OLED_TRY_ALL_BUSES") == "1"

	oledResetPinName := os.Getenv("OLED_RESET")
	if oledResetPinName == "" {
		log.Println("Warning: OLED_RESET environment variable not set. Display might not initialize correctly without reset.")
	}

	// Open I2C bus
	i2cBus, err := i2creg.Open(i2cBusName)
	if err != nil {
		if !tryAllBuses {
			return nil, fmt.Errorf("failed to open I2C bus %s: %w", i2cBusName, err)
		}

		// Try to find any available I2C bus
		log.Printf("Failed to open specified I2C bus %s, trying to find available buses...", i2cBusName)

		// Get list of available bus names
		var busNames []string
		for _, ref := range i2creg.All() {
			busNames = append(busNames, ref.Name)
		}

		if len(busNames) == 0 {
			return nil, fmt.Errorf("no I2C buses found on the system")
		}

		log.Printf("Found %d I2C buses, trying each one...", len(busNames))
		var lastErr error

		// Try each bus until one works
		for _, busName := range busNames {
			log.Printf("Trying I2C bus: %s", busName)
			i2cBus, lastErr = i2creg.Open(busName)
			if lastErr == nil {
				log.Printf("Successfully opened I2C bus: %s", busName)
				break
			}
		}

		if lastErr != nil {
			return nil, fmt.Errorf("failed to open any I2C bus: %w", lastErr)
		}
	}

	// Get reset pin if specified
	var resetPin gpio.PinIO
	if oledResetPinName != "" {
		resetPin = gpioreg.ByName(oledResetPinName)
		if resetPin == nil {
			log.Printf("Warning: Failed to find GPIO pin for OLED_RESET: %s. Continuing without reset.", oledResetPinName)
		} else {
			log.Printf("Found OLED Reset pin: %s", resetPin.Name())
		}
	}

	// Manually pulse reset pin if available before init
	if resetPin != nil {
		log.Println("Pulsing OLED reset pin...")
		if err := resetPin.Out(gpio.Low); err == nil {
			time.Sleep(10 * time.Millisecond)
			if err := resetPin.Out(gpio.High); err == nil {
				time.Sleep(10 * time.Millisecond)
			} else {
				log.Printf("Warning: Failed to set OLED reset pin high: %v", err)
			}
		} else {
			log.Printf("Warning: Failed to set OLED reset pin low: %v", err)
		}
	}

	// Create SSD1306 device
	// Using default address 0x3C, might need option if 0x3D is used
	devOpts := &ssd1306.Opts{
		W:       oledWidth,
		H:       oledHeight,
		Rotated: conf.OLED.Rotate, // Use rotation from config
		// Reset:   resetPin, // Removed - Handle manually above
	}

	// Try to create the device
	dev, err := ssd1306.NewI2C(i2cBus, devOpts)
	if err != nil {
		// SSD1306 initialization failed
		return nil, fmt.Errorf("failed to initialize SSD1306 device: %w", err)
	}

	log.Printf("Initialized OLED Display (%dx%d, Rotated: %t)", devOpts.W, devOpts.H, devOpts.Rotated)

	// Create image buffer
	img := image1bit.NewVerticalLSB(dev.Bounds())

	// Basic font for now
	face := basicfont.Face7x13
	lineHeight := face.Height

	o := &OLEDDisplay{
		dev:        dev,
		img:        img,
		lineHeight: lineHeight,
		face:       face,
		conf:       conf,
	}

	return o, nil
}

// Clear clears the display buffer.
func (o *OLEDDisplay) Clear() {
	o.mu.Lock()
	defer o.mu.Unlock()
	o.img = image1bit.NewVerticalLSB(o.dev.Bounds()) // Create a new blank image
}

// DrawText draws a line of text at the specified coordinates.
// Note: Coordinates are relative to the top-left, regardless of rotation.
func (o *OLEDDisplay) DrawText(x, y int, text string, col color.Color) {
	o.mu.Lock()
	defer o.mu.Unlock()

	point := fixed.Point26_6{
		X: fixed.Int26_6(x << 6),
		Y: fixed.Int26_6(y << 6),
	}

	d := &font.Drawer{
		Dst:  o.img,
		Src:  image.NewUniform(col),
		Face: o.face,
		Dot:  point,
	}
	d.DrawString(text)
}

// Show sends the buffer content to the display hardware.
func (o *OLEDDisplay) Show() error {
	o.mu.Lock()
	defer o.mu.Unlock()

	err := o.dev.Draw(o.dev.Bounds(), o.img, image.Point{})
	if err != nil {
		return fmt.Errorf("failed to write to OLED display: %w", err)
	}
	return nil
}

// Welcome displays the initial loading message.
func (o *OLEDDisplay) Welcome() error {
	o.Clear()
	o.DrawText(0, o.lineHeight, "ROCKPi SATA HAT", color.White) // Line 1
	o.DrawText(32, o.lineHeight*2+4, "Loading...", color.White) // Line 2ish
	return o.Show()
}

// Goodbye displays the shutdown message.
func (o *OLEDDisplay) Goodbye() error {
	o.Clear()
	o.DrawText(32, o.lineHeight+4, "Good Bye ~", color.White) // Centered-ish
	err := o.Show()
	time.Sleep(2 * time.Second)
	o.Clear()
	errShow := o.Show() // Show cleared screen
	if err != nil {
		return err
	}
	return errShow
}

// DisplayPage renders a specific information page.
func (o *OLEDDisplay) DisplayPage(pageIndex int) error {
	o.Clear()
	white := color.RGBA{255, 255, 255, 255}

	// Draw header with page number
	pageType := ""
	switch pageIndex {
	case 0:
		pageType = "SYSTEM"
	case 1:
		pageType = "DISK"
	case 2:
		pageType = "DISK TEMP"
	case 3:
		pageType = "NET"
	default:
		pageType = fmt.Sprintf("PAGE %d", pageIndex+1)
	}
	o.DrawText(0, 13, fmt.Sprintf("= %s =", pageType), white)

	var lines []string

	switch pageIndex {
	case 0: // System info page
		sysPage1 := []string{
			"", // Empty line after header
			"",
		}

		// Get system metrics
		uptime, errUp := sysinfo.GetUptime()
		if errUp != nil {
			lines = []string{"Error getting uptime"}
			break
		}

		temp, errTemp := sysinfo.GetCPUTemp(o.conf.OLED.FTemp)
		if errTemp != nil {
			lines = []string{"Error getting CPU temp"}
			break
		}

		mem, errMem := sysinfo.GetMemUsage()
		if errMem != nil {
			lines = []string{"Error getting memory usage"}
			break
		}

		load, errLoad := sysinfo.GetCPULoad()
		if errLoad != nil {
			lines = []string{"Error getting CPU load"}
			break
		}

		lines = append(sysPage1, uptime, temp, mem, load)

	case 1: // Disk usage page
		diskLines, errDisk := sysinfo.FormatDiskInfoForOLED()
		if errDisk != nil {
			lines = []string{"Error getting disk info", errDisk.Error()}
			break
		}
		lines = diskLines

	case 2: // Disk temperature page
		diskTempLines, errDiskTemp := sysinfo.FormatDiskTemperaturesForOLED()
		if errDiskTemp != nil {
			lines = []string{"Error getting disk temps", errDiskTemp.Error()}
			break
		}
		lines = diskTempLines

	case 3: // Network page
		ip, errIP := sysinfo.GetIPAddress()
		if errIP != nil {
			lines = []string{"Error getting IP address"}
			break
		}
		lines = []string{"", "", ip} // Simple for now

	default:
		lines = []string{"", "Unknown page", fmt.Sprintf("Index: %d", pageIndex)}
	}

	// Draw lines
	lineHeight := o.lineHeight
	y := 13 + lineHeight // Start after header
	for _, line := range lines {
		o.DrawText(0, y, line, white)
		y += lineHeight
		if y >= oledHeight {
			break // Don't try to draw outside the display
		}
	}

	return o.Show()
}

// RunSlider handles automatic cycling through OLED info pages.
func RunSlider(conf *config.Config, oled *OLEDDisplay, buttonChan <-chan button.ButtonEvent, stopChan chan struct{}) {
	if oled == nil {
		log.Println("Warning: RunSlider called with nil OLED. Exiting slider.")
		return
	}

	log.Println("Starting OLED slider...")

	const numPages = 4 // Updated: Now includes disk temperature page

	// Initialize to -1 to start on page 0 on first tick
	idx := conf.Idx.Load()
	if idx < 0 || idx >= numPages {
		idx = 0
		conf.Idx.Store(idx)
	}

	// Immediately show the current page
	if err := oled.DisplayPage(idx); err != nil {
		log.Printf("Error displaying initial OLED page: %v", err)
	}

	// Create timer for auto-sliding
	var timer *time.Timer
	var resetTimer func()
	resetTimer = func() {
		if timer != nil {
			timer.Stop()
		}
		// Only auto-slide if enabled in config
		if conf.Slider.Auto {
			timer = time.AfterFunc(time.Duration(conf.Slider.Time*float64(time.Second)), func() {
				// Advance to next page
				idx = (idx + 1) % numPages
				conf.Idx.Store(idx)
				if err := oled.DisplayPage(idx); err != nil {
					log.Printf("Error auto-advancing OLED page: %v", err)
				}
				resetTimer() // Schedule next auto-slide
			})
		}
	}
	resetTimer()

	// Main event loop
	for {
		select {
		case evt := <-buttonChan:
			if evt == button.EventClick {
				// Advance to next page on button click
				idx = (idx + 1) % numPages
				conf.Idx.Store(idx)
				if err := oled.DisplayPage(idx); err != nil {
					log.Printf("Error advancing OLED page on button click: %v", err)
				}
				resetTimer() // Reset auto-slide timer
			}
		case <-stopChan:
			if timer != nil {
				timer.Stop()
			}
			oled.Clear()
			if err := oled.Show(); err != nil {
				log.Printf("Error clearing OLED on shutdown: %v", err)
			}
			log.Println("OLED slider stopped.")
			return
		}
	}
}

// InitializeOLED sets up the display and returns it.
// It returns (nil, nil) if the OLED is not detected (assumed top board not present).
// It returns (nil, error) for other initialization errors.
func InitializeOLED(conf *config.Config) (*OLEDDisplay, error) {
	oled, err := newOLEDDisplay(conf)
	if err != nil {
		// Check for common I2C/device errors suggesting device not present
		if errors.Is(err, os.ErrNotExist) || strings.Contains(err.Error(), "device not found") || strings.Contains(err.Error(), "connection refused") || strings.Contains(err.Error(), "input/output error") {
			log.Println("OLED display not detected (I2C error), assuming top board not present.")
			return nil, nil // Treat as top board not present, return nil error
		}
		// Otherwise, it's a potentially recoverable error during setup
		return nil, fmt.Errorf("failed to initialize OLED: %w", err)
	}

	// Success
	log.Println("OLED Top Board Detected.")
	// Display welcome message
	if err := oled.Welcome(); err != nil {
		log.Printf("Warning: Failed to display welcome message: %v", err)
		// Continue anyway
	}
	return oled, nil
}
