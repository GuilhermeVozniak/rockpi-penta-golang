package main

import (
	"context"
	"log"
	"os"
	"os/exec"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"github.com/GuilhermeVozniak/rockpi-penta-golang/pkg/config"
	"github.com/GuilhermeVozniak/rockpi-penta-golang/pkg/hardware/button"
	"github.com/GuilhermeVozniak/rockpi-penta-golang/pkg/hardware/fan"
	"github.com/GuilhermeVozniak/rockpi-penta-golang/pkg/hardware/oled"
	"github.com/GuilhermeVozniak/rockpi-penta-golang/pkg/sysinfo"
)

type Application struct {
	fanController    *fan.Controller
	oledController   *oled.Controller
	buttonController *button.Controller
	sysInfo          *sysinfo.SystemInfo
	ctx              context.Context
	cancel           context.CancelFunc
	wg               sync.WaitGroup
	hasOLED          bool
}

func main() {
	log.Println("Starting RockPi Penta service...")

	// Load configuration
	cfg := config.Load()
	log.Printf("Configuration loaded: %s", cfg)

	// Create application
	app := &Application{}
	app.ctx, app.cancel = context.WithCancel(context.Background())

	// Initialize components
	if err := app.initialize(); err != nil {
		log.Fatalf("Failed to initialize application: %v", err)
	}

	// Setup signal handling
	signalCh := make(chan os.Signal, 1)
	signal.Notify(signalCh, syscall.SIGINT, syscall.SIGTERM)

	// Start the application
	if err := app.start(); err != nil {
		log.Fatalf("Failed to start application: %v", err)
	}

	log.Println("RockPi Penta service started successfully")

	// Wait for shutdown signal
	select {
	case sig := <-signalCh:
		log.Printf("Received signal %v, shutting down...", sig)
	case <-app.ctx.Done():
		log.Println("Context cancelled, shutting down...")
	}

	// Graceful shutdown
	app.shutdown()
	log.Println("RockPi Penta service stopped")
}

func (app *Application) initialize() error {
	// Initialize system info
	app.sysInfo = sysinfo.GetInstance()

	// Initialize hardware controllers
	app.fanController = fan.GetInstance()
	app.buttonController = button.GetInstance()
	app.oledController = oled.GetInstance()

	// Try to initialize OLED (it might not be available)
	if err := app.oledController.Initialize(); err != nil {
		log.Printf("OLED not available, running without display: %v", err)
		app.hasOLED = false
	} else {
		app.hasOLED = true
		log.Println("OLED display available")
	}

	// Initialize other hardware
	if err := app.fanController.Initialize(); err != nil {
		return err
	}

	if err := app.buttonController.Initialize(); err != nil {
		log.Printf("Button not available, running without button control: %v", err)
	}

	// Update disk devices list
	app.sysInfo.GetBlockDevices()

	return nil
}

func (app *Application) start() error {
	// Start fan controller (this always runs)
	if err := app.fanController.Start(); err != nil {
		return err
	}

	// Start OLED if available
	if app.hasOLED {
		if err := app.oledController.Start(); err != nil {
			log.Printf("Failed to start OLED: %v", err)
			app.hasOLED = false
		} else {
			log.Println("OLED display started")
		}
	}

	// Start button controller if available
	if err := app.buttonController.Start(); err != nil {
		log.Printf("Button controller not started: %v", err)
	} else {
		// Start button event handler if we have OLED
		if app.hasOLED {
			app.wg.Add(1)
			go app.handleButtonEvents()
		}
	}

	// Start system info updater
	app.wg.Add(1)
	go app.systemInfoUpdater()

	return nil
}

func (app *Application) handleButtonEvents() {
	defer app.wg.Done()

	eventCh := app.buttonController.GetEventChannel()

	for {
		select {
		case <-app.ctx.Done():
			return
		case event := <-eventCh:
			action := config.GlobalConfig.GetKeyAction(event)
			log.Printf("Button event: %s -> action: %s", event, action)
			
			switch action {
			case "slider":
				if app.hasOLED {
					app.oledController.NextSlide()
				}
			case "switch":
				if config.GlobalConfig.ToggleRunning() {
					log.Println("Fan enabled")
				} else {
					log.Println("Fan disabled")
				}
			case "reboot":
				log.Println("Reboot requested via button")
				app.executeSystemCommand("reboot")
			case "poweroff":
				log.Println("Poweroff requested via button")
				app.executeSystemCommand("poweroff")
			case "none":
				// Do nothing
			default:
				log.Printf("Unknown action: %s", action)
			}
		}
	}
}

func (app *Application) systemInfoUpdater() {
	defer app.wg.Done()

	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-app.ctx.Done():
			return
		case <-ticker.C:
			// Update block devices list
			app.sysInfo.GetBlockDevices()
			
			// Update system info
			if err := app.sysInfo.Update(); err != nil {
				log.Printf("Failed to update system info: %v", err)
			}
		}
	}
}

func (app *Application) executeSystemCommand(cmd string) {
	// Execute system command in a goroutine to avoid blocking
	go func() {
		log.Printf("Executing system command: %s", cmd)
		
		// Give some time for logging to complete
		time.Sleep(1 * time.Second)
		
		if err := exec.Command("sudo", cmd).Run(); err != nil {
			log.Printf("Failed to execute %s: %v", cmd, err)
		}
	}()
}

func (app *Application) shutdown() {
	log.Println("Shutting down application...")

	// Cancel context to stop goroutines
	app.cancel()

	// Stop hardware controllers
	if app.fanController != nil {
		app.fanController.Stop()
	}

	if app.buttonController != nil {
		app.buttonController.Stop()
	}

	if app.oledController != nil {
		app.oledController.Stop()
	}

	// Wait for goroutines to finish
	done := make(chan struct{})
	go func() {
		app.wg.Wait()
		close(done)
	}()

	// Wait for graceful shutdown or timeout
	select {
	case <-done:
		log.Println("All goroutines stopped")
	case <-time.After(5 * time.Second):
		log.Println("Shutdown timeout, forcing exit")
	}
} 