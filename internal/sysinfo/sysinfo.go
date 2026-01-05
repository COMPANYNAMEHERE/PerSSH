package sysinfo

import (
	"fmt"
	"math"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/shirou/gopsutil/v3/cpu"
	"github.com/shirou/gopsutil/v3/disk"
	"github.com/shirou/gopsutil/v3/host"
	"github.com/shirou/gopsutil/v3/mem"
	"github.com/COMPANYNAMEHERE/PerSSH/internal/common"
)

// GetTelemetry gathers current system stats.
func GetTelemetry() (*common.TelemetryData, error) {
	v, err := mem.VirtualMemory()
	if err != nil {
		return nil, err
	}

	c, err := cpu.Percent(0, false)
	if err != nil {
		return nil, err
	}
	cpuVal := 0.0
	if len(c) > 0 {
		cpuVal = c[0]
	}

	d, err := disk.Usage("/")
	if err != nil {
		return nil, err
	}

	// Try to get temperature
	tempVal := 0.0
	temps, err := host.SensorsTemperatures()
	if err == nil {
		// Common sensor names for Intel/AMD
		priorityKeys := []string{
			"coretemp_package_id_0",
			"coretemp_core_0",
			"k10temp_tctl",
			"k10temp_tdie",
			"cpu_thermal", // Raspberry Pi
		}

		found := false
		for _, key := range priorityKeys {
			for _, t := range temps {
				if t.SensorKey == key {
					tempVal = t.Temperature
					found = true
					break
				}
			}
			if found {
				break
			}
		}
	}

	// Fallback 2: Manual scan of hwmon if gopsutil failed or returned 0
	if tempVal == 0 {
		for i := 0; i < 10; i++ {
			path := fmt.Sprintf("/sys/class/hwmon/hwmon%d", i)
			if _, err := os.Stat(path); err != nil {
				continue
			}

			nameB, _ := os.ReadFile(filepath.Join(path, "name"))
			name := string(nameB)

			// Priority for CPU sensors
			if strings.Contains(name, "coretemp") || strings.Contains(name, "k10temp") || strings.Contains(name, "acpitz") {
				tempB, err := os.ReadFile(filepath.Join(path, "temp1_input"))
				if err == nil {
					fmt.Sscanf(string(tempB), "%f", &tempVal)
					tempVal /= 1000.0
					break
				}
			}
		}
	}

	return &common.TelemetryData{
		Timestamp: time.Now(),
		CPUUsage:  math.Round(cpuVal*100) / 100,
		CPUTemp:   tempVal,
		RAMUsage:  math.Round(v.UsedPercent*100) / 100,
		RAMTotal:  v.Total,
		RAMUsed:   v.Used,
		DiskFree:  d.Free,
		DiskTotal: d.Total,
	}, nil
}
