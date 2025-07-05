package button

import (
	"context"
	"fmt"
	"log"
	"strconv"
	"time"

	"github.com/radxa/rocki-penta-golang/pkg/config"
	"github.com/warthog618/gpiod"
)

// Handler manages button input
type Handler struct {
	cfg   *config.Config
	line  *gpiod.Line
	chip  *gpiod.Chip
}

// ButtonEvent represents a button event
type ButtonEvent struct {
	Type      string
	Timestamp time.Time
}

// ActionFunc is a function that handles button actions
type ActionFunc func(action string)

// New creates a new button handler
func New(cfg *config.Config) (*Handler, error) {
	chipNum, err := strconv.Atoi(cfg.Hardware.ButtonChip)
	if err != nil {
		return nil, fmt.Errorf("invalid button chip: %s", cfg.Hardware.ButtonChip)
	}

	lineNum, err := strconv.Atoi(cfg.Hardware.ButtonLine)
	if err != nil {
		return nil, fmt.Errorf("invalid button line: %s", cfg.Hardware.ButtonLine)
	}

	chip, err := gpiod.NewChip(fmt.Sprintf("gpiochip%d", chipNum))
	if err != nil {
		return nil, fmt.Errorf("failed to open GPIO chip: %w", err)
	}

	line, err := chip.RequestLine(lineNum, gpiod.AsInput, gpiod.WithPullUp)
	if err != nil {
		chip.Close()
		return nil, fmt.Errorf("failed to request button line: %w", err)
	}

	handler := &Handler{
		cfg:  cfg,
		line: line,
		chip: chip,
	}

	return handler, nil
}

// readButton reads the current button state
func (h *Handler) readButton() (int, error) {
	return h.line.Value()
}

// detectButtonEvent detects button events (click, double-click, long press)
func (h *Handler) detectButtonEvent(ctx context.Context) (string, error) {
	var pattern []int
	var timestamps []time.Time
	
	// Read initial state
	lastState, err := h.readButton()
	if err != nil {
		return "", err
	}
	
	// Set initial state to high (button not pressed)
	if err := h.line.SetValue(1); err != nil {
		log.Printf("Warning: Could not set initial button state: %v", err)
	}
	
	timeout := time.After(h.cfg.GetLongPressTimeout() + time.Second)
	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()
	
	for {
		select {
		case <-ctx.Done():
			return "", ctx.Err()
		case <-timeout:
			// Analyze the pattern
			return h.analyzePattern(pattern, timestamps), nil
		case <-ticker.C:
			state, err := h.readButton()
			if err != nil {
				continue
			}
			
			if state != lastState {
				pattern = append(pattern, state)
				timestamps = append(timestamps, time.Now())
				lastState = state
				
				// Reset timeout for each state change
				timeout = time.After(h.cfg.GetLongPressTimeout())
			}
		}
	}
}

// analyzePattern analyzes the button pattern to determine the event type
func (h *Handler) analyzePattern(pattern []int, timestamps []time.Time) string {
	if len(pattern) == 0 {
		return "none"
	}
	
	// Convert pattern to string for easier matching
	patternStr := ""
	for _, state := range pattern {
		patternStr += fmt.Sprintf("%d", state)
	}
	
	// Calculate durations between state changes
	var durations []time.Duration
	for i := 1; i < len(timestamps); i++ {
		durations = append(durations, timestamps[i].Sub(timestamps[i-1]))
	}
	
	// Analyze patterns
	doubleClickTimeout := h.cfg.GetDoubleClickTimeout()
	longPressTimeout := h.cfg.GetLongPressTimeout()
	
	// Long press: button held down for long time
	if len(pattern) >= 2 && pattern[0] == 1 && pattern[1] == 0 {
		if len(durations) > 0 && durations[0] > longPressTimeout {
			return "press"
		}
	}
	
	// Double click: quick press-release-press pattern
	if len(pattern) >= 4 {
		// Look for pattern like: 1-0-1-0 (pressed-released-pressed-released)
		if pattern[0] == 1 && pattern[1] == 0 && pattern[2] == 1 && pattern[3] == 0 {
			if len(durations) >= 2 && durations[1] < doubleClickTimeout {
				return "twice"
			}
		}
	}
	
	// Single click: simple press-release
	if len(pattern) >= 2 && pattern[0] == 1 && pattern[1] == 0 {
		if len(durations) > 0 && durations[0] < longPressTimeout {
			return "click"
		}
	}
	
	return "none"
}

// Run runs the button handler
func (h *Handler) Run(ctx context.Context, actionFunc ActionFunc) {
	log.Println("Button handler started")
	
	for {
		select {
		case <-ctx.Done():
			return
		default:
			event, err := h.detectButtonEvent(ctx)
			if err != nil {
				if err == context.Canceled {
					return
				}
				log.Printf("Error detecting button event: %v", err)
				time.Sleep(100 * time.Millisecond)
				continue
			}
			
			if event != "none" {
				log.Printf("Button event detected: %s", event)
				
				// Map event to action
				var action string
				switch event {
				case "click":
					action = h.cfg.Key.Click
				case "twice":
					action = h.cfg.Key.Twice
				case "press":
					action = h.cfg.Key.Press
				default:
					action = "none"
				}
				
				if action != "none" {
					actionFunc(action)
				}
			}
		}
	}
}

// Close closes the button handler
func (h *Handler) Close() error {
	if h.line != nil {
		h.line.Close()
	}
	if h.chip != nil {
		return h.chip.Close()
	}
	return nil
} 