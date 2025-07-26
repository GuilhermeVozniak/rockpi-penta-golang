package sysinfo

import (
	"bufio"
	"fmt"
	"net"
	"os"
	"os/exec"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/GuilhermeVozniak/rockpi-penta-golang/pkg/config"
)

type SystemInfo struct {
	Uptime       string
	CPUTemp      float64
	IPAddress    string
	CPULoad      float64
	MemoryUsed   int
	MemoryTotal  int
	DiskUsage    map[string]DiskInfo
	cacheMutex   sync.RWMutex
	cacheTime    time.Time
	cacheDisk    time.Time
}

type DiskInfo struct {
	Used       string
	Total      string
	Percentage string
}

var (
	instance *SystemInfo
	once     sync.Once
)

// GetInstance returns the singleton SystemInfo instance
func GetInstance() *SystemInfo {
	once.Do(func() {
		instance = &SystemInfo{
			DiskUsage: make(map[string]DiskInfo),
		}
	})
	return instance
}

// Update refreshes system information
func (s *SystemInfo) Update() error {
	s.cacheMutex.Lock()
	defer s.cacheMutex.Unlock()

	now := time.Now()
	
	// Update basic info every time
	if err := s.updateBasicInfo(); err != nil {
		return err
	}

	// Update disk info every 30 seconds
	if now.Sub(s.cacheDisk) > 30*time.Second {
		s.updateDiskInfo()
		s.cacheDisk = now
	}

	s.cacheTime = now
	return nil
}

func (s *SystemInfo) updateBasicInfo() error {
	// Get uptime
	if uptime, err := s.getUptime(); err == nil {
		s.Uptime = uptime
	}

	// Get CPU temperature
	if temp, err := s.getCPUTemp(); err == nil {
		s.CPUTemp = temp
	}

	// Get IP address
	if ip, err := s.getIPAddress(); err == nil {
		s.IPAddress = ip
	}

	// Get CPU load
	if load, err := s.getCPULoad(); err == nil {
		s.CPULoad = load
	}

	// Get memory info
	if used, total, err := s.getMemoryInfo(); err == nil {
		s.MemoryUsed = used
		s.MemoryTotal = total
	}

	return nil
}

func (s *SystemInfo) updateDiskInfo() {
	s.DiskUsage = make(map[string]DiskInfo)
	
	// Get root disk usage
	if info, err := s.getDiskInfo("/"); err == nil {
		s.DiskUsage["root"] = info
	}

	// Get SATA disk usage
	devices := config.GlobalConfig.GetDiskDevices()
	for _, device := range devices {
		mountPoint := fmt.Sprintf("/dev/%s", device)
		if info, err := s.getDiskInfo(mountPoint); err == nil {
			s.DiskUsage[device] = info
		}
	}
}

func (s *SystemInfo) getUptime() (string, error) {
	cmd := exec.Command("sh", "-c", "uptime | sed 's/.*up \\([^,]*\\), .*/\\1/'")
	output, err := cmd.Output()
	if err != nil {
		return "", err
	}
	uptime := strings.TrimSpace(string(output))
	return fmt.Sprintf("Uptime: %s", uptime), nil
}

func (s *SystemInfo) getCPUTemp() (float64, error) {
	data, err := os.ReadFile("/sys/class/thermal/thermal_zone0/temp")
	if err != nil {
		return 0, err
	}
	
	tempStr := strings.TrimSpace(string(data))
	tempMilliC, err := strconv.ParseFloat(tempStr, 64)
	if err != nil {
		return 0, err
	}
	
	return tempMilliC / 1000.0, nil
}

func (s *SystemInfo) getIPAddress() (string, error) {
	cmd := exec.Command("hostname", "-I")
	output, err := cmd.Output()
	if err != nil {
		return "", err
	}
	
	ips := strings.Fields(string(output))
	if len(ips) > 0 {
		return fmt.Sprintf("IP %s", ips[0]), nil
	}
	
	// Fallback to network interface detection
	addrs, err := net.InterfaceAddrs()
	if err != nil {
		return "", err
	}
	
	for _, addr := range addrs {
		if ipnet, ok := addr.(*net.IPNet); ok && !ipnet.IP.IsLoopback() {
			if ipnet.IP.To4() != nil {
				return fmt.Sprintf("IP %s", ipnet.IP.String()), nil
			}
		}
	}
	
	return "IP N/A", nil
}

func (s *SystemInfo) getCPULoad() (float64, error) {
	cmd := exec.Command("sh", "-c", "uptime | awk '{printf \"%.2f\", $(NF-2)}'")
	output, err := cmd.Output()
	if err != nil {
		return 0, err
	}
	
	loadStr := strings.TrimSpace(string(output))
	return strconv.ParseFloat(loadStr, 64)
}

func (s *SystemInfo) getMemoryInfo() (int, int, error) {
	cmd := exec.Command("sh", "-c", "free -m | awk 'NR==2{printf \"%s %s\", $3,$2}'")
	output, err := cmd.Output()
	if err != nil {
		return 0, 0, err
	}
	
	parts := strings.Fields(string(output))
	if len(parts) != 2 {
		return 0, 0, fmt.Errorf("unexpected memory info format")
	}
	
	used, err1 := strconv.Atoi(parts[0])
	total, err2 := strconv.Atoi(parts[1])
	if err1 != nil || err2 != nil {
		return 0, 0, fmt.Errorf("failed to parse memory values")
	}
	
	return used, total, nil
}

func (s *SystemInfo) getDiskInfo(mountPoint string) (DiskInfo, error) {
	var info DiskInfo
	
	cmd := exec.Command("df", "-h", mountPoint)
	output, err := cmd.Output()
	if err != nil {
		return info, err
	}
	
	lines := strings.Split(string(output), "\n")
	if len(lines) < 2 {
		return info, fmt.Errorf("unexpected df output")
	}
	
	// Parse the second line (actual data)
	fields := strings.Fields(lines[1])
	if len(fields) < 5 {
		return info, fmt.Errorf("unexpected df output format")
	}
	
	info.Total = fields[1]
	info.Used = fields[2]
	info.Percentage = fields[4]
	
	return info, nil
}

// GetBlockDevices updates the list of SATA block devices
func (s *SystemInfo) GetBlockDevices() []string {
	cmd := exec.Command("lsblk", "-no", "NAME")
	output, err := cmd.Output()
	if err != nil {
		return []string{}
	}
	
	var devices []string
	scanner := bufio.NewScanner(strings.NewReader(string(output)))
	for scanner.Scan() {
		device := strings.TrimSpace(scanner.Text())
		if strings.HasPrefix(device, "sd") {
			devices = append(devices, device)
		}
	}
	
	// Update global config
	if config.GlobalConfig != nil {
		config.GlobalConfig.SetDiskDevices(devices)
	}
	
	return devices
}

// FormatTemperature formats temperature based on configuration
func (s *SystemInfo) FormatTemperature() string {
	s.cacheMutex.RLock()
	temp := s.CPUTemp
	s.cacheMutex.RUnlock()
	
	if config.GlobalConfig != nil && config.GlobalConfig.OLED.FTemp {
		fahrenheit := temp*1.8 + 32
		return fmt.Sprintf("CPU Temp: %.0f°F", fahrenheit)
	}
	return fmt.Sprintf("CPU Temp: %.1f°C", temp)
}

// FormatUptime returns formatted uptime string
func (s *SystemInfo) FormatUptime() string {
	s.cacheMutex.RLock()
	defer s.cacheMutex.RUnlock()
	return s.Uptime
}

// FormatIPAddress returns formatted IP address string
func (s *SystemInfo) FormatIPAddress() string {
	s.cacheMutex.RLock()
	defer s.cacheMutex.RUnlock()
	return s.IPAddress
}

// FormatCPULoad returns formatted CPU load string
func (s *SystemInfo) FormatCPULoad() string {
	s.cacheMutex.RLock()
	defer s.cacheMutex.RUnlock()
	return fmt.Sprintf("CPU Load: %.2f", s.CPULoad)
}

// FormatMemory returns formatted memory usage string
func (s *SystemInfo) FormatMemory() string {
	s.cacheMutex.RLock()
	defer s.cacheMutex.RUnlock()
	return fmt.Sprintf("Mem: %d/%dMB", s.MemoryUsed, s.MemoryTotal)
}

// FormatDiskUsage returns formatted disk usage for OLED display
func (s *SystemInfo) FormatDiskUsage() ([]string, []string) {
	s.cacheMutex.RLock()
	defer s.cacheMutex.RUnlock()
	
	var keys, values []string
	
	// Add root disk first
	if rootInfo, exists := s.DiskUsage["root"]; exists {
		keys = append(keys, "Disk:")
		values = append(values, rootInfo.Percentage)
	}
	
	// Add SATA disks
	devices := config.GlobalConfig.GetDiskDevices()
	for _, device := range devices {
		if info, exists := s.DiskUsage[device]; exists {
			keys = append(keys, device+":")
			values = append(values, info.Percentage)
		}
	}
	
	return keys, values
}

// CleanupIPCommand removes potential command injection patterns
func cleanupIPCommand(input string) string {
	// Allow only alphanumeric, dots, spaces, and basic IP characters
	re := regexp.MustCompile(`[^a-zA-Z0-9\.\s\-:]`)
	return re.ReplaceAllString(input, "")
} 