package main

import (
	"bufio"
	"context"
	"flag"
	"fmt"
	"net"
	"os"
	"os/signal"
	"strings"
	"sync"
	"syscall"
	"text/tabwriter"
	"time"
)

type PingResult struct {
	Timestamp time.Time
	Host      string
	Success   bool
	Latency   time.Duration
}

func main() {
	hostsFile := flag.String("f", "", "Path to hosts file (required)")
	count := flag.Int("c", 0, "Number of checks per host for one-time mode")
	monitor := flag.Bool("monitor", false, "Run continuous monitoring mode")
	interval := flag.Int("interval", 5, "Monitoring interval in seconds")
	port := flag.Int("port", 80, "TCP port for checks")
	timeout := flag.Int("timeout", 2, "Dial timeout in seconds")
	logFile := flag.String("log", "monitor.log", "Log file path for monitor mode")
	flag.Parse()

	if *hostsFile == "" {
		exitWithUsage("flag -f is required")
	}
	if *monitor && *count > 0 {
		exitWithUsage("use either -monitor or -c, not both")
	}
	if !*monitor && *count <= 0 {
		exitWithUsage("for one-time mode use -c > 0, or pass -monitor")
	}
	if *interval <= 0 || *timeout <= 0 || *port <= 0 || *port > 65535 {
		exitWithUsage("invalid flags: interval/timeout > 0, port in 1..65535")
	}

	hosts, err := readHosts(*hostsFile)
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to read hosts: %v\n", err)
		os.Exit(1)
	}
	if len(hosts) == 0 {
		fmt.Fprintln(os.Stderr, "hosts file is empty")
		os.Exit(1)
	}

	dialTimeout := time.Duration(*timeout) * time.Second
	if *monitor {
		runMonitorMode(hosts, *interval, *port, dialTimeout, *logFile)
		return
	}
	runOneTimeMode(hosts, *count, *port, dialTimeout)
}

func exitWithUsage(msg string) {
	fmt.Fprintln(os.Stderr, "error:", msg)
	flag.Usage()
	os.Exit(2)
}

func readHosts(path string) ([]string, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	var hosts []string
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		hosts = append(hosts, line)
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}
	return hosts, nil
}

func runOneTimeMode(hosts []string, count, port int, timeout time.Duration) {
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, "Host\tStatus\tLatency\tAttempt")

	for _, host := range hosts {
		for attempt := 1; attempt <= count; attempt++ {
			success, latency, _ := pingHost(host, port, timeout)
			status, latencyText := formatResult(success, latency)
			fmt.Fprintf(w, "%s\t%s\t%s\t%d/%d\n", host, status, latencyText, attempt, count)

			if attempt < count {
				time.Sleep(1 * time.Second)
			}
		}
	}

	_ = w.Flush()
}

func runMonitorMode(hosts []string, intervalSec, port int, timeout time.Duration, logPath string) {
	logHandle, err := os.OpenFile(logPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to open log file: %v\n", err)
		os.Exit(1)
	}
	defer logHandle.Close()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	results := make(chan PingResult, len(hosts)*2)
	var workersWG sync.WaitGroup
	var listenerWG sync.WaitGroup

	listenerWG.Add(1)
	go func() {
		defer listenerWG.Done()
		for r := range results {
			printRealtimeResult(r)
			writeLogLine(logHandle, r)
		}
	}()

	interval := time.Duration(intervalSec) * time.Second
	for _, host := range hosts {
		workersWG.Add(1)
		go monitorWorker(ctx, &workersWG, host, port, timeout, interval, results)
	}

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	defer signal.Stop(sigChan)

	<-sigChan
	fmt.Println("\nSignal received, shutting down...")
	cancel()

	workersWG.Wait()
	close(results)
	listenerWG.Wait()
	fmt.Println("Monitor stopped gracefully.")
}

func monitorWorker(
	ctx context.Context,
	wg *sync.WaitGroup,
	host string,
	port int,
	timeout time.Duration,
	interval time.Duration,
	results chan<- PingResult,
) {
	defer wg.Done()

	send := func() {
		success, latency, _ := pingHost(host, port, timeout)
		result := PingResult{
			Timestamp: time.Now(),
			Host:      host,
			Success:   success,
			Latency:   latency,
		}

		select {
		case results <- result:
		case <-ctx.Done():
		}
	}

	send()
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			send()
		}
	}
}

func pingHost(host string, port int, timeout time.Duration) (success bool, latency time.Duration, err error) {
	address := net.JoinHostPort(host, fmt.Sprintf("%d", port))
	start := time.Now()
	conn, err := net.DialTimeout("tcp", address, timeout)
	if err != nil {
		return false, 0, err
	}
	latency = time.Since(start)
	_ = conn.Close()
	return true, latency, nil
}

func printRealtimeResult(r PingResult) {
	status, latencyText := formatResult(r.Success, r.Latency)
	fmt.Printf("%s | %s | %s | %s\n", r.Timestamp.Format("2006-01-02 15:04:05"), r.Host, status, latencyText)
}

func writeLogLine(file *os.File, r PingResult) {
	status, latencyText := formatResult(r.Success, r.Latency)
	line := fmt.Sprintf("%s | %s | %s | %s\n", r.Timestamp.Format("2006-01-02 15:04:05"), r.Host, status, latencyText)
	if _, err := file.WriteString(line); err != nil {
		fmt.Fprintf(os.Stderr, "failed to write log line: %v\n", err)
	}
}

func formatResult(success bool, latency time.Duration) (status, latencyText string) {
	if !success {
		return "Timeout", "0ms"
	}
	return "OK", fmt.Sprintf("%dms", latency.Milliseconds())
}
