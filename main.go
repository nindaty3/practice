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
	hostsFile := flag.String("f", "", "Hosts file")
	count := flag.Int("c", 3, "Checks")
	monitor := flag.Bool("monitor", false, "Monitor")
	flag.Parse()

	if *hostsFile == "" {
		fmt.Println("Need -f")
		os.Exit(1)
	}

	hosts, err := readHosts(*hostsFile)
	if err != nil {
		fmt.Println("Error:", err)
		os.Exit(1)
	}

	if *monitor {
		runMonitor(hosts)
	} else {
		runOneTime(hosts, *count)
	}
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
		if line != "" && !strings.HasPrefix(line, "#") {
			hosts = append(hosts, line)
		}
	}
	return hosts, scanner.Err()
}

func runOneTime(hosts []string, count int) {
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, "Host\tStatus\tLatency\tAttempt")

	for _, host := range hosts {
		for i := 1; i <= count; i++ {
			success, latency := pingHost(host)
			status, latencyText := formatResult(success, latency)
			fmt.Fprintf(w, "%s\t%s\t%s\t%d/%d\n", host, status, latencyText, i, count)
			if i < count {
				time.Sleep(1 * time.Second)
			}
		}
	}
	w.Flush()
}

func runMonitor(hosts []string) {
	log, err := os.OpenFile("monitor.log", os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0644)
	if err != nil {
		fmt.Println("Log error:", err)
		os.Exit(1)
	}
	defer log.Close()

	ctx, cancel := context.WithCancel(context.Background())
	results := make(chan PingResult, 10)
	var wg sync.WaitGroup

	go func() {
		for r := range results {
			printResult(r)
			writeLog(log, r)
		}
	}()

	for _, host := range hosts {
		wg.Add(1)
		go monitorHost(ctx, &wg, host, results)
	}

	sig := make(chan os.Signal, 1)
	signal.Notify(sig, syscall.SIGINT)
	<-sig
	cancel()
	wg.Wait()
	close(results)
}

func monitorHost(ctx context.Context, wg *sync.WaitGroup, host string, results chan<- PingResult) {
	defer wg.Done()
	send := func() {
		success, latency := pingHost(host)
		results <- PingResult{time.Now(), host, success, latency}
	}
	send()
	ticker := time.NewTicker(5 * time.Second)
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

func pingHost(host string) (bool, time.Duration) {
	start := time.Now()
	conn, err := net.DialTimeout("tcp", host+":80", 2*time.Second)
	if err != nil {
		return false, 0
	}
	conn.Close()
	return true, time.Since(start)
}

func formatResult(success bool, latency time.Duration) (string, string) {
	if !success {
		return "Timeout", "0ms"
	}
	return "OK", fmt.Sprintf("%dms", latency.Milliseconds())
}

func printResult(r PingResult) {
	status, latencyText := formatResult(r.Success, r.Latency)
	fmt.Printf("%s | %s | %s | %s\n", r.Timestamp.Format("2006-01-02 15:04:05"), r.Host, status, latencyText)
}

func writeLog(file *os.File, r PingResult) {
	status, latencyText := formatResult(r.Success, r.Latency)
	line := fmt.Sprintf("%s | %s | %s | %s\n", r.Timestamp.Format("2006-01-02 15:04:05"), r.Host, status, latencyText)
	file.WriteString(line)
}
