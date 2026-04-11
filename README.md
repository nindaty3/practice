# srvchk
This is a lightweight command-line utility written in Go for checking server availability and monitoring host uptime over time. It performs TCP-based reachability checks and supports both one-time batch checks and continuous monitoring mode.

Features
Reads a list of hosts from a file
Performs TCP connectivity checks (port 80)
Configurable number of check attempts per host
Two operational modes:
One-time scan mode
Continuous monitoring mode
Real-time output to console
Persistent logging to file in monitoring mode
Graceful shutdown via SIGINT (Ctrl+C)
Concurrency support for monitoring multiple hosts simultaneously
How It Works

The tool attempts to establish a TCP connection to each host on port 80 using a timeout-based dial. A successful connection is treated as an "online" status, while failures are considered "timeout" or "offline".

Latency is measured as the time required to establish the connection.

Usage
Build
go build -o srvchk
Run one-time check
./srvchk -f hosts.txt -c 3
Run in monitoring mode
./srvchk -f hosts.txt -monitor
Flags
-f
Path to a file containing a list of hosts (one per line)
-c
Number of check attempts per host (default: 3)
-monitor
Enables continuous monitoring mode with periodic checks
Hosts File Format

Each line should contain a host in the following format:

example.com
google.com
127.0.0.1

Lines starting with # are treated as comments and ignored.

Output
One-time mode

Displays a table with:

Host
Status (OK / Timeout)
Latency
Attempt number
Monitoring mode
Prints real-time status updates
Writes logs to monitor.log in the following format:
YYYY-MM-DD HH:MM:SS | host | status | latency
Architecture Notes
Uses goroutines for concurrent host monitoring
Context-based cancellation for graceful shutdown
Channel-based pipeline for result processing
Buffered logging system for reduced I/O overhead
Limitations
Only TCP port 80 is checked (no ICMP/ping support)
No DNS resolution diagnostics
No retry backoff strategy
Fixed monitoring interval (5 seconds)
Possible Improvements
Configurable ports per host
ICMP ping support
Exponential backoff on failures
JSON or structured logging output
Metrics export (Prometheus support)
HTTP API mode
