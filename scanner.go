package main

import (
	"flag"
	"fmt"
	"net"
	"os"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
)

// Terminal color codes used for output
const (
	Reset = "\033[0m"
	Green = "\033[32m"
)

func parsePorts(portArg string) ([]int, error) {
	var ports []int

	if portArg == "" {
		for i := 1; i <= 65535; i++ { // Default: scan all ports
			ports = append(ports, i)
		}
		return ports, nil
	}

	parts := strings.Split(portArg, ",") // Split input string on commas
	for _, part := range parts {

		if strings.Contains(part, "-") { // If port range (e.g., 20-80)
			bounds := strings.Split(part, "-")
			if len(bounds) != 2 {
				return nil, fmt.Errorf("invalid range: %s", part)
			}
			start, err1 := strconv.Atoi(bounds[0])
			end, err2 := strconv.Atoi(bounds[1])
			if err1 != nil || err2 != nil || start > end {
				return nil, fmt.Errorf("invalid range: %s", part)
			}
			for i := start; i <= end; i++ {
				ports = append(ports, i)
			}
		} else {

			port, err := strconv.Atoi(part) // If single port (e.g., 443)
			if err != nil {
				return nil, fmt.Errorf("invalid port: %s", part)
			}
			ports = append(ports, port)
		}
	}
	return ports, nil
}

func ScanPort(host string, portsToScan <-chan int, openPorts chan<- int, wg *sync.WaitGroup, timeout time.Duration) {
	defer wg.Done()

	for port := range portsToScan {
		address := fmt.Sprintf("%s:%d", host, port)
		conn, err := net.DialTimeout("tcp", address, timeout)
		if err == nil {
			openPorts <- port
		}
		if conn != nil { //Safely close connection only if successfully created
			conn.Close()
		}
	}
}

func main() {
	var host string
	var portArg string
	var timeoutMs int

	flag.StringVar(&host, "host", "", "Target host to scan")
	flag.StringVar(&portArg, "p", "", "Ports to scan (e.g. 80,443 or 20-100). Defaults to all ports.")
	flag.IntVar(&timeoutMs, "timeout", 500, "Timeout in milliseconds for each port scan")
	flag.Parse()

	if host == "" {
		fmt.Println("Error: You must specify a host to scan using the -host flag.")
		os.Exit(1)
	}

	// Parse port argument
	ports, err := parsePorts(portArg)
	if err != nil {
		fmt.Println("Error parsing ports:", err)
		os.Exit(1)
	}

	openPorts := make(chan int, len(ports))   // Channel to collect open ports
	portsToScan := make(chan int, len(ports)) // Channel to distribute ports to workers
	var wg sync.WaitGroup

	timeout := time.Duration(timeoutMs) * time.Millisecond
	numWorkers := min(len(ports), runtime.NumCPU()*20)

	for i := 0; i < numWorkers; i++ { // Start worker goroutines
		wg.Add(1)
		go ScanPort(host, portsToScan, openPorts, &wg, timeout)
	}

	// Send ports to scan into the channel
	for _, port := range ports {
		portsToScan <- port
	}
	close(portsToScan) //Close 'portsToScan' chan when all specified ports have been sent

	wg.Wait()
	close(openPorts) // Close 'openPorts' chan when scan is finished.

	var foundPorts []int
	for port := range openPorts {
		foundPorts = append(foundPorts, port)
	}
	sort.Ints(foundPorts)

	for _, port := range foundPorts {
		fmt.Printf("%sPort %d is open%s\n", Green, port, Reset)
	}

	fmt.Printf("\nScan complete. %d open ports found.\n", len(foundPorts))
}
