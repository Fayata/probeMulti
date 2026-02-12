package probe

import (
	"net"
	"net/url"
	"strings"
	"time"

	probing "github.com/prometheus-community/pro-bing"
)

// ProbeResult menyimpan hasil pengecekan
type ProbeResult struct {
	StatusCode int   // Kita isi 200 jika Reply, 0 jika Timeout
	LatencyMs  int64 // Lama waktu ping dalam ms
	NetworkErr bool  // True jika gagal ping
}

// DoProbe menjalankan ICMP Ping ke target
func DoProbe(target string) ProbeResult {
	// 1. BERSIHKAN INPUT (Hapus http://, https://, dan port)
	// Contoh: "http://192.168.1.1:8080" -> "192.168.1.1"
	cleanHost := target

	// Hapus scheme (http://)
	if strings.Contains(target, "://") {
		u, err := url.Parse(target)
		if err == nil {
			cleanHost = u.Hostname()
		}
	}

	// Hapus port manual (misal user ketik 1.1.1.1:80)
	// Kita cek ada ':' tapi bukan IPv6 (kurung siku)
	if strings.Contains(cleanHost, ":") && !strings.HasPrefix(cleanHost, "[") {
		host, _, err := net.SplitHostPort(cleanHost)
		if err == nil {
			cleanHost = host
		}
	}

	// 2. SETUP PINGER
	// Menggunakan library pro-bing
	pinger, err := probing.NewPinger(cleanHost)
	if err != nil {
		// Error biasanya karena DNS tidak ketemu atau format IP salah
		return ProbeResult{StatusCode: 0, LatencyMs: 0, NetworkErr: true}
	}

	pinger.Count = 1                 // Cukup ping 1 kali
	pinger.Timeout = 2 * time.Second // Waktu tunggu maksimal 2 detik

	// PENTING: SetPrivileged(true) wajib untuk Windows & Linux (biar lancar tanpa root khusus)
	pinger.SetPrivileged(true)

	// 3. JALANKAN PING
	err = pinger.Run()
	if err != nil {
		return ProbeResult{StatusCode: 0, LatencyMs: 0, NetworkErr: true}
	}

	stats := pinger.Statistics()

	// 4. HASIL
	if stats.PacketsRecv > 0 {
		// Jika ada balasan, kita anggap statusnya 200 (OK)
		return ProbeResult{
			StatusCode: 200,
			LatencyMs:  stats.AvgRtt.Milliseconds(),
			NetworkErr: false,
		}
	}

	// Jika tidak ada balasan (RTO)
	return ProbeResult{
		StatusCode: 0,
		LatencyMs:  0,
		NetworkErr: true,
	}
}
