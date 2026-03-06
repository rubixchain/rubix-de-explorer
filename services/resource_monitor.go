package services

import (
	"runtime"
)

// ResourceMonitor provides light-weight, cross-platform system resource statistics.
// It is used by the DynamicTxnProcessor to make scaling decisions.
type ResourceMonitor struct{}

// GetResourceStats returns the current memory usage and goroutine count.
// Since full cross-platform CPU monitoring requires external CGO/system packages
// (which we want to avoid for the Explorer build), we rely primarily on memory pressure
// and queue length to drive scaling decisions.
func (rm *ResourceMonitor) GetResourceStats() map[string]interface{} {
	var m runtime.MemStats
	runtime.ReadMemStats(&m)

	// Convert to MB for easier reading
	usedMB := float64(m.Alloc) / 1024 / 1024
	sysMB := float64(m.Sys) / 1024 / 1024

	// Estimate memory usage percent based on what the OS has given the process vs what is active
	var memoryUsagePct float64
	if sysMB > 0 {
		memoryUsagePct = (usedMB / sysMB) * 100.0
	}

	return map[string]interface{}{
		"memory_total_mb":     sysMB,
		"memory_used_mb":      usedMB,
		"memory_available_mb": sysMB - usedMB,
		"memory_usage_pct":    memoryUsagePct,
		"cpu_count":           runtime.NumCPU(),
		"goroutines":          runtime.NumGoroutine(),
	}
}
