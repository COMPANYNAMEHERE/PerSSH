package sysinfo

import (
	"math"
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
		for _, t := range temps {
			// Basic heuristic: find first core temp or package id 0
			if t.SensorKey == "coretemp_package_id_0" || t.SensorKey == "k10temp_tctl" {
				tempVal = t.Temperature
				break
			}
			// Fallback: take the first positive value if nothing else matches
			if tempVal == 0 && t.Temperature > 0 {
				tempVal = t.Temperature
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
