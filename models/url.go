package models

import (
	"database/sql"
	"fmt"
	"html/template"
	"time"
)

type TargetURL struct {
	ID              int
	URL             string
	LastStatus      int
	LastLatencyMs   int64
	LastChecked     time.Time
	IsUp            bool
	FirstUpTime     sql.NullTime
	TotalProbeCount int64
	TotalLatencySum int64
	ProbeMode       string
}

type ProbeHistory struct {
	URLID     int
	URL       string
	LatencyMs int64
	Timestamp time.Time
}

type PageData struct {
	Page             string
	URLs             []TargetURL
	CurrentInterval  string
	GlobalAvgLatency int64
	GlobalUptimePct  int
	LastCheckedTime  time.Time
	HistoryData      []ProbeHistory
	SelectedURLID    int
	PageNumber       int
	PageSize         int
	TotalItems       int64
	TotalPages       int
	HasPrev          bool
	HasNext          bool
	PrevPage         int
	NextPage         int
	ChartRange       string
	NavigatorPages   []int
	JSONHistoryData  template.JS
}

// === FUNGSI HELPER UNTUK TEMPLATE ===
func (tu *TargetURL) GetUptime() string {
	if !tu.FirstUpTime.Valid {
		return "N/A"
	}
	duration := time.Since(tu.FirstUpTime.Time)

	days := int(duration.Hours() / 24)
	hours := int(duration.Hours()) % 24
	minutes := int(duration.Minutes()) % 60

	if days > 0 {
		return fmt.Sprintf("%d day, %d hour", days, hours)
	}
	if hours > 0 {
		return fmt.Sprintf("%d hour, %d minute", hours, minutes)
	}
	if minutes > 0 {
		return fmt.Sprintf("%d minute", minutes)
	}
	if duration.Seconds() < 60 {
		return "Just now"
	}
	return "N/A"
}

// GetAverageLatency menghitung rata-rata latency (sebagai string)
func (tu *TargetURL) GetAverageLatency() string {
	if tu.TotalProbeCount == 0 {
		return "N/A"
	}
	avg := tu.TotalLatencySum / tu.TotalProbeCount
	return fmt.Sprintf("%d ms", avg)
}
