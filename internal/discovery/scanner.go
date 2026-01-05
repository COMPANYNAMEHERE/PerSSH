package discovery

import (
	"fmt"
	"net"
	"strings"
	"sync"
	"time"
)

// ScanResult holds the IP and status of a scanned host.
type ScanResult struct {
	IP   string
	Open bool
}

// GetLocalSubnet attempts to find the local subnet (e.g., "192.168.1").
// This is a naive implementation; production code should handle multiple interfaces better.
func GetLocalSubnet() (string, error) {
	addrs, err := net.InterfaceAddrs()
	if err != nil {
		return "", err
	}

	for _, addr := range addrs {
		if ipnet, ok := addr.(*net.IPNet); ok && !ipnet.IP.IsLoopback() {
			if ipnet.IP.To4() != nil {
				// Assumes /24 for simplicity
				ip := ipnet.IP.String()
				parts := strings.Split(ip, ".")
				if len(parts) == 4 {
					return strings.Join(parts[:3], "."), nil
				}
			}
		}
	}
	return "", fmt.Errorf("no local IP found")
}

// ScanSubnet scans a given subnet (e.g., "192.168.1") for a specific port.
// It returns a list of IPs that have the port open.
func ScanSubnet(subnet string, port int, timeout time.Duration) []string {
	var results []string
	var mutex sync.Mutex
	var wg sync.WaitGroup

	// Limit concurrency to avoid file descriptor limits
	sem := make(chan struct{}, 50) 

	for i := 1; i < 255; i++ {
		ip := fmt.Sprintf("%s.%d", subnet, i)
		wg.Add(1)
		
		go func(targetIP string) {
			defer wg.Done()
			sem <- struct{}{} // Acquire token
			defer func() { <-sem }() // Release token

			conn, err := net.DialTimeout("tcp", fmt.Sprintf("%s:%d", targetIP, port), timeout)
			if err == nil {
				conn.Close()
				mutex.Lock()
				results = append(results, targetIP)
				mutex.Unlock()
			}
		}(ip)
	}

	wg.Wait()
	return results
}
