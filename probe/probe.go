package probe

import (
	"net"
	"net/http"
	"net/url"
	"os"
	"runtime"
	"strings"
	"time"

	"golang.org/x/net/icmp"
	"golang.org/x/net/ipv4"
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
func DoICMPProbe(target string) ProbeResult {
	startTime := time.Now()

	// 1. Resolve alamat IP (Bisa input IP langsung atau Domain)
	ip, err := net.ResolveIPAddr("ip4", target)
	if err != nil {
		return ProbeResult{StatusCode: 0, LatencyMs: 0, NetworkErr: true}
	}

	// 2. Listen ICMP (Membutuhkan akses Privileged/Root di Linux)
	// Gunakan "udp4" jika ingin mencoba tanpa root (unprivileged),
	// tapi "ip4:icmp" lebih akurat untuk real ping.
	network := "ip4:icmp"
	if runtime.GOOS == "darwin" || runtime.GOOS == "windows" {
		network = "udp4"
	}

	conn, err := icmp.ListenPacket(network, "0.0.0.0")
	if err != nil {
		return ProbeResult{StatusCode: 0, LatencyMs: 0, NetworkErr: true}
	}
	defer conn.Close()

	// 3. Buat Pesan ICMP Echo Request
	msg := icmp.Message{
		Type: ipv4.ICMPTypeEcho, Code: 0,
		Body: &icmp.Echo{
			ID: os.Getpid() & 0xffff, Seq: 1,
			Data: []byte("HELLO-PROBE"),
		},
	}
	msgBytes, _ := msg.Marshal(nil)

	// 4. Kirim dan Set Timeout
	conn.SetDeadline(time.Now().Add(5 * time.Second))
	if _, err := conn.WriteTo(msgBytes, ip); err != nil {
		return ProbeResult{StatusCode: 0, LatencyMs: 0, NetworkErr: true}
	}

	// 5. Baca Balasan
	reply := make([]byte, 1500)
	n, _, err := conn.ReadFrom(reply)
	duration := time.Since(startTime).Milliseconds()

	if err != nil {
		return ProbeResult{StatusCode: 0, LatencyMs: duration, NetworkErr: true}
	}

	// 6. Validasi apakah itu Echo Reply
	rm, err := icmp.ParseMessage(1, reply[:n])
	if err == nil && rm.Type == ipv4.ICMPTypeEchoReply {
		return ProbeResult{StatusCode: 200, LatencyMs: duration, NetworkErr: false}
	}

	return ProbeResult{StatusCode: 0, LatencyMs: duration, NetworkErr: true}

}
