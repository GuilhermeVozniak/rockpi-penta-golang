package sysinfo

import (
	"fmt"
	"io/ioutil"
	"net"
	"os/exec"
	"strconv"
	"strings"
	"time"
)

// SysInfo provides system information
type SysInfo struct {
	startTime time.Time
}

// New creates a new SysInfo instance
func New() *SysInfo {
	return &SysInfo{
		startTime: time.Now(),
	}
}

// GetUptime returns the system uptime
func (s *SysInfo) GetUptime() (string, error) {
	data, err := ioutil.ReadFile("/proc/uptime")
	if err != nil {
		return "", fmt.Errorf("failed to read uptime: %w", err)
	}

	parts := strings.Fields(string(data))
	if len(parts) < 1 {
		return "", fmt.Errorf("invalid uptime format")
	}

	uptimeSeconds, err := strconv.ParseFloat(parts[0], 64)
	if err != nil {
		return "", fmt.Errorf("failed to parse uptime: %w", err)
	}

	duration := time.Duration(uptimeSeconds) * time.Second
	return fmt.Sprintf("Uptime: %s", formatDuration(duration)), nil
}

// GetCPUTemp returns the CPU temperature
func (s *SysInfo) GetCPUTemp(fahrenheit bool) (string, error) {
	data, err := ioutil.ReadFile("/sys/class/thermal/thermal_zone0/temp")
	if err != nil {
		return "", fmt.Errorf("failed to read temperature: %w", err)
	}

	tempMilli, err := strconv.ParseInt(strings.TrimSpace(string(data)), 10, 64)
	if err != nil {
		return "", fmt.Errorf("failed to parse temperature: %w", err)
	}

	tempC := float64(tempMilli) / 1000.0

	if fahrenheit {
		tempF := tempC*1.8 + 32
		return fmt.Sprintf("CPU Temp: %.0f°F", tempF), nil
	}

	return fmt.Sprintf("CPU Temp: %.1f°C", tempC), nil
}

// GetIPAddress returns the first non-loopback IP address
func (s *SysInfo) GetIPAddress() (string, error) {
	addrs, err := net.InterfaceAddrs()
	if err != nil {
		return "", fmt.Errorf("failed to get network interfaces: %w", err)
	}

	for _, addr := range addrs {
		if ipNet, ok := addr.(*net.IPNet); ok && !ipNet.IP.IsLoopback() {
			if ipNet.IP.To4() != nil {
				return fmt.Sprintf("IP %s", ipNet.IP.String()), nil
			}
		}
	}

	return "IP N/A", nil
}

// GetCPULoad returns the CPU load average
func (s *SysInfo) GetCPULoad() (string, error) {
	data, err := ioutil.ReadFile("/proc/loadavg")
	if err != nil {
		return "", fmt.Errorf("failed to read load average: %w", err)
	}

	parts := strings.Fields(string(data))
	if len(parts) < 3 {
		return "", fmt.Errorf("invalid load average format")
	}

	load1, err := strconv.ParseFloat(parts[0], 64)
	if err != nil {
		return "", fmt.Errorf("failed to parse load average: %w", err)
	}

	return fmt.Sprintf("CPU Load: %.2f", load1), nil
}

// GetMemoryUsage returns memory usage information
func (s *SysInfo) GetMemoryUsage() (string, error) {
	data, err := ioutil.ReadFile("/proc/meminfo")
	if err != nil {
		return "", fmt.Errorf("failed to read memory info: %w", err)
	}

	lines := strings.Split(string(data), "\n")
	memInfo := make(map[string]int64)

	for _, line := range lines {
		if strings.Contains(line, ":") {
			parts := strings.Fields(line)
			if len(parts) >= 2 {
				key := strings.TrimSuffix(parts[0], ":")
				value, err := strconv.ParseInt(parts[1], 10, 64)
				if err == nil {
					memInfo[key] = value
				}
			}
		}
	}

	memTotal := memInfo["MemTotal"]
	memFree := memInfo["MemFree"]
	memBuffers := memInfo["Buffers"]
	memCached := memInfo["Cached"]

	memUsed := memTotal - memFree - memBuffers - memCached
	memTotalMB := memTotal / 1024
	memUsedMB := memUsed / 1024

	return fmt.Sprintf("Mem: %d/%dMB", memUsedMB, memTotalMB), nil
}

// GetDiskUsage returns disk usage for the root filesystem
func (s *SysInfo) GetDiskUsage() (string, error) {
	cmd := exec.Command("df", "-h", "/")
	output, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("failed to get disk usage: %w", err)
	}

	lines := strings.Split(string(output), "\n")
	if len(lines) < 2 {
		return "", fmt.Errorf("invalid df output")
	}

	// Parse the df output
	fields := strings.Fields(lines[1])
	if len(fields) < 5 {
		return "", fmt.Errorf("invalid df output format")
	}

	size := fields[1]
	used := fields[2]
	usage := fields[4]

	return fmt.Sprintf("Disk: %s/%s %s", used, size, usage), nil
}

// GetBlockDevices returns information about block devices
func (s *SysInfo) GetBlockDevices() ([]string, []string, error) {
	cmd := exec.Command("lsblk", "-o", "NAME")
	output, err := cmd.Output()
	if err != nil {
		return nil, nil, fmt.Errorf("failed to get block devices: %w", err)
	}

	lines := strings.Split(string(output), "\n")
	var devices []string
	var usages []string

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "sd") {
			devices = append(devices, line)

			// Get usage for this device
			cmd := exec.Command("df", "-Bg", fmt.Sprintf("/dev/%s", line))
			if output, err := cmd.Output(); err == nil {
				dfLines := strings.Split(string(output), "\n")
				if len(dfLines) >= 2 {
					fields := strings.Fields(dfLines[1])
					if len(fields) >= 5 {
						usages = append(usages, fields[4])
						continue
					}
				}
			}
			usages = append(usages, "N/A")
		}
	}

	return devices, usages, nil
}

// GetDiskInfo returns formatted disk information
func (s *SysInfo) GetDiskInfo() ([]string, []string, error) {
	devices, usages, err := s.GetBlockDevices()
	if err != nil {
		return nil, nil, err
	}

	// Always include root filesystem
	allDevices := []string{"root"}
	allUsages := []string{}

	// Get root filesystem usage
	if rootUsage, err := s.getRootUsage(); err == nil {
		allUsages = append(allUsages, rootUsage)
	} else {
		allUsages = append(allUsages, "N/A")
	}

	// Add other devices
	allDevices = append(allDevices, devices...)
	allUsages = append(allUsages, usages...)

	return allDevices, allUsages, nil
}

// getRootUsage gets the root filesystem usage percentage
func (s *SysInfo) getRootUsage() (string, error) {
	cmd := exec.Command("df", "-h", "/")
	output, err := cmd.Output()
	if err != nil {
		return "", err
	}

	lines := strings.Split(string(output), "\n")
	if len(lines) < 2 {
		return "", fmt.Errorf("invalid df output")
	}

	fields := strings.Fields(lines[1])
	if len(fields) < 5 {
		return "", fmt.Errorf("invalid df output format")
	}

	return fields[4], nil
}

// formatDuration formats a duration in a human-readable way
func formatDuration(d time.Duration) string {
	if d < time.Minute {
		return fmt.Sprintf("%.0fs", d.Seconds())
	}
	if d < time.Hour {
		return fmt.Sprintf("%.0fm", d.Minutes())
	}
	if d < 24*time.Hour {
		hours := int(d.Hours())
		minutes := int(d.Minutes()) % 60
		return fmt.Sprintf("%dh%dm", hours, minutes)
	}
	days := int(d.Hours()) / 24
	hours := int(d.Hours()) % 24
	return fmt.Sprintf("%dd%dh", days, hours)
}
