package probe

import (
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"
)

type ProbeResult struct {
	StatusCode int
	LatencyMs  int64
	NetworkErr bool
}

// DoHTTPProbe menjalankan satu kali HTTP GET probe dan mengukur waktu.
func DoHTTPProbe(urlStr string) ProbeResult {
	startTime := time.Now()

	client := http.Client{
		Timeout: 5 * time.Second,
	}

	resp, err := client.Get(urlStr)

	duration := time.Since(startTime)
	milliseconds := duration.Milliseconds()

	if err != nil {
		return ProbeResult{
			StatusCode: 0,
			LatencyMs:  milliseconds,
			NetworkErr: true,
		}
	}
	defer resp.Body.Close()

	return ProbeResult{
		StatusCode: resp.StatusCode,
		LatencyMs:  milliseconds,
		NetworkErr: false,
	}
}

// DoTCPPing attempts to open a TCP connection to a host:port
func DoTCPPing(rawURL string) ProbeResult {
	startTime := time.Now()

	parsedURL, err := url.Parse(rawURL)
	targetHost := rawURL
	if err == nil && parsedURL.Host != "" {
		targetHost = parsedURL.Host
		// if no port, add one based on scheme
		if !strings.Contains(targetHost, ":") {
			if parsedURL.Scheme == "https" {
				targetHost = targetHost + ":443"
			} else {
				targetHost = targetHost + ":80"
			}
		}
	}

	conn, err := net.DialTimeout("tcp", targetHost, 5*time.Second)
	duration := time.Since(startTime)
	milliseconds := duration.Milliseconds()

	if err != nil {
		return ProbeResult{
			StatusCode: 0,
			LatencyMs:  milliseconds,
			NetworkErr: true,
		}
	}
	conn.Close()

	return ProbeResult{
		StatusCode: 200, 
		LatencyMs:  milliseconds,
		NetworkErr: false,
	}
}

// DoICMPProbe is an ICMP-like probe.
// For now, it performs a TCP dial to a common port as a proxy for reachability.
// This is NOT a real ICMP ping, which is more complex and may require elevated privileges.
func DoICMPProbe(rawURL string) ProbeResult {
	// Using DoTCPPing as a stand-in for an ICMP-like check.
	return DoTCPPing(rawURL)
}
