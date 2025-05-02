package main

import (
	"airplayScanner/updater"
	"encoding/csv"
	"errors"
	"flag"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/fatih/color"
	"howett.net/plist"
)

const (
	companyName     = "@nma-io"
	copyright       = "(c) Nicholas Albright"
	fileDescription = "Airplay Device Hunter"
	version         = "2025.2.1"
	internalName    = "airplay-scanner"
	productName     = "airplay-scanner"
)

type AirPlayDevice struct {
	Name            string
	Model           string
	IP              string
	MacAddress      string
	DeviceID        string
	Features        string
	StatusFlags     string
	SourceVersion   string
	ProtocolVersion string
}

func parsePlist(data []byte, ip string) (AirPlayDevice, error) {
	var m map[string]interface{}
	_, err := plist.Unmarshal(data, &m)
	if err != nil {
		return AirPlayDevice{}, err
	}

	getString := func(key string) string {
		if v, ok := m[key]; ok {
			return fmt.Sprintf("%v", v)
		}
		return ""
	}

	return AirPlayDevice{
		Name:            getString("name"),
		Model:           getString("model"),
		IP:              ip,
		MacAddress:      getString("macAddress"),
		DeviceID:        getString("deviceID"),
		Features:        getString("features"),
		StatusFlags:     getString("statusFlags"),
		SourceVersion:   getString("sourceVersion"),
		ProtocolVersion: getString("protocolVersion"),
	}, nil
}

func scanPort(host string, timeout time.Duration) bool {
	conn, err := net.DialTimeout("tcp", net.JoinHostPort(host, "7000"), timeout)
	if err != nil {
		return false
	}
	_ = conn.Close()
	return true
}

func fetchInfo(host string, timeout time.Duration) ([]byte, error) {
	url := fmt.Sprintf("http://%s:7000/info", host)
	client := &http.Client{Timeout: timeout}
	resp, err := client.Get(url)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return nil, errors.New("non-200 response")
	}
	return io.ReadAll(resp.Body)
}

func parseCIDRorIP(input string) ([]string, error) {
	if strings.Contains(input, "/") {
		ip, ipnet, err := net.ParseCIDR(input)
		if err != nil {
			return nil, err
		}
		var ips []string
		for ip := ip.Mask(ipnet.Mask); ipnet.Contains(ip); incIP(ip) {
			ips = append(ips, ip.String())
		}
		// Remove network and broadcast addresses
		if len(ips) > 2 {
			return ips[1 : len(ips)-1], nil
		}
		return ips, nil
	}
	parsed := net.ParseIP(input)
	if parsed == nil {
		return nil, fmt.Errorf("invalid IP: %s", input)
	}
	return []string{input}, nil
}

func incIP(ip net.IP) {
	for j := len(ip) - 1; j >= 0; j-- {
		ip[j]++
		if ip[j] != 0 {
			break
		}
	}
}

func outputCSV(filename string, devices []AirPlayDevice) error {
	f, err := os.Create(filename)
	if err != nil {
		return err
	}
	defer f.Close()
	w := csv.NewWriter(f)
	defer w.Flush()
	w.Write([]string{"name", "model", "ip", "macAddress", "deviceID", "features", "statusFlags", "sourceVersion", "protocolVersion"})
	for _, d := range devices {
		w.Write([]string{d.Name, d.Model, d.IP, d.MacAddress, d.DeviceID, d.Features, d.StatusFlags, d.SourceVersion, d.ProtocolVersion})
	}
	return w.Error()
}

func printDevice(d AirPlayDevice, useColor bool) {
	if useColor {
		color.New(color.FgGreen, color.Bold).Printf("[+] Found: ")
		color.New(color.FgCyan).Printf("%s ", d.Name)
		color.New(color.FgYellow).Printf("(%s) ", d.Model)
		color.New(color.FgHiWhite).Printf("%s ", d.IP)
		color.New(color.FgMagenta).Printf("MAC: %s ", d.MacAddress)
		color.New(color.FgHiBlue).Printf("Features: %s ", d.Features)
		color.New(color.FgHiBlack).Printf("Status: %s ", d.StatusFlags)
		fmt.Println()
	} else {
		fmt.Printf("[+] Found: %s (%s) %s MAC: %s Features: %s Status: %s\n", d.Name, d.Model, d.IP, d.MacAddress, d.Features, d.StatusFlags)
	}
}

func main() {
	fmt.Printf("AirPlay Scanner v%s - %s, %s\n", version, companyName, copyright)
	outputFile := flag.String("o", "", "Output file (CSV, optional)")
	noColor := flag.Bool("no-color", false, "Disable color output")
	workers := flag.Int("workers", 64, "Number of concurrent workers")
	updateFlag := flag.Bool("update", false, "Check for updates")
	flag.Parse()

	ipRange := flag.Arg(0)
	if ipRange == "" {
		flag.PrintDefaults()
		fmt.Printf("Usage: %s -o <output_file> <ip_range>\n", os.Args[0])
		os.Exit(1)
	}

	if *updateFlag {
		fmt.Println("Checking for updates...")
		if err := updater.Update(version); err != nil {
			fmt.Printf("Error updating: %v\n", err)
			os.Exit(1)
		}
		fmt.Println("Update completed successfully")
		os.Exit(0)
	}

	ips, err := parseCIDRorIP(ipRange)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Invalid IP or range: %v\n", err)
		os.Exit(1)
	}

	var (
		resultsMu sync.Mutex
		results   []AirPlayDevice
	)
	sem := make(chan struct{}, *workers)
	wg := sync.WaitGroup{}

	for _, ip := range ips {
		wg.Add(1)
		sem <- struct{}{}
		go func(ip string) {
			defer wg.Done()
			if !scanPort(ip, 500*time.Millisecond) {
				<-sem
				return
			}
			data, err := fetchInfo(ip, 2*time.Second)
			if err != nil {
				<-sem
				return
			}
			dev, err := parsePlist(data, ip)
			if err != nil {
				<-sem
				return
			}
			resultsMu.Lock()
			results = append(results, dev)
			resultsMu.Unlock()
			if *outputFile == "" {
				printDevice(dev, !*noColor)
			}
			<-sem
		}(ip)
	}
	wg.Wait()

	if *outputFile != "" {
		err := outputCSV(*outputFile, results)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Failed to write output: %v\n", err)
			os.Exit(1)
		}
		fmt.Printf("Wrote %d results to %s\n", len(results), *outputFile)
	} else if len(results) == 0 {
		fmt.Println("No AirPlay devices found.")
	}
}
