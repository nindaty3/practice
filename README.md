# srvchk
A lightweight CLI tool for checking server availability over TCP. It tests connectivity to a list of hosts (port 80) and reports whether each host is reachable, along with connection latency.

Supports two modes:

One-time batch check
Continuous monitoring mode with live updates and logging
Run
Build
go build -o srvchk
One-time check
./srvchk -f hosts.txt -c 3
Monitor mode
./srvchk -f hosts.txt -monitor
Hosts file

One host per line:

example.com
google.com
127.0.0.1

Lines starting with # are ignored.
