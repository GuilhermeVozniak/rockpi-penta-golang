package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/exec"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"github.com/radxa/rocki-penta-golang/pkg/config"
	"github.com/radxa/rocki-penta-golang/pkg/hardware/button"
	"github.com/radxa/rocki-penta-golang/pkg/hardware/fan"
	"github.com/radxa/rocki-penta-golang/pkg/hardware/oled"
	"github.com/radxa/rocki-penta-golang/pkg/sysinfo"
)

func main() {
	// Load configuration
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("Failed to load configuration: %v", err)
	}

	// Initialize system info
	sysInfo := sysinfo.New()

	// Initialize hardware components
	fanController, err := fan.New(cfg)
	if err != nil {
		log.Fatalf("Failed to initialize fan controller: %v", err)
	}
	defer fanController.Close()

	// Try to initialize OLED display (optional)
	var oledDisplay *oled.Display
	var hasOLED bool
	if oledDisplay, err = oled.New(cfg); err != nil {
		log.Printf("OLED display not available: %v", err)
		hasOLED = false
	} else {
		hasOLED = true
		defer oledDisplay.Close()
		oledDisplay.ShowWelcome()
	}

	// Initialize button handler (optional)
	var buttonHandler *button.Handler
	var hasButton bool
	if buttonHandler, err = button.New(cfg); err != nil {
		log.Printf("Button handler not available: %v", err)
		hasButton = false
	} else {
		hasButton = true
		defer buttonHandler.Close()
	}

	// Create context for graceful shutdown
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Channel for coordinating shutdown
	var wg sync.WaitGroup

	// Start fan controller
	wg.Add(1)
	go func() {
		defer wg.Done()
		fanController.Run(ctx)
	}()

	// Start OLED display if available
	if hasOLED {
		wg.Add(1)
		go func() {
			defer wg.Done()
			oledDisplay.Run(ctx, sysInfo)
		}()
	}

	// Start button handler if available
	if hasButton {
		wg.Add(1)
		go func() {
			defer wg.Done()
			buttonHandler.Run(ctx, func(action string) {
				switch action {
				case "slider":
					if hasOLED {
						oledDisplay.NextPage()
					}
				case "switch":
					fanController.Toggle()
				case "reboot":
					log.Println("Rebooting system...")
					exec.Command("reboot").Run()
				case "poweroff":
					log.Println("Powering off system...")
					exec.Command("poweroff").Run()
				}
			})
		}()
	}

	// Wait for interrupt signal
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	log.Println("ROCK Pi Penta SATA HAT controller started")
	<-sigChan

	log.Println("Shutting down...")
	if hasOLED {
		oledDisplay.ShowGoodbye()
		time.Sleep(2 * time.Second)
	}

	cancel()
	wg.Wait()
	fmt.Println("GoodBye ~")
}
