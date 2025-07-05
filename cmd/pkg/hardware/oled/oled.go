package oled

import (
	"context"
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"log"
	"sync"
	"time"

	"github.com/radxa/rocki-penta-golang/pkg/config"
	"github.com/radxa/rocki-penta-golang/pkg/sysinfo"
	"golang.org/x/image/font"
	"golang.org/x/image/font/basicfont"
	"golang.org/x/image/math/fixed"
)

// Display represents the OLED display
type Display struct {
	cfg         *config.Config
	width       int
	height      int
	currentPage int
	mu          sync.Mutex
	i2c         *I2CDevice
	buffer      []byte
	image       *image.RGBA
}

// I2CDevice represents an I2C device for the OLED display
type I2CDevice struct {
	address byte
	bus     int
	// In a real implementation, this would contain the actual I2C connection
	// For now, we'll simulate the interface
}

// DisplayPage represents a page of information to display
type DisplayPage struct {
	Lines []DisplayLine
}

// DisplayLine represents a line of text on the display
type DisplayLine struct {
	X    int
	Y    int
	Text string
	Font font.Face
}

// New creates a new OLED display
func New(cfg *config.Config) (*Display, error) {
	// For now, we'll create a mock I2C device
	// In a real implementation, this would initialize the actual I2C connection
	i2c := &I2CDevice{
		address: 0x3C, // Common SSD1306 address
		bus:     1,    // I2C bus 1
	}

	display := &Display{
		cfg:         cfg,
		width:       128,
		height:      32,
		currentPage: 0,
		i2c:         i2c,
		buffer:      make([]byte, 128*32/8), // 1 bit per pixel
		image:       image.NewRGBA(image.Rect(0, 0, 128, 32)),
	}

	// Initialize the display
	if err := display.init(); err != nil {
		return nil, fmt.Errorf("failed to initialize display: %w", err)
	}

	return display, nil
}

// init initializes the OLED display
func (d *Display) init() error {
	// Clear the display
	d.clear()
	return d.show()
}

// clear clears the display buffer
func (d *Display) clear() {
	// Clear the image
	draw.Draw(d.image, d.image.Bounds(), &image.Uniform{color.Black}, image.Point{}, draw.Src)
	
	// Clear the buffer
	for i := range d.buffer {
		d.buffer[i] = 0
	}
}

// show updates the physical display with the current buffer
func (d *Display) show() error {
	// Convert image to buffer
	d.imageToBuffer()
	
	// In a real implementation, this would send the buffer to the I2C device
	// For now, we'll just log that we're updating the display
	log.Printf("Updating OLED display")
	
	return nil
}

// imageToBuffer converts the image to the display buffer format
func (d *Display) imageToBuffer() {
	bounds := d.image.Bounds()
	for y := bounds.Min.Y; y < bounds.Max.Y; y++ {
		for x := bounds.Min.X; x < bounds.Max.X; x++ {
			c := d.image.At(x, y)
			r, g, b, _ := c.RGBA()
			
			// Convert to grayscale and determine if pixel should be on
			gray := (r + g + b) / 3
			isOn := gray > 0x8000 // 50% threshold
			
			if isOn {
				// Set the bit in the buffer
				page := y / 8
				bit := y % 8
				byteIndex := page*d.width + x
				if byteIndex < len(d.buffer) {
					d.buffer[byteIndex] |= (1 << bit)
				}
			}
		}
	}
}

// drawText draws text on the display
func (d *Display) drawText(x, y int, text string, face font.Face) {
	if face == nil {
		face = basicfont.Face7x13 // Default font
	}

	drawer := &font.Drawer{
		Dst:  d.image,
		Src:  image.NewUniform(color.White),
		Face: face,
		Dot:  fixed.Point26_6{X: fixed.I(x), Y: fixed.I(y)},
	}
	drawer.DrawString(text)
}

// ShowWelcome displays the welcome message
func (d *Display) ShowWelcome() {
	d.mu.Lock()
	defer d.mu.Unlock()
	
	d.clear()
	d.drawText(0, 16, "ROCKPi SATA HAT", basicfont.Face7x13)
	d.drawText(32, 28, "Loading...", basicfont.Face7x13)
	d.show()
}

// ShowGoodbye displays the goodbye message
func (d *Display) ShowGoodbye() {
	d.mu.Lock()
	defer d.mu.Unlock()
	
	d.clear()
	d.drawText(32, 16, "Good Bye ~", basicfont.Face7x13)
	d.show()
}

// generatePages generates display pages with system information
func (d *Display) generatePages(sysInfo *sysinfo.SysInfo) []DisplayPage {
	var pages []DisplayPage

	// Page 0: System info
	uptime, _ := sysInfo.GetUptime()
	cpuTemp, _ := sysInfo.GetCPUTemp(d.cfg.OLED.FTemp)
	ipAddr, _ := sysInfo.GetIPAddress()
	
	pages = append(pages, DisplayPage{
		Lines: []DisplayLine{
			{X: 0, Y: 12, Text: uptime, Font: basicfont.Face7x13},
			{X: 0, Y: 24, Text: cpuTemp, Font: basicfont.Face7x13},
			{X: 0, Y: 36, Text: ipAddr, Font: basicfont.Face7x13},
		},
	})

	// Page 1: CPU and Memory
	cpuLoad, _ := sysInfo.GetCPULoad()
	memUsage, _ := sysInfo.GetMemoryUsage()
	
	pages = append(pages, DisplayPage{
		Lines: []DisplayLine{
			{X: 0, Y: 16, Text: cpuLoad, Font: basicfont.Face7x13},
			{X: 0, Y: 28, Text: memUsage, Font: basicfont.Face7x13},
		},
	})

	// Page 2: Disk info
	diskDevices, diskUsages, err := sysInfo.GetDiskInfo()
	if err == nil && len(diskDevices) > 0 {
		var diskLines []DisplayLine
		
		// Format disk information
		if len(diskDevices) >= 5 {
			// Multiple disks - compact format
			text1 := fmt.Sprintf("Disk: %s %s", diskDevices[0], diskUsages[0])
			text2 := fmt.Sprintf("%s %s  %s %s", diskDevices[1], diskUsages[1], diskDevices[2], diskUsages[2])
			text3 := fmt.Sprintf("%s %s  %s %s", diskDevices[3], diskUsages[3], diskDevices[4], diskUsages[4])
			
			diskLines = []DisplayLine{
				{X: 0, Y: 12, Text: text1, Font: basicfont.Face7x13},
				{X: 0, Y: 24, Text: text2, Font: basicfont.Face7x13},
				{X: 0, Y: 36, Text: text3, Font: basicfont.Face7x13},
			}
		} else if len(diskDevices) >= 3 {
			// Medium number of disks
			text1 := fmt.Sprintf("Disk: %s %s", diskDevices[0], diskUsages[0])
			text2 := fmt.Sprintf("%s %s  %s %s", diskDevices[1], diskUsages[1], diskDevices[2], diskUsages[2])
			
			diskLines = []DisplayLine{
				{X: 0, Y: 16, Text: text1, Font: basicfont.Face7x13},
				{X: 0, Y: 28, Text: text2, Font: basicfont.Face7x13},
			}
		} else {
			// Single disk
			text1 := fmt.Sprintf("Disk: %s %s", diskDevices[0], diskUsages[0])
			diskLines = []DisplayLine{
				{X: 0, Y: 20, Text: text1, Font: basicfont.Face7x13},
			}
		}
		
		pages = append(pages, DisplayPage{Lines: diskLines})
	}

	return pages
}

// displayPage displays a specific page
func (d *Display) displayPage(page DisplayPage) {
	d.clear()
	
	// Apply rotation if configured
	for _, line := range page.Lines {
		x, y := line.X, line.Y
		if d.cfg.OLED.Rotate {
			// Rotate 180 degrees
			x = d.width - x - len(line.Text)*7 // Approximate character width
			y = d.height - y
		}
		d.drawText(x, y, line.Text, line.Font)
	}
	
	d.show()
}

// NextPage advances to the next page
func (d *Display) NextPage() {
	d.mu.Lock()
	defer d.mu.Unlock()
	
	// This will be called when the button is pressed
	// For now, we'll just increment the page counter
	d.currentPage++
	log.Printf("OLED: Advanced to page %d", d.currentPage)
}

// Run runs the OLED display loop
func (d *Display) Run(ctx context.Context, sysInfo *sysinfo.SysInfo) {
	ticker := time.NewTicker(d.cfg.GetSliderInterval())
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if d.cfg.Slider.Auto {
				d.mu.Lock()
				pages := d.generatePages(sysInfo)
				if len(pages) > 0 {
					pageIndex := d.currentPage % len(pages)
					d.displayPage(pages[pageIndex])
					d.currentPage++
				}
				d.mu.Unlock()
			}
		}
	}
}

// Close closes the OLED display
func (d *Display) Close() error {
	d.mu.Lock()
	defer d.mu.Unlock()
	
	// Clear the display
	d.clear()
	d.show()
	
	// In a real implementation, we would close the I2C connection here
	log.Println("OLED display closed")
	return nil
} 