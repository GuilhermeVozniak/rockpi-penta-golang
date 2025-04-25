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
)

const (
	oledWidth         = 128
	oledHeight        = 32
	oledDefaultI2CBus = "/dev/i2c-1" // Common default, might need config/env var
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
	i2cBusName := oledDefaultI2CBus // TODO: Make configurable if needed
	oledResetPinName := os.Getenv("OLED_RESET")
	if oledResetPinName == "" {
		log.Println("Warning: OLED_RESET environment variable not set. Display might not initialize correctly without reset.")
	}

	// Open I2C bus
	i2cBus, err := i2creg.Open(i2cBusName)
	if err != nil {
		return nil, fmt.Errorf("failed to open I2C bus %s: %w", i2cBusName, err)
	}
	// Note: Closing the bus will likely be handled globally at application shutdown

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
	// NewI2C expects only bus and opts
	dev, err := ssd1306.NewI2C(i2cBus, devOpts)
	if err != nil {
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

// DisplayPage shows a specific page of system information.
func (o *OLEDDisplay) DisplayPage(pageIndex int) error {
	o.Clear()

	// Fetch info (handle errors gracefully)
	uptime, errUptime := sysinfo.GetUptime()
	if errUptime != nil {
		uptime = "Uptime: Err"
		log.Println(errUptime)
	}
	cpuTemp, errTemp := sysinfo.GetCPUTemp(o.conf.OLED.FTemp)
	if errTemp != nil {
		cpuTemp = "Temp: Err"
		log.Println(errTemp)
	}
	ipAddr, errIP := sysinfo.GetIPAddress()
	if errIP != nil {
		ipAddr = "IP: Err"
		log.Println(errIP)
	}
	cpuLoad, errLoad := sysinfo.GetCPULoad()
	if errLoad != nil {
		cpuLoad = "Load: Err"
		log.Println(errLoad)
	}
	memUsage, errMem := sysinfo.GetMemUsage()
	if errMem != nil {
		memUsage = "Mem: Err"
		log.Println(errMem)
	}
	diskLines, errDisk := sysinfo.FormatDiskInfoForOLED()
	if errDisk != nil {
		diskLines = []string{"Disk: Err"}
		log.Println(errDisk)
	}

	numPages := 3
	currentPage := pageIndex % numPages
	log.Printf("Displaying OLED Page %d", currentPage)

	switch currentPage {
	case 0: // Uptime, Temp, IP
		o.DrawText(0, o.lineHeight, uptime, color.White)
		o.DrawText(0, o.lineHeight*2, cpuTemp, color.White)
		o.DrawText(0, o.lineHeight*3, ipAddr, color.White)
	case 1: // CPU Load, Mem
		o.DrawText(0, o.lineHeight+2, cpuLoad, color.White)
		o.DrawText(0, o.lineHeight*2+4, memUsage, color.White)
	case 2: // Disk Info
		y := o.lineHeight
		for i, line := range diskLines {
			if i >= 3 {
				break
			} // Max 3 lines on 32px display
			o.DrawText(0, y, line, color.White)
			y += o.lineHeight
		}
	}

	return o.Show()
}

// RunSlider manages the automatic and manual OLED page sliding.
func RunSlider(conf *config.Config, oled *OLEDDisplay, buttonChan <-chan button.ButtonEvent, stopChan chan struct{}) {
	log.Println("Starting OLED slider...")
	// Convert float64 seconds to time.Duration
	sliderInterval := time.Duration(conf.Slider.Time * float64(time.Second))
	ticker := time.NewTicker(sliderInterval)
	defer ticker.Stop()

	displayCurrentPage := func() {
		if err := oled.DisplayPage(conf.Idx.Load()); err != nil {
			log.Printf("Error displaying OLED page: %v", err)
		}
	}

	// Initial display
	displayCurrentPage()

	for {
		select {
		case event := <-buttonChan:
			// Check if the action for this button event is 'slider'
			action := button.EventNone // Default action
			switch event {
			case button.EventClick:
				action = button.ButtonEvent(conf.Key.Click)
			case button.EventTwice:
				action = button.ButtonEvent(conf.Key.Twice)
			case button.EventPress:
				action = button.ButtonEvent(conf.Key.Press)
			}

			if action == "slider" {
				log.Println("Slider event triggered by button.")
				conf.Idx.Add(1)
				displayCurrentPage()
				// Reset timer if auto-sliding is enabled
				if conf.Slider.Auto {
					// Convert float64 seconds to time.Duration
					sliderInterval = time.Duration(conf.Slider.Time * float64(time.Second))
					ticker.Reset(sliderInterval)
				}
			}

		case <-ticker.C:
			if conf.Slider.Auto {
				log.Println("Slider event triggered by timer.")
				conf.Idx.Add(1)
				displayCurrentPage()
			} else {
				// If auto is off, the ticker still runs but does nothing
				// We could stop/start it, but this is simpler.
			}

		case <-stopChan:
			log.Println("Stopping OLED slider...")
			// Attempt to clear display on exit
			oled.Clear()
			if err := oled.Show(); err != nil {
				log.Printf("Warning: Failed to clear OLED on exit: %v", err)
			}
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
