package button

import (
	"errors"
	"fmt"
	"log"
	"os"
	"regexp"
	"strings"
	"time"

	"rockpi-penta-golang/pkg/config"

	"periph.io/x/conn/v3/gpio"
	"periph.io/x/conn/v3/gpio/gpioreg"
	"periph.io/x/host/v3"
)

// ButtonEvent represents a detected button action.
type ButtonEvent string

const (
	EventNone  ButtonEvent = "none"
	EventClick ButtonEvent = "click"
	EventTwice ButtonEvent = "twice"
	EventPress ButtonEvent = "press"
)

// buttonWatcher monitors the button pin and sends events to a channel.
type buttonWatcher struct {
	pin          gpio.PinIO
	eventChan    chan<- ButtonEvent
	conf         *config.Config
	clickPattern *regexp.Regexp
	twicePattern *regexp.Regexp
	pressPattern *regexp.Regexp
	sampleRate   time.Duration
	bufferSize   int
}

// newButtonWatcher initializes the GPIO pin and watcher state.
func newButtonWatcher(eventChan chan<- ButtonEvent, conf *config.Config) (*buttonWatcher, error) {
	chipName := os.Getenv("BUTTON_CHIP")
	lineNumStr := os.Getenv("BUTTON_LINE")
	if chipName == "" || lineNumStr == "" {
		return nil, errors.New("BUTTON_CHIP and BUTTON_LINE environment variables must be set")
	}

	// Initialize periph.io
	if _, err := host.Init(); err != nil {
		return nil, fmt.Errorf("failed to initialize periph host: %w", err)
	}

	// Construct the pin name for gpioreg.ByName
	// Assuming chipName might be like "gpiochip0" and lineNumStr is the offset.
	// Periph format is often like "GPIO<line>" for default chip or "<chip>/<line>"
	// We might need to adjust this based on the actual hardware/driver.
	// Let's try a common format first.
	pinName := fmt.Sprintf("%s/%s", chipName, lineNumStr) // e.g., gpiochip0/27
	p := gpioreg.ByName(pinName)
	if p == nil {
		// Try simpler name if registry didn't find the combined one
		p = gpioreg.ByName(lineNumStr) // e.g., "27"
		if p == nil {
			return nil, fmt.Errorf("failed to find GPIO pin: %s or %s", pinName, lineNumStr)
		}
	}

	log.Printf("Found button pin: %s (%s)\n", p.Name(), p.Function())

	// Configure the pin as input with pull-up (assuming button connects pin to ground when pressed)
	// The Python code set it to OUT then value 1, which simulates an input with pull-up for some drivers.
	// Let's try Input with PullUp directly using periph.io.
	if err := p.In(gpio.PullUp, gpio.BothEdges); err != nil {
		// Fallback: Try mimicking Python's approach if PullUp fails
		log.Printf("Warning: Setting pin as Input/PullUp failed (%v), trying Output/High fallback...", err)
		if errOut := p.Out(gpio.High); errOut != nil {
			return nil, fmt.Errorf("failed to configure button pin %s as InputPullUp (%v) or OutputHigh (%w)", p.Name(), err, errOut)
		}
		log.Println("Configured button pin as Output/High (fallback mode).")
		// Note: Reading might behave differently in this mode.
	}

	// Pre-compile regexps based on config timings
	// Note: Go regex doesn't support {min,} repetition directly like Python.
	// We simulate it by checking buffer length in the loop.
	// Regexps match patterns of 0s (pressed) and 1s (released)
	pressDuration := int(conf.Time.Press / 0.1) // Number of 0.1s samples for press
	// twiceGapMax := int(conf.Time.Twice / 0.1) // Max 0.1s samples between clicks for double // Removed - calculated in checkTwiceTiming

	// Click: 1+ 0+ 1{twiceGapMax,}
	// Simplified: Ends with 1, preceded by 0s, preceded by 1s.
	// We check the length of the final 1s run.
	clickPattern := regexp.MustCompile(`1+0+1+$`)

	// Twice: 1+ 0+ 1+ 0+ 1{3,}
	// Simplified: Ends with 1, preceded by 0, preceded by 1+, preceded by 0+, preceded by 1+
	// We check the length of the final 1s run and the gap in the state buffer.
	twicePattern := regexp.MustCompile(`1+0+1+0+1+$`)

	// Press: 1+ 0{pressDuration,}
	// Simplified: Ends with 0, preceded by 1+. We check length of 0s run.
	pressPattern := regexp.MustCompile(`1+0+$`)

	bw := &buttonWatcher{
		pin:          p,
		eventChan:    eventChan,
		conf:         conf,
		sampleRate:   100 * time.Millisecond, // Match Python's 0.1s sleep
		bufferSize:   pressDuration + 5,      // Buffer needs to be long enough for longest pattern
		clickPattern: clickPattern,
		twicePattern: twicePattern,
		pressPattern: pressPattern,
	}

	return bw, nil
}

// watch runs the button monitoring loop.
func (bw *buttonWatcher) watch() {
	log.Println("Starting button watcher...")
	stateBuffer := ""
	lastLevel := gpio.High // Assume released initially

	// Fill buffer initially with '1's
	for i := 0; i < bw.bufferSize; i++ {
		stateBuffer += "1"
	}

	for {
		level := bw.pin.Read() // Read current pin level
		stateStr := "1"        // 1 = released (High)
		if level == gpio.Low { // 0 = pressed (Low)
			stateStr = "0"
		}

		// Append current state and trim buffer
		stateBuffer += stateStr
		if len(stateBuffer) > bw.bufferSize {
			stateBuffer = stateBuffer[len(stateBuffer)-bw.bufferSize:]
		}

		// Check for patterns ONLY when the button is released (transition to 1)
		if level == gpio.High && lastLevel == gpio.Low {
			log.Printf("Button released. State buffer: %s", stateBuffer)

			// Check for TWICE first (most complex)
			if matches := bw.twicePattern.FindStringSubmatch(stateBuffer); len(matches) > 0 {
				// Need to verify timing constraints that regex alone can't capture easily in Go
				// Python: 1+0+1+0+1{3,} - Check the gap and final press duration
				if bw.checkTwiceTiming(stateBuffer) {
					log.Println("Event: Twice")
					bw.eventChan <- EventTwice
					stateBuffer = strings.Repeat("1", bw.bufferSize) // Reset buffer after event
					lastLevel = level
					continue // Skip other checks
				}
			}

			// Check for CLICK
			if matches := bw.clickPattern.FindStringSubmatch(stateBuffer); len(matches) > 0 {
				// Python: 1+0+1{wait,} -> check length of final 1s run
				if bw.checkClickTiming(stateBuffer) {
					log.Println("Event: Click")
					bw.eventChan <- EventClick
					stateBuffer = strings.Repeat("1", bw.bufferSize) // Reset buffer
					lastLevel = level
					continue // Skip other checks
				}
			}
		}

		// Check for PRESS (ongoing press)
		if level == gpio.Low {
			if matches := bw.pressPattern.FindStringSubmatch(stateBuffer); len(matches) > 0 {
				// Python: 1+0{size,} -> check length of trailing 0s
				if bw.checkPressTiming(stateBuffer) {
					log.Println("Event: Press")
					bw.eventChan <- EventPress
					stateBuffer = strings.Repeat("1", bw.bufferSize) // Reset buffer
					// Keep level Low, but treat as handled
				}
			}
		}

		lastLevel = level
		time.Sleep(bw.sampleRate)
	}
}

// checkClickTiming verifies the 'wait' duration for a click.
func (bw *buttonWatcher) checkClickTiming(buffer string) bool {
	// Pattern: 1+0+1+ (ends in 1)
	// Find the last sequence of 1s.
	lastZero := strings.LastIndex(buffer, "0")
	if lastZero == -1 || lastZero == len(buffer)-1 {
		return false // Should end in 1 and have a 0 before it
	}
	finalOnes := buffer[lastZero+1:]
	waitSamples := int(bw.conf.Time.Twice / 0.1) // Python used 'wait' = twice time
	// Python regex was 1{wait,} meaning *at least* 'wait' samples long
	// This feels counter-intuitive for *click*. Let's re-read python misc.py:112
	// pattern = {'click': re.compile(r'1+0+1{%d,}' % wait), ...}
	// This means the button must be RELEASED for at least `wait` (0.7s default) duration *after* the press.
	// This detects a click *after* the release has stabilized.
	isLongEnoughRelease := len(finalOnes) >= waitSamples
	// We also need to ensure it wasn't a long press (check the duration of the preceding 0s)
	firstOneBeforeLastZero := strings.LastIndex(buffer[:lastZero+1], "1")
	if firstOneBeforeLastZero == -1 {
		return false
	}
	zeroDuration := lastZero - firstOneBeforeLastZero
	pressSamples := int(bw.conf.Time.Press / 0.1)
	isShortEnoughPress := zeroDuration < pressSamples // Must not be a long press

	//log.Printf("Click check: finalOnes=%d >= waitSamples=%d? %t. zeroDuration=%d < pressSamples=%d? %t", len(finalOnes), waitSamples, isLongEnoughRelease, zeroDuration, pressSamples, isShortEnoughPress)

	return isLongEnoughRelease && isShortEnoughPress
}

// checkTwiceTiming verifies the gaps and durations for a double click.
func (bw *buttonWatcher) checkTwiceTiming(buffer string) bool {
	// Pattern: 1+0+1+0+1+ (ends in 1)
	// Python: r'1+0+1+0+1{3,}' - final release at least 3 samples (0.3s)
	// Find the relevant indices
	lastZero := strings.LastIndex(buffer, "0")
	if lastZero == -1 || lastZero == len(buffer)-1 {
		return false
	} // Ends in 1, has 0
	finalOnes := buffer[lastZero+1:]
	if len(finalOnes) < 3 {
		return false
	} // Final release must be >= 0.3s

	lastOneBeforeLastZero := strings.LastIndex(buffer[:lastZero+1], "1")
	if lastOneBeforeLastZero == -1 {
		return false
	} // Must have 1 before last 0
	secondPressDuration := lastZero - lastOneBeforeLastZero

	secondLastZero := strings.LastIndex(buffer[:lastOneBeforeLastZero+1], "0")
	if secondLastZero == -1 {
		return false
	} // Must have another 0 before that 1
	interClickReleaseDuration := lastOneBeforeLastZero - secondLastZero

	firstOneBeforeSecondLastZero := strings.LastIndex(buffer[:secondLastZero+1], "1")
	if firstOneBeforeSecondLastZero == -1 {
		return false
	} // Must have 1 before the first press
	firstPressDuration := secondLastZero - firstOneBeforeSecondLastZero

	// Timing constraints from Python conf['time']['twice'] (default 0.7s)
	maxGapSamples := int(bw.conf.Time.Twice / 0.1)
	pressSamples := int(bw.conf.Time.Press / 0.1)

	// Press durations must not be long presses
	isFirstPressShort := firstPressDuration < pressSamples
	isSecondPressShort := secondPressDuration < pressSamples
	// The release gap between presses must be short enough
	isGapShortEnough := interClickReleaseDuration < maxGapSamples

	//log.Printf("Twice Check: final1s=%d>=3?%t | p1_dur=%d<%d?%t | p2_dur=%d<%d?%t | gap_dur=%d<%d?%t",
	//	len(finalOnes), isFinalReleaseOk, firstPressDuration, pressSamples, isFirstPressShort,
	//	secondPressDuration, pressSamples, isSecondPressShort, interClickReleaseDuration,
	//	maxGapSamples, isGapShortEnough)

	return len(finalOnes) >= 3 && isFirstPressShort && isSecondPressShort && isGapShortEnough
}

// checkPressTiming verifies the duration for a long press.
func (bw *buttonWatcher) checkPressTiming(buffer string) bool {
	// Pattern: 1+0+ (ends in 0)
	// Find the last sequence of 0s.
	lastOne := strings.LastIndex(buffer, "1")
	if lastOne == -1 || lastOne == len(buffer)-1 {
		return false // Should end in 0 and have a 1 before it
	}
	finalZeros := buffer[lastOne+1:]
	pressSamples := int(bw.conf.Time.Press / 0.1)
	// Python regex was 0{size,} meaning *at least* 'size' samples long.
	isLongEnoughPress := len(finalZeros) >= pressSamples
	//log.Printf("Press check: finalZeros=%d >= pressSamples=%d? %t", len(finalZeros), pressSamples, isLongEnoughPress)
	return isLongEnoughPress
}

// StartButtonWatcher creates and starts the button watcher goroutine.
func StartButtonWatcher(conf *config.Config) (<-chan ButtonEvent, error) {
	eventChan := make(chan ButtonEvent, 5) // Buffered channel
	bw, err := newButtonWatcher(eventChan, conf)
	if err != nil {
		close(eventChan)
		return nil, err
	}

	go bw.watch() // Run the watcher in a separate goroutine

	return eventChan, nil
}
