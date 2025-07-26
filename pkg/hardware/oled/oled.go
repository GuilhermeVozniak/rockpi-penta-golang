package oled

import (
	"embed"
	"fmt"
	"image"
	"log"
	"sync"
	"time"

	"github.com/fogleman/gg"
	"github.com/golang/freetype/truetype"
	"golang.org/x/image/font"
	"periph.io/x/conn/v3/i2c/i2creg"
	"periph.io/x/devices/v3/ssd1306"
	"periph.io/x/host/v3"

	"github.com/GuilhermeVozniak/rockpi-penta-golang/pkg/config"
	"github.com/GuilhermeVozniak/rockpi-penta-golang/pkg/sysinfo"
)

//go:embed fonts/*
var fontFS embed.FS

type Controller struct {
	device      *ssd1306.Dev
	width       int
	height      int
	ctx         *gg.Context
	fonts       map[int]font.Face
	running     bool
	autoSliding bool
	stopCh      chan struct{}
	mutex       sync.RWMutex
	currentPage int
}

type Page struct {
	Lines []Line
}

type Line struct {
	X    int
	Y    int
	Text string
	Font int
}

var (
	instance *Controller
	once     sync.Once
)

// GetInstance returns the singleton OLED controller
func GetInstance() *Controller {
	once.Do(func() {
		instance = &Controller{
			width:  128,
			height: 32,
			fonts:  make(map[int]font.Face),
			stopCh: make(chan struct{}),
		}
	})
	return instance
}

// Initialize sets up the OLED display
func (c *Controller) Initialize() error {
	c.mutex.Lock()
	defer c.mutex.Unlock()

	// Initialize periph.io
	if _, err := host.Init(); err != nil {
		return fmt.Errorf("failed to initialize periph.io: %v", err)
	}

	// Open I2C bus
	bus, err := i2creg.Open("")
	if err != nil {
		return fmt.Errorf("failed to open I2C bus: %v", err)
	}

	// Initialize SSD1306 display
	opts := ssd1306.DefaultOpts
	opts.W = c.width
	opts.H = c.height

	device, err := ssd1306.NewI2C(bus, &opts)
	if err != nil {
		return fmt.Errorf("failed to initialize SSD1306: %v", err)
	}

	c.device = device

	// Initialize drawing context
	c.ctx = gg.NewContext(c.width, c.height)

	// Load fonts
	if err := c.loadFonts(); err != nil {
		return fmt.Errorf("failed to load fonts: %v", err)
	}

	// Clear display
	c.clear()

	log.Println("OLED controller initialized")
	return nil
}

// loadFonts loads embedded fonts
func (c *Controller) loadFonts() error {
	fontSizes := []int{10, 11, 12, 14}

	// Try to load DejaVu Sans Mono Bold
	fontData, err := fontFS.ReadFile("fonts/DejaVuSansMono-Bold.ttf")
	if err != nil {
		// Fallback to system font or embedded alternative
		log.Printf("Could not load DejaVu font, using fallback: %v", err)
		return c.loadFallbackFonts(fontSizes)
	}

	parsedFont, err := truetype.Parse(fontData)
	if err != nil {
		return c.loadFallbackFonts(fontSizes)
	}

	for _, size := range fontSizes {
		face := truetype.NewFace(parsedFont, &truetype.Options{
			Size: float64(size),
			DPI:  72,
		})
		c.fonts[size] = face
	}

	return nil
}

// loadFallbackFonts loads system fonts as fallback
func (c *Controller) loadFallbackFonts(sizes []int) error {
	// For now, create a basic font face
	// In a real implementation, you might want to load from system fonts
	for _, size := range sizes {
		// This is a placeholder - in production, load actual font files
		c.fonts[size] = nil // Will use default font
	}
	return nil
}

// Start begins the OLED control
func (c *Controller) Start() error {
	c.mutex.Lock()
	defer c.mutex.Unlock()

	if c.running {
		return fmt.Errorf("OLED controller already running")
	}

	if c.device == nil {
		if err := c.Initialize(); err != nil {
			return fmt.Errorf("failed to initialize OLED: %v", err)
		}
	}

	c.running = true
	c.stopCh = make(chan struct{})

	// Show welcome message
	c.showWelcome()

	// Start auto slider if enabled
	if config.GlobalConfig.Slider.Auto {
		go c.autoSliderLoop()
	}

	log.Println("OLED controller started")
	return nil
}

// Stop stops the OLED control
func (c *Controller) Stop() {
	c.mutex.Lock()
	defer c.mutex.Unlock()

	if !c.running {
		return
	}

	c.running = false
	close(c.stopCh)

	// Show goodbye message
	c.showGoodbye()

	log.Println("OLED controller stopped")
}

// clear clears the display
func (c *Controller) clear() {
	if c.ctx != nil {
		c.ctx.SetRGB(0, 0, 0) // Black background
		c.ctx.Clear()
	}
	if c.device != nil {
		// Create black image
		img := image.NewGray(image.Rect(0, 0, c.width, c.height))
		c.device.Draw(c.device.Bounds(), img, image.Point{})
	}
}

// display updates the physical display
func (c *Controller) display() error {
	if c.device == nil || c.ctx == nil {
		return fmt.Errorf("display not initialized")
	}

	img := c.ctx.Image()

	// Convert to grayscale if needed and apply rotation
	var finalImg image.Image = img
	if config.GlobalConfig.OLED.Rotate {
		finalImg = c.rotateImage180(img)
	}

	// Draw to device
	return c.device.Draw(c.device.Bounds(), finalImg, image.Point{})
}

// rotateImage180 rotates an image 180 degrees
func (c *Controller) rotateImage180(img image.Image) image.Image {
	bounds := img.Bounds()
	rotated := image.NewRGBA(bounds)

	for y := bounds.Min.Y; y < bounds.Max.Y; y++ {
		for x := bounds.Min.X; x < bounds.Max.X; x++ {
			newX := bounds.Max.X - 1 - x
			newY := bounds.Max.Y - 1 - y
			rotated.Set(newX, newY, img.At(x, y))
		}
	}

	return rotated
}

// showWelcome displays the welcome message
func (c *Controller) showWelcome() {
	c.clear()
	c.ctx.SetRGB(1, 1, 1) // White text

	// Set font for welcome message
	if font14, exists := c.fonts[14]; exists && font14 != nil {
		c.ctx.SetFontFace(font14)
	}

	c.ctx.DrawString("ROCKPi SATA HAT", 0, 14)

	if font12, exists := c.fonts[12]; exists && font12 != nil {
		c.ctx.SetFontFace(font12)
	}

	c.ctx.DrawString("Loading...", 32, 28)
	c.display()
}

// showGoodbye displays the goodbye message
func (c *Controller) showGoodbye() {
	c.clear()
	c.ctx.SetRGB(1, 1, 1) // White text

	if font14, exists := c.fonts[14]; exists && font14 != nil {
		c.ctx.SetFontFace(font14)
	}

	c.ctx.DrawString("Good Bye ~", 32, 20)
	c.display()

	time.Sleep(2 * time.Second)
	c.clear()
	c.display()
}

// NextSlide advances to the next slide
func (c *Controller) NextSlide() {
	c.mutex.Lock()
	defer c.mutex.Unlock()

	c.currentPage = (c.currentPage + 1) % 3
	c.displayCurrentPage()
}

// displayCurrentPage displays the current page
func (c *Controller) displayCurrentPage() {
	if !c.running {
		return
	}

	pages := c.generatePages()
	if c.currentPage >= len(pages) {
		c.currentPage = 0
	}

	if len(pages) > c.currentPage {
		c.displayPage(pages[c.currentPage])
	}
}

// generatePages creates the display pages based on system info
func (c *Controller) generatePages() []Page {
	sysInfo := sysinfo.GetInstance()

	// Update system info
	sysInfo.Update()

	var pages []Page

	// Page 0: System overview
	page0 := Page{
		Lines: []Line{
			{X: 0, Y: 9, Text: sysInfo.FormatUptime(), Font: 11},
			{X: 0, Y: 21, Text: sysInfo.FormatTemperature(), Font: 11},
			{X: 0, Y: 32, Text: sysInfo.FormatIPAddress(), Font: 11},
		},
	}
	pages = append(pages, page0)

	// Page 1: CPU and Memory
	page1 := Page{
		Lines: []Line{
			{X: 0, Y: 14, Text: sysInfo.FormatCPULoad(), Font: 12},
			{X: 0, Y: 30, Text: sysInfo.FormatMemory(), Font: 12},
		},
	}
	pages = append(pages, page1)

	// Page 2: Disk usage
	page2 := c.generateDiskPage(sysInfo)
	pages = append(pages, page2)

	return pages
}

// generateDiskPage creates the disk usage page
func (c *Controller) generateDiskPage(sysInfo *sysinfo.SystemInfo) Page {
	keys, values := sysInfo.FormatDiskUsage()

	var lines []Line

	if len(keys) == 0 {
		lines = append(lines, Line{X: 0, Y: 16, Text: "No disk info", Font: 12})
		return Page{Lines: lines}
	}

	// Format based on number of disks
	if len(keys) >= 5 {
		// 5 disks - compact layout
		text1 := fmt.Sprintf("Disk: %s", values[0])
		text2 := fmt.Sprintf("%s %s  %s %s", keys[1], values[1], keys[2], values[2])
		text3 := fmt.Sprintf("%s %s  %s %s", keys[3], values[3], keys[4], values[4])

		lines = []Line{
			{X: 0, Y: 9, Text: text1, Font: 11},
			{X: 0, Y: 20, Text: text2, Font: 11},
			{X: 0, Y: 32, Text: text3, Font: 11},
		}
	} else if len(keys) >= 3 {
		// 3 disks - medium layout
		text1 := fmt.Sprintf("Disk: %s", values[0])
		text2 := fmt.Sprintf("%s %s  %s %s", keys[1], values[1], keys[2], values[2])

		lines = []Line{
			{X: 0, Y: 14, Text: text1, Font: 12},
			{X: 0, Y: 30, Text: text2, Font: 12},
		}
	} else {
		// 1-2 disks - large layout
		text1 := fmt.Sprintf("Disk: %s", values[0])
		lines = []Line{
			{X: 0, Y: 16, Text: text1, Font: 14},
		}
	}

	return Page{Lines: lines}
}

// displayPage renders a page to the display
func (c *Controller) displayPage(page Page) {
	c.clear()
	c.ctx.SetRGB(1, 1, 1) // White text

	for _, line := range page.Lines {
		if fontFace, exists := c.fonts[line.Font]; exists && fontFace != nil {
			c.ctx.SetFontFace(fontFace)
		}
		c.ctx.DrawString(line.Text, float64(line.X), float64(line.Y))
	}

	c.display()
}

// autoSliderLoop runs the automatic slide advancing
func (c *Controller) autoSliderLoop() {
	ticker := time.NewTicker(time.Duration(config.GlobalConfig.Slider.Time) * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-c.stopCh:
			return
		case <-ticker.C:
			if config.GlobalConfig.Slider.Auto {
				c.NextSlide()
			} else {
				return // Auto sliding disabled
			}
		}
	}
}

// IsRunning returns whether the OLED controller is running
func (c *Controller) IsRunning() bool {
	c.mutex.RLock()
	defer c.mutex.RUnlock()
	return c.running
}

// SetAutoSliding enables or disables automatic sliding
func (c *Controller) SetAutoSliding(enabled bool) {
	c.mutex.Lock()
	defer c.mutex.Unlock()

	if enabled && !c.autoSliding && c.running {
		c.autoSliding = true
		go c.autoSliderLoop()
	} else if !enabled {
		c.autoSliding = false
	}
}
