package node

import (
	"net"
	"strconv"
)

// OutboundIP returns the preferred outbound IPv4 address of this host. It does
// not actually send packets; dialing a UDP socket just selects a source IP.
// Falls back to 127.0.0.1 if nothing better is found.
func OutboundIP() string {
	conn, err := net.Dial("udp4", "8.8.8.8:80")
	if err != nil {
		return "127.0.0.1"
	}
	defer conn.Close()
	if addr, ok := conn.LocalAddr().(*net.UDPAddr); ok {
		return addr.IP.String()
	}
	return "127.0.0.1"
}

// PortOf extracts the numeric port from a host:port string, or 0 on failure.
func PortOf(hostport string) int {
	_, portStr, err := net.SplitHostPort(hostport)
	if err != nil {
		return 0
	}
	p, err := strconv.Atoi(portStr)
	if err != nil {
		return 0
	}
	return p
}
