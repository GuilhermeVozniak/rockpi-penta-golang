package main

import (
	"log"
	"os"
	"os/exec"
	"os/signal"
	"syscall"

	"rockpi-penta-golang/pkg/config"
	"rockpi-penta-golang/pkg/hardware/button"
	"rockpi-penta-golang/pkg/hardware/fan"
	"rockpi-penta-golang/pkg/hardware/oled"
	"rockpi-penta-golang/pkg/sysinfo"
)

func main() {
	log.Println("Starting RockPi Penta Service...")

	// --- 1. Load Configuration ---
	conf, err := config.LoadConfig()
	if err != nil {
		// LoadConfig logs details, including parsing errors.
		// It returns defaults even on error, so we might continue,
		// but log the fact that defaults are likely in use.
		log.Printf("Warning: Error loading configuration: %v. Continuing with defaults.", err)
		// No fatal exit here, defaults might be sufficient.
	}

	// --- 2. Initialize Hardware & Start Goroutines ---

	// Initialize periph.io drivers (needed for GPIO/I2C)
	// It's safe to call Init multiple times.
	// _, err = host.Init() // Already called in newButtonWatcher/newOLEDDisplay, ensure it's called
	// if err != nil {
	// 	log.Fatalf("Fatal: Failed to initialize periph host drivers: %v", err)
	// }

	// Initialize Disk Detection (happens on first GetDiskInfo call)
	// Force initial detection and cache population now
	log.Println("Detecting disks...")
	sysinfo.SetPentaDiskNames(nil)    // Pass nil to trigger detection
	_, _, err = sysinfo.GetDiskInfo() // Populate cache
	if err != nil {
		log.Printf("Warning: Initial disk info query failed: %v", err)
	}

	// Initialize disk temperature monitoring
	log.Println("Initializing disk temperature monitoring...")
	disks, err := sysinfo.DetectDisks()
	if err != nil {
		log.Printf("Warning: Could not detect disks for temperature monitoring: %v", err)
	} else {
		log.Printf("Disk temperature monitoring enabled for: %v", disks)
		// Try to get an initial reading
		temps, err := sysinfo.GetAllDiskTemperatures()
		if err != nil {
			log.Printf("Warning: Could not get initial disk temperatures: %v", err)
		} else {
			for disk, temp := range temps {
				log.Printf("Disk %s temperature: %.1f°C", disk, temp)
			}
		}
	}

	// Initialize OLED (checks for top board presence)
	oledDisplay, err := oled.InitializeOLED(conf)
	if err != nil {
		log.Fatalf("Fatal: OLED initialization failed critically: %v", err)
	}
	topBoardPresent := (oledDisplay != nil)

	// Start Fan Control
	fanStopChan, err := fan.StartFanControl(conf)
	if err != nil {
		log.Fatalf("Fatal: Failed to start fan control: %v", err)
	}
	log.Println("Fan control started.")

	// Start Button Watcher & OLED Slider *only* if top board is present
	var buttonEventChan <-chan button.ButtonEvent
	var oledStopChan chan struct{}

	if topBoardPresent {
		buttonEventChan, err = button.StartButtonWatcher(conf)
		if err != nil {
			log.Fatalf("Fatal: Failed to start button watcher: %v", err)
		}
		log.Println("Button watcher started.")

		oledStopChan = make(chan struct{})
		go oled.RunSlider(conf, oledDisplay, buttonEventChan, oledStopChan)
		log.Println("OLED slider started.")
	} else {
		log.Println("Top board not present, skipping button watcher and OLED slider.")
		// Create a dummy channel that will never receive to prevent nil channel issues
		buttonEventChan = make(chan button.ButtonEvent)
	}

	// --- 3. Main Event Loop / Signal Handling ---
	shutdownSignals := make(chan os.Signal, 1)
	signal.Notify(shutdownSignals, syscall.SIGINT, syscall.SIGTERM)

	log.Println("Initialization complete. Waiting for events or shutdown signal...")

mainLoop:
	for {
		select {
		case event := <-buttonEventChan:
			if !topBoardPresent {
				continue
			} // Should not happen with dummy chan, but safe check

			var action string
			switch event {
			case button.EventClick:
				action = conf.Key.Click
			case button.EventTwice:
				action = conf.Key.Twice
			case button.EventPress:
				action = conf.Key.Press
			default:
				action = "none"
			}

			log.Printf("Button Event: %s -> Action: %s", event, action)

			switch action {
			case "slider":
				// Handled internally by RunSlider goroutine listening on the same channel
				log.Println("Action 'slider' triggered (handled by OLED goroutine).")
			case "switch":
				newState := conf.Run.Toggle() // Toggle the fan state
				log.Printf("Action 'switch' triggered. Fan enabled: %t", newState)
				// Fan control loop will pick up the new state via conf.run.Load()
			case "reboot":
				log.Println("Action 'reboot' triggered. Initiating reboot...")
				if oledDisplay != nil {
					oledDisplay.Goodbye()
				} // Show goodbye message
				if err := exec.Command("sudo", "reboot").Run(); err != nil {
					log.Printf("Error initiating reboot: %v", err)
				}
				break mainLoop // Exit after initiating reboot
			case "poweroff":
				log.Println("Action 'poweroff' triggered. Initiating power off...")
				if oledDisplay != nil {
					oledDisplay.Goodbye()
				} // Show goodbye message
				if err := exec.Command("sudo", "poweroff").Run(); err != nil {
					log.Printf("Error initiating poweroff: %v", err)
				}
				break mainLoop // Exit after initiating poweroff
			case "none":
				// Do nothing
			default:
				log.Printf("Warning: Unknown action configured: %s", action)
			}

		case sig := <-shutdownSignals:
			log.Printf("Received signal: %v. Initiating graceful shutdown...", sig)
			break mainLoop // Exit the loop to proceed to cleanup
		}
	}

	// --- 4. Cleanup ---
	log.Println("Shutting down goroutines...")

	// Stop OLED slider if running
	if topBoardPresent && oledStopChan != nil {
		close(oledStopChan)
	}

	// Stop Fan control
	close(fanStopChan)

	// Button watcher goroutine will exit on its own or when program terminates
	// OLED display Clear/Goodbye is handled by RunSlider stop or reboot/poweroff actions

	// Wait briefly for goroutines to attempt cleanup (optional)
	// time.Sleep(100 * time.Millisecond)

	log.Println("RockPi Penta Service stopped.")
}
