package sysinfo

import (
	"testing"
)

func TestGetTelemetry(t *testing.T) {
	data, err := GetTelemetry()
	if err != nil {
		t.Fatalf("GetTelemetry failed: %v", err)
	}

	if data.RAMTotal == 0 {
		t.Error("RAMTotal should not be 0")
	}

	if data.CPUUsage < 0 || data.CPUUsage > 100 {
		t.Errorf("Invalid CPUUsage: %f", data.CPUUsage)
	}

	if data.DiskTotal == 0 {
		t.Error("DiskTotal should not be 0")
	}

	t.Logf("Detected CPU Temp: %.2fC", data.CPUTemp)
}
