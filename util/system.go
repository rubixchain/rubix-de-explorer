package util

import (
	"bufio"
	"os"
	"runtime"
	"strconv"
	"strings"
)

// GetSystemMemoryUsage returns the system's active memory usage percentage.
func GetSystemMemoryUsage() float64 {
	if runtime.GOOS == "linux" {
		file, err := os.Open("/proc/meminfo")
		if err == nil {
			defer file.Close()
			scanner := bufio.NewScanner(file)
			var memTotal, memAvailable float64
			for scanner.Scan() {
				line := scanner.Text()
				if strings.HasPrefix(line, "MemTotal:") {
					fields := strings.Fields(line)
					if len(fields) >= 2 {
						if val, err := strconv.ParseFloat(fields[1], 64); err == nil {
							memTotal = val
						}
					}
				} else if strings.HasPrefix(line, "MemAvailable:") {
					fields := strings.Fields(line)
					if len(fields) >= 2 {
						if val, err := strconv.ParseFloat(fields[1], 64); err == nil {
							memAvailable = val
						}
					}
				}
				if memTotal > 0 && memAvailable > 0 {
					break
				}
			}
			if memTotal > 0 && memAvailable > 0 {
				used := memTotal - memAvailable
				return (used / memTotal) * 100.0
			}
		}
	}

	// Fallback for non-Linux (local testing)
	var m runtime.MemStats
	runtime.ReadMemStats(&m)
	usedMB := float64(m.Alloc) / 1024 / 1024
	sysMB := float64(m.Sys) / 1024 / 1024
	if sysMB > 0 {
		return (usedMB / sysMB) * 100.0
	}
	return 0.0
}
