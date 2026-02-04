package handler

import (
	"encoding/json"
	"html/template"
	"log"
	"net/http"
	"strconv"
	"strings"
	"test/database"
	"test/models"
	"test/scheduler"
	"time"

	"github.com/gorilla/mux"
	"github.com/robfig/cron/v3"
)

type Application struct {
	Store     *database.Store
	Templates *template.Template
	Scheduler *cron.Cron
	JobID     cron.EntryID
}

type Handlers struct {
	App *Application
}

func NewHandlers(app *Application) *Handlers {
	return &Handlers{App: app}
}

// SchedulerHistoryAPI mengembalikan history terbaru (untuk animasi row baru di tabel)
func (h *Handlers) SchedulerHistoryAPI(w http.ResponseWriter, r *http.Request) {
	limit := 30
	if v := r.URL.Query().Get("limit"); v != "" {
		if n, convErr := strconv.Atoi(v); convErr == nil && n > 0 && n <= 200 {
			limit = n
		}
	}

	qrange := r.URL.Query().Get("range")
	now := time.Now()
	var since time.Time
	switch qrange {
	case "1s":
		since = now.Add(-1 * time.Second)
	case "10s":
		since = now.Add(-10 * time.Second)
	case "30s":
		since = now.Add(-30 * time.Second)
	case "1min":
		since = now.Add(-1 * time.Minute)
	case "1h":
		since = now.Add(-1 * time.Hour)
	case "4h":
		since = now.Add(-4 * time.Hour)
	case "1d":
		since = now.Add(-24 * time.Hour)
	case "1w":
		since = now.Add(-7 * 24 * time.Hour)
	case "1m":
		since = now.Add(-30 * 24 * time.Hour)
	}

	var (
		history []models.ProbeHistory
		err     error
	)
	if !since.IsZero() {
		history, err = h.App.Store.GetAllProbeHistoryByRangePaged(since, limit, 0)
	} else {
		history, err = h.App.Store.GetAllProbeHistory(limit)
	}
	if err != nil {
		log.Printf("SchedulerHistoryAPI: %v", err)
		http.Error(w, `{"error":"failed to get history"}`, http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(history)
}

// URLsAPI mengembalikan data URL terbaru untuk update real-time halaman /urls
func (h *Handlers) URLsAPI(w http.ResponseWriter, r *http.Request) {
	urls, err := h.App.Store.GetAllURLs()
	if err != nil {
		log.Printf("URLsAPI: %v", err)
		http.Error(w, `{"error":"failed to get urls"}`, http.StatusInternalServerError)
		return
	}

	type urlDTO struct {
		ID              int       `json:"ID"`
		URL             string    `json:"URL"`
		ProbeMode       string    `json:"ProbeMode"`
		LastStatus      int       `json:"LastStatus"`
		LastLatencyMs   int64     `json:"LastLatencyMs"`
		LastChecked     time.Time `json:"LastChecked"`
		IsUp            bool      `json:"IsUp"`
		TotalProbeCount int64     `json:"TotalProbeCount"`
		TotalLatencySum int64     `json:"TotalLatencySum"`
		Uptime          string    `json:"Uptime"`
	}

	out := make([]urlDTO, 0, len(urls))
	for i := range urls {
		u := urls[i]
		out = append(out, urlDTO{
			ID:              u.ID,
			URL:             u.URL,
			ProbeMode:       u.ProbeMode,
			LastStatus:      u.LastStatus,
			LastLatencyMs:   u.LastLatencyMs,
			LastChecked:     u.LastChecked,
			IsUp:            u.IsUp,
			TotalProbeCount: u.TotalProbeCount,
			TotalLatencySum: u.TotalLatencySum,
			Uptime:          u.GetUptime(),
		})
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(out)
}

// === HANDLER HALAMAN ===

// DashboardPage menangani halaman utama ('/')
func (h *Handlers) DashboardPage(w http.ResponseWriter, r *http.Request) {
	// HANYA proses request untuk path root '/'
	if r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}

	urls, err := h.App.Store.GetAllURLs()
	if err != nil {
		log.Printf("Gagal mengambil URL: %v", err)
		http.Error(w, "Gagal mengambil data", http.StatusInternalServerError)
		return
	}

	// Ambil URL yang dipilih dari query param
	selectedURLIDStr := r.URL.Query().Get("url_id")
	selectedID, _ := strconv.Atoi(selectedURLIDStr)

	if selectedID == 0 && len(urls) > 0 {
		selectedID = urls[0].ID
	}

	// Ambil data history probe (untuk chart)
	var historyData []models.ProbeHistory
	if selectedID > 0 {
		qrange := r.URL.Query().Get("range")
		var since time.Time
		now := time.Now()
		switch qrange {
		case "1s":
			since = now.Add(-1 * time.Second)
		case "10s":
			since = now.Add(-10 * time.Second)
		case "30s":
			since = now.Add(-30 * time.Second)
		case "1min":
			since = now.Add(-1 * time.Minute)
		case "1h":
			since = now.Add(-1 * time.Hour)
		case "4h":
			since = now.Add(-4 * time.Hour)
		case "1d":
			since = now.Add(-24 * time.Hour)
		case "1w":
			since = now.Add(-7 * 24 * time.Hour)
		case "1m":
			since = now.Add(-30 * 24 * time.Hour)
		}
		if !since.IsZero() {
			historyData, err = h.App.Store.GetProbeHistoryByRange(selectedID, since)
			if err != nil {
				log.Printf("Gagal mengambil data history/Filter: %v", err)
			}
		} else {
			historyData, err = h.App.Store.GetProbeHistory(selectedID, 30)
			if err != nil {
				log.Printf("Gagal mengambil data history: %v", err)
			}
		}
	}

	// Siapkan PageData untuk dikirim ke template
	urlActive := 0
	for _, u := range urls {
		if u.IsUp {
			urlActive++
		}
	}
	uptimePerc := 0
	if len(urls) > 0 {
		uptimePerc = int(100 * urlActive / len(urls))
	}

	jsonHistory, _ := json.Marshal(historyData)

	data := models.PageData{
		Page:             "dashboard",
		URLs:             urls,
		GlobalAvgLatency: calculateGlobalAvgLatency(urls),
		LastCheckedTime:  getLatestProbeTime(urls),
		HistoryData:      historyData,
		SelectedURLID:    selectedID,
		ChartRange:       r.URL.Query().Get("range"),
		JSONHistoryData:  template.JS(string(jsonHistory)),
		TotalItems:       int64(len(historyData)),
		TotalPages:       1,
		PageNumber:       1,
		PageSize:         len(historyData),
		GlobalUptimePct:  uptimePerc,
	}

	// Render template DASHBOARD
	tpl, perr := template.ParseFiles("templates/layout.html", "templates/dashboard.html")
	if perr != nil {
		log.Printf("Error parsing dashboard templates: %v", perr)
		http.Error(w, perr.Error(), http.StatusInternalServerError)
		return
	}
	err = tpl.ExecuteTemplate(w, "layout", data)
	if err != nil {
		log.Printf("Error rendering dashboard template: %v", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
}

// URLsPage menangani halaman '/urls'
func (h *Handlers) URLsPage(w http.ResponseWriter, r *http.Request) {
	urls, err := h.App.Store.GetAllURLs()
	if err != nil {
		log.Printf("Gagal mengambil URL: %v", err)
		http.Error(w, "Gagal mengambil data", http.StatusInternalServerError)
		return
	}

	data := models.PageData{
		Page:            "urls",
		URLs:            urls,
		LastCheckedTime: getLatestProbeTime(urls),
	}

	// Render template URLS (parse spesifik agar konten sesuai halaman)
	tpl, perr := template.ParseFiles("templates/layout.html", "templates/urls.html")
	if perr != nil {
		log.Printf("Error parsing urls templates: %v", perr)
		http.Error(w, perr.Error(), http.StatusInternalServerError)
		return
	}
	err = tpl.ExecuteTemplate(w, "layout", data)
	if err != nil {
		log.Printf("Error rendering urls template: %v", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

// SchedulerPage menangani halaman '/scheduler'
func (h *Handlers) SchedulerPage(w http.ResponseWriter, r *http.Request) {
	// Ambil interval saat ini
	interval, err := h.App.Store.GetScheduleInterval()
	if err != nil {
		log.Printf("Gagal mengambil interval: %v", err)
		http.Error(w, "Gagal mengambil data", http.StatusInternalServerError)
		return
	}

	// Ambil semua URL untuk last checked time
	urls, _ := h.App.Store.GetAllURLs()

	// Pagination params
	pageSize := 20
	if v := r.URL.Query().Get("size"); v != "" {
		if n, convErr := strconv.Atoi(v); convErr == nil && n > 0 && n <= 200 {
			pageSize = n
		}
	}
	pageNum := 1
	if v := r.URL.Query().Get("page"); v != "" {
		if n, convErr := strconv.Atoi(v); convErr == nil && n > 0 {
			pageNum = n
		}
	}
	offset := (pageNum - 1) * pageSize

	qrange := r.URL.Query().Get("range")
	now := time.Now()
	var since time.Time
	switch qrange {
	case "1s":
		since = now.Add(-1 * time.Second)
	case "10s":
		since = now.Add(-10 * time.Second)
	case "30s":
		since = now.Add(-30 * time.Second)
	case "1min":
		since = now.Add(-1 * time.Minute)
	case "1h":
		since = now.Add(-1 * time.Hour)
	case "4h":
		since = now.Add(-4 * time.Hour)
	case "1d":
		since = now.Add(-24 * time.Hour)
	case "1w":
		since = now.Add(-7 * 24 * time.Hour)
	case "1m":
		since = now.Add(-30 * 24 * time.Hour)
	}

	var totalItems int64
	var historyData []models.ProbeHistory
	if !since.IsZero() {
		totalItems, _ = h.App.Store.CountProbeHistoryByRange(since)
		historyData, err = h.App.Store.GetAllProbeHistoryByRangePaged(since, pageSize, offset)
	} else {
		totalItems, _ = h.App.Store.CountProbeHistory()
		historyData, err = h.App.Store.GetAllProbeHistoryPaged(pageSize, offset)
	}
	if err != nil {
		log.Printf("Gagal mengambil semua history: %v", err)
	}
	totalPages := 0
	if pageSize > 0 {
		totalPages = int((totalItems + int64(pageSize) - 1) / int64(pageSize))
	}

	// Buat navigator for page buttons max 10
	var pages []int
	start := 1
	end := totalPages
	if totalPages > 10 {
		if pageNum <= 6 {
			start = 1
			end = 10
		} else if pageNum+4 >= totalPages {
			start = totalPages - 9
			end = totalPages
		} else {
			start = pageNum - 5
			end = pageNum + 4
		}
	}
	for i := start; i <= end; i++ {
		pages = append(pages, i)
	}

	data := models.PageData{
		Page:            "scheduler",
		CurrentInterval: interval,
		LastCheckedTime:  getLatestProbeTime(urls),
		HistoryData:      historyData,
		PageNumber:       pageNum,
		PageSize:         pageSize,
		TotalItems:       totalItems,
		TotalPages:       totalPages,
		NavigatorPages:   pages,
		ChartRange:       qrange,
	}

	// Render template SCHEDULER
	funcMap := template.FuncMap{
		"add":      func(a, b int) int { return a + b },
		"subtract": func(a, b int) int { return a - b },
	}
	tpl, perr := template.New("layout.html").Funcs(funcMap).ParseFiles("templates/layout.html", "templates/scheduler.html")
	if perr != nil {
		log.Printf("Error parsing scheduler templates: %v", perr)
		http.Error(w, perr.Error(), http.StatusInternalServerError)
		return
	}
	err = tpl.ExecuteTemplate(w, "layout", data)
	if err != nil {
		log.Printf("Error rendering scheduler template: %v", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

// AddURL menangani form 'Tambah URL'
func (h *Handlers) AddURL(w http.ResponseWriter, r *http.Request) {
	url := r.FormValue("url")
	if url == "" {
		http.Redirect(w, r, "/urls", http.StatusSeeOther)
		return
	}
	mode := r.FormValue("mode")
	if mode != "tcp" && mode != "icmp" {
		mode = "http"
	}

	if mode == "http" && !((strings.HasPrefix(url, "http://")) || (strings.HasPrefix(url, "https://"))) {
		url = "https://" + url
	}

	err := h.App.Store.AddURLWithMode(url, mode)
	if err != nil {
		log.Printf("Gagal menambah URL: %v", err)
	}
	http.Redirect(w, r, "/urls", http.StatusSeeOther)
}

// DeleteURL menangani link 'Hapus'
func (h *Handlers) DeleteURL(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	idStr := vars["id"]

	id, err := strconv.Atoi(idStr)
	if err != nil {
		http.Error(w, "ID tidak valid", http.StatusBadRequest)
		return
	}
	err = h.App.Store.DeleteProbeHistory(id)
	if err != nil {
		log.Printf("Gagal menghapus history URL: %v", err)
	}
	err = h.App.Store.DeleteURL(id)
	if err != nil {
		log.Printf("Gagal menghapus URL: %v", err)
	}
	http.Redirect(w, r, "/urls", http.StatusSeeOther)
}

// UpdateSettings menangani form 'Simpan Jadwal'
func (h *Handlers) UpdateSettings(w http.ResponseWriter, r *http.Request) {
	interval := r.FormValue("interval")

	// Validasi input
	validIntervals := map[string]bool{
		"@every 1s":  true,
		"@every 10s": true,
		"@every 30s": true,
		"@every 1m":  true,
		"@every 5m":  true,
		"@every 10m": true,
		"@every 30m": true,
	}
	if !validIntervals[interval] {
		log.Println("Input test", interval)
		http.Redirect(w, r, "/scheduler", http.StatusSeeOther)
		return
	}
	err := h.App.Store.SetScheduleInterval(interval)
	if err != nil {
		log.Println("Failed to save interval:", err)
		http.Redirect(w, r, "/scheduler", http.StatusSeeOther)
		return
	}

	// Restart Cron Job
	log.Printf("Changing scheduler interval to: %s", interval)
	h.App.Scheduler.Remove(h.App.JobID)
	newID, err := h.App.Scheduler.AddFunc(interval, scheduler.CreateJob(h.App.Store))
	if err != nil {
		log.Println("Failed to add new cron job:", err)
	}
	h.App.JobID = newID

	http.Redirect(w, r, "/scheduler", http.StatusSeeOther)
}

// === FUNCTION HELPER ===

// calculateGlobalAvgLatency menghitung rata-rata dari semua URL
func calculateGlobalAvgLatency(urls []models.TargetURL) int64 {
	var totalSum, totalCount int64
	for _, u := range urls {
		totalSum += u.TotalLatencySum
		totalCount += u.TotalProbeCount
	}
	if totalCount == 0 {
		return 0
	}
	return totalSum / totalCount
}

// ChartAPI mengembalikan data history dalam JSON untuk chart (refresh real-time)
func (h *Handlers) ChartAPI(w http.ResponseWriter, r *http.Request) {
	urlIDStr := r.URL.Query().Get("url_id")
	rangeParam := r.URL.Query().Get("range")
	urlID, _ := strconv.Atoi(urlIDStr)
	if urlID <= 0 {
		http.Error(w, `{"error":"url_id required"}`, http.StatusBadRequest)
		return
	}
	var since time.Time
	now := time.Now()
	switch rangeParam {
	case "1s":
		since = now.Add(-1 * time.Second)
	case "10s":
		since = now.Add(-10 * time.Second)
	case "30s":
		since = now.Add(-30 * time.Second)
	case "1min":
		since = now.Add(-1 * time.Minute)
	case "1h":
		since = now.Add(-1 * time.Hour)
	case "4h":
		since = now.Add(-4 * time.Hour)
	case "1d":
		since = now.Add(-24 * time.Hour)
	case "1w":
		since = now.Add(-7 * 24 * time.Hour)
	case "1m":
		since = now.Add(-30 * 24 * time.Hour)
	default:
		since = now.Add(-24 * time.Hour)
	}
	historyData, err := h.App.Store.GetProbeHistoryByRange(urlID, since)
	if err != nil {
		log.Printf("ChartAPI: %v", err)
		http.Error(w, `{"error":"failed to get history"}`, http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(historyData)
}

// getLatestProbeTime mencari waktu probe terbaru dari semua URL
func getLatestProbeTime(urls []models.TargetURL) time.Time {
	var latest time.Time
	for _, u := range urls {
		if u.LastChecked.After(latest) {
			latest = u.LastChecked
		}
	}
	return latest
}
