package sysinfo

import (
	"fmt"
	"log"
	"os/exec"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
)

// runCommand executes a shell command and returns its trimmed output.
func runCommand(command string) (string, error) {
	cmd := exec.Command("sh", "-c", command)
	out, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("error running command '%s': %w", command, err)
	}
	return strings.TrimSpace(string(out)), nil
}

// GetUptime retrieves the system uptime.
func GetUptime() (string, error) {
	// uptime command example: " 10:48:37 up 1 day, 19:39,  1 user,  load average: 0.09, 0.07, 0.02"
	// We want "1 day, 19:39"
	cmd := "uptime | sed 's/.*up \\([^,]*\\), .*/\\1/'"
	out, err := runCommand(cmd)
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("Uptime: %s", out), nil
}

// GetCPUTemp retrieves the CPU temperature.
func GetCPUTemp(useFahrenheit bool) (string, error) {
	cmd := "cat /sys/class/thermal/thermal_zone0/temp"
	out, err := runCommand(cmd)
	if err != nil {
		return "", err
	}
	tempMilliC, err := strconv.ParseFloat(out, 64)
	if err != nil {
		return "", fmt.Errorf("error parsing temperature '%s': %w", out, err)
	}
	tempC := tempMilliC / 1000.0

	if useFahrenheit {
		tempF := tempC*1.8 + 32
		return fmt.Sprintf("CPU Temp: %.0f°F", tempF), nil
	}
	return fmt.Sprintf("CPU Temp: %.1f°C", tempC), nil
}

// GetIPAddress retrieves the primary IP address.
func GetIPAddress() (string, error) {
	// hostname -I example: "192.168.1.100 172.17.0.1 "
	cmd := "hostname -I | awk '{printf \"%s\", $1}'" // Get the first IP
	out, err := runCommand(cmd)
	if err != nil {
		return "", err
	}
	if out == "" {
		return "IP: Not Found", nil
	}
	return fmt.Sprintf("IP %s", out), nil
}

// GetCPULoad retrieves the 1-minute load average.
func GetCPULoad() (string, error) {
	// uptime example: " 10:48:37 up 1 day, 19:39,  1 user,  load average: 0.09, 0.07, 0.02"
	cmd := "uptime | awk '{printf \"%.2f\", $(NF-2)}'" // Get the third to last field
	out, err := runCommand(cmd)
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("CPU Load: %s", out), nil
}

// GetMemUsage retrieves memory usage information.
func GetMemUsage() (string, error) {
	// free -m example:
	//                total        used        free      shared  buff/cache   available
	// Mem:           3856         308        2788          14         759        3289
	// Swap:             0           0           0
	cmd := "free -m | awk 'NR==2{printf \"%s/%sMB\", $3,$2}'" // Get used/total from the second line
	out, err := runCommand(cmd)
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("Mem: %s", out), nil
}

// --- Disk Info --- Needs caching like the Python version

type DiskInfoCache struct {
	RootUsage     string
	PentaDisks    []string // Names like sda, sdb (read from config or detected)
	PentaUsage    map[string]string
	LastRefreshed time.Time
}

var diskCache = DiskInfoCache{PentaUsage: make(map[string]string)}
var diskRefreshInterval = 30 * time.Second // Cache disk info for 30 seconds

// SetPentaDiskNames sets the list of disk names to monitor (e.g., ["sda", "sdb"]).
// This should be called after loading the config or detecting disks.
func SetPentaDiskNames(names []string) {
	// In the python version, this was done by calling get_blk(), which dynamically found sdX devices.
	// For now, we assume they might be set externally or we need a detection mechanism.
	// Let's simulate detection for now.
	// TODO: Implement proper disk detection if needed, similar to get_blk
	detectedNames, err := detectBlockDevices()
	if err != nil {
		fmt.Println("Warning: Could not detect block devices:", err)
		// Fallback to using provided names or empty list
		if names != nil {
			diskCache.PentaDisks = names
		} else {
			diskCache.PentaDisks = []string{}
		}
	} else {
		diskCache.PentaDisks = detectedNames
	}
	fmt.Printf("Monitoring disks: %v\n", diskCache.PentaDisks)
	// Clear old usage data when names change
	diskCache.PentaUsage = make(map[string]string)
}

// detectBlockDevices tries to list sd* devices similar to the Python version.
func detectBlockDevices() ([]string, error) {
	cmd := "lsblk -ndo NAME,TYPE | awk '$2==\"disk\" && $1 ~ /^sd/ {print $1}'"
	out, err := runCommand(cmd)
	if err != nil {
		return nil, err
	}
	if out == "" {
		return []string{}, nil
	}
	return strings.Fields(out), nil
}

// refreshDiskInfoIfNeeded updates the cached disk usage info if the cache is stale.
func refreshDiskInfoIfNeeded() error {
	if time.Since(diskCache.LastRefreshed) < diskRefreshInterval {
		return nil // Cache is fresh
	}

	// Refresh Root Usage
	cmdRoot := "df -h | awk '$NF==\"/\"{printf \"%s\", $5}'" // Get percentage used for root
	rootUsage, err := runCommand(cmdRoot)
	if err != nil {
		fmt.Println("Warning: Could not get root disk usage:", err)
		diskCache.RootUsage = "ERR"
	} else {
		diskCache.RootUsage = rootUsage
	}

	// Refresh Penta Disk Usages
	newPentaUsage := make(map[string]string)
	for _, diskName := range diskCache.PentaDisks {
		// Note: Python used df -Bg (Gigabytes). df -h is human-readable.
		// Python used $5 (percentage). Let's stick to percentage.
		// Need to handle cases where the disk isn't mounted or df fails.
		// The awk command needs the disk name correctly inserted.
		// We look for mount points associated with partitions of the disk (e.g., /dev/sda1)
		// Simpler approach: Just get the usage percentage for the base device if mounted.
		// This might not be perfect if partitions are mounted separately.
		// Let's refine the command to find mount points for partitions of the disk.
		cmdDisk := fmt.Sprintf("df -h | awk '$1 ~ /\\/dev\\/%s[0-9]*/ {print $5; exit}'", diskName) // Find first partition mount and print usage %
		diskUsage, err := runCommand(cmdDisk)
		if err != nil || diskUsage == "" {
			// If no partition is mounted or error, try the base device (e.g., /dev/sda)
			cmdDiskBase := fmt.Sprintf("df -h | awk '$1 == \"/dev/%s\" {print $5; exit}'", diskName)
			diskUsage, err = runCommand(cmdDiskBase)
			if err != nil || diskUsage == "" {
				fmt.Printf("Warning: Could not get usage for disk %s: %v\n", diskName, err)
				diskUsage = "N/A" // Mark as Not Available
			}
		}
		newPentaUsage[diskName] = diskUsage
	}
	diskCache.PentaUsage = newPentaUsage
	diskCache.LastRefreshed = time.Now()
	return nil
}

// GetDiskInfo returns the cached disk usage information.
// It returns the root usage string and a map of penta disk names to their usage strings.
func GetDiskInfo() (string, map[string]string, error) {
	if err := refreshDiskInfoIfNeeded(); err != nil {
		return "ERR", nil, err // Return error if refresh failed badly
	}
	return diskCache.RootUsage, diskCache.PentaUsage, nil
}

// FormatDiskInfoForOLED prepares disk info strings suitable for the OLED display lines.
func FormatDiskInfoForOLED() ([]string, error) {
	rootUsage, pentaUsage, err := GetDiskInfo()
	if err != nil {
		return []string{"Error getting disk info"}, err
	}

	lines := []string{}
	lines = append(lines, fmt.Sprintf("Disk: root %s", rootUsage))

	// Format Penta disks - try to fit 2 per line like Python
	line2 := ""
	line3 := ""
	i := 0
	for name, usage := range pentaUsage {
		entry := fmt.Sprintf("%s %s", name, usage)
		if i < 2 {
			line2 += entry + "  "
		} else if i < 4 {
			line3 += entry + "  "
		}
		i++
		if i >= 4 { // Max 4 penta disks shown
			break
		}
	}

	if line2 != "" {
		lines = append(lines, strings.TrimSpace(line2))
	}
	if line3 != "" {
		lines = append(lines, strings.TrimSpace(line3))
	}

	return lines, nil
}

// --- Disk Temperature Monitoring ---

// DiskTemp holds temperature information for a disk
type DiskTemp struct {
	Device      string  // Device name (e.g., sda)
	Temperature float64 // Temperature in Celsius
	LastUpdated time.Time
}

var (
	diskTempCache     map[string]DiskTemp // Cache of disk temperatures
	diskTempCacheMu   sync.RWMutex        // Mutex to protect diskTempCache
	diskTempCacheTime = 60 * time.Second  // Cache disk temp for 60 seconds
	lastDiskScanTime  time.Time           // Time of last disk scan
	diskScanInterval  = 300 * time.Second // Scan for new disks every 5 minutes
	knownDisks        []string            // List of known disk devices
	diskMutex         sync.Mutex          // Mutex for disk operations
)

func init() {
	diskTempCache = make(map[string]DiskTemp)
	knownDisks = []string{}
}

// DetectDisks returns a list of disk device names (e.g., sda, sdb)
func DetectDisks() ([]string, error) {
	diskMutex.Lock()
	defer diskMutex.Unlock()

	// Rescan for disks periodically
	if time.Since(lastDiskScanTime) < diskScanInterval && len(knownDisks) > 0 {
		return knownDisks, nil
	}

	cmd := exec.Command("lsblk", "-ndo", "NAME,TYPE")
	output, err := cmd.Output()
	if err != nil {
		return knownDisks, fmt.Errorf("error running lsblk: %w", err)
	}

	var disks []string
	for _, line := range strings.Split(string(output), "\n") {
		fields := strings.Fields(line)
		if len(fields) >= 2 && fields[1] == "disk" && strings.HasPrefix(fields[0], "sd") {
			disks = append(disks, fields[0])
		}
	}

	knownDisks = disks
	lastDiskScanTime = time.Now()

	fmt.Printf("Detected %d storage disks: %v\n", len(disks), disks)
	return disks, nil
}

// GetDiskTemperature reads the temperature of a disk using smartctl
func GetDiskTemperature(disk string) (float64, error) {
	diskTempCacheMu.RLock()
	cachedTemp, exists := diskTempCache[disk]
	diskTempCacheMu.RUnlock()

	// Return cached value if recent enough
	if exists && time.Since(cachedTemp.LastUpdated) < diskTempCacheTime {
		return cachedTemp.Temperature, nil
	}

	// Check if smartctl is available
	if _, err := exec.LookPath("smartctl"); err != nil {
		return 0, fmt.Errorf("smartctl not found: %w", err)
	}

	devicePath := "/dev/" + disk
	cmd := exec.Command("smartctl", "-A", devicePath)
	output, err := cmd.Output()
	if err != nil {
		// Try with sudo if permission denied
		if exitErr, ok := err.(*exec.ExitError); ok && exitErr.ExitCode() == 1 {
			sudoCmd := exec.Command("sudo", "smartctl", "-A", devicePath)
			output, err = sudoCmd.Output()
			if err != nil {
				return 0, fmt.Errorf("error running sudo smartctl on %s: %w", devicePath, err)
			}
		} else {
			return 0, fmt.Errorf("error running smartctl on %s: %w", devicePath, err)
		}
	}

	// Parse output to find temperature attribute
	// Temperature is usually attribute 194 (Temperature_Celsius) or 190 (Airflow_Temperature_Cel)
	tempRegex := regexp.MustCompile(`(?i)(Temperature_Celsius|Airflow_Temperature_Cel|Temperature).*?(\d+)(?:\s|$)`)
	matches := tempRegex.FindStringSubmatch(string(output))
	if len(matches) >= 3 {
		temp, err := strconv.ParseFloat(matches[2], 64)
		if err != nil {
			return 0, fmt.Errorf("error parsing temperature value '%s': %w", matches[2], err)
		}

		// Update cache
		diskTempCacheMu.Lock()
		diskTempCache[disk] = DiskTemp{
			Device:      disk,
			Temperature: temp,
			LastUpdated: time.Now(),
		}
		diskTempCacheMu.Unlock()

		return temp, nil
	}

	// If we can't find temperature in standard attributes, look for the raw value
	rawTempRegex := regexp.MustCompile(`(?i)(194|190)\s+.*?(\d+)(?:\s|$)`)
	matches = rawTempRegex.FindStringSubmatch(string(output))
	if len(matches) >= 3 {
		temp, err := strconv.ParseFloat(matches[2], 64)
		if err != nil {
			return 0, fmt.Errorf("error parsing temperature value '%s': %w", matches[2], err)
		}

		// Update cache
		diskTempCacheMu.Lock()
		diskTempCache[disk] = DiskTemp{
			Device:      disk,
			Temperature: temp,
			LastUpdated: time.Now(),
		}
		diskTempCacheMu.Unlock()

		return temp, nil
	}

	return 0, fmt.Errorf("temperature attribute not found in smartctl output for %s", devicePath)
}

// GetAllDiskTemperatures returns a map of disk names to temperatures
func GetAllDiskTemperatures() (map[string]float64, error) {
	disks, err := DetectDisks()
	if err != nil {
		return nil, fmt.Errorf("error detecting disks: %w", err)
	}

	temps := make(map[string]float64)
	for _, disk := range disks {
		temp, err := GetDiskTemperature(disk)
		if err != nil {
			log.Printf("Warning: Could not get temperature for disk %s: %v", disk, err)
			continue
		}
		temps[disk] = temp
	}

	return temps, nil
}

// GetHighestDiskTemperature returns the highest temperature among all disks
func GetHighestDiskTemperature() (float64, error) {
	temps, err := GetAllDiskTemperatures()
	if err != nil {
		return 0, err
	}

	if len(temps) == 0 {
		return 0, fmt.Errorf("no disk temperatures available")
	}

	var highest float64
	for _, temp := range temps {
		if temp > highest {
			highest = temp
		}
	}

	return highest, nil
}

// FormatDiskTemperaturesForOLED prepares disk temperature strings for the OLED display
func FormatDiskTemperaturesForOLED() ([]string, error) {
	temps, err := GetAllDiskTemperatures()
	if err != nil {
		return []string{"Error getting disk temps"}, err
	}

	if len(temps) == 0 {
		return []string{"No disk temps available"}, nil
	}

	// Format as "sdX: YY°C"
	lines := []string{"Disk Temperatures:"}
	currentLine := ""
	count := 0

	// Sort disk names for consistent display
	var diskNames []string
	for disk := range temps {
		diskNames = append(diskNames, disk)
	}
	sort.Strings(diskNames)

	for _, disk := range diskNames {
		temp := temps[disk]
		entry := fmt.Sprintf("%s:%.0f°C", disk, temp)

		if count%2 == 0 {
			currentLine = entry
		} else {
			currentLine += " " + entry
			lines = append(lines, currentLine)
			currentLine = ""
		}
		count++
	}

	// Add last line if odd number of disks
	if currentLine != "" {
		lines = append(lines, currentLine)
	}

	return lines, nil
}
