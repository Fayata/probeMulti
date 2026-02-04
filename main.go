package main

import (
	"html/template"
	"log"
	"net/http"
	"test/database"
	"test/handler"
	"test/scheduler"

	"github.com/gorilla/mux"
)

func main() {
	// Inisialisasi Database
	store := database.NewStore("probe.db")
	log.Println("Database terhubung dan tabel siap.")

	// Muat SEMUA Template HTML dengan ParseGlob
	tpl, err := template.ParseFiles(
		"templates/layout.html",
		"templates/dashboard.html",
		"templates/urls.html",
	)
	if err != nil {
		log.Fatalf("Gagal memuat template (ParseGlob): %v", err)
	}

	// Debug: Print template names yang berhasil dimuat
	log.Println("Templates yang dimuat:")
	for _, t := range tpl.Templates() {
		log.Printf("  - %s", t.Name())
	}

	// Ambil interval awal dari DB
	initialInterval, err := store.GetScheduleInterval()
	if err != nil {
		log.Fatalf("Gagal mengambil interval awal: %v", err)
	}

	// Buat struct 'app'
	app := &handler.Application{
		Store:     store,
		Templates: tpl,
	}

	// Mulai Scheduler dan simpan state-nya ke 'app'
	app.Scheduler, app.JobID = scheduler.StartScheduler(initialInterval, app.Store)

	// Setup Handlers
	h := handler.NewHandlers(app)
	r := mux.NewRouter()

	// Routing untuk Halaman
	r.HandleFunc("/", h.DashboardPage).Methods("GET")
	r.HandleFunc("/urls", h.URLsPage).Methods("GET")
	r.HandleFunc("/scheduler", h.SchedulerPage).Methods("GET")

	// Routing untuk Aksi (POST/GET)
	r.HandleFunc("/add", h.AddURL).Methods("POST")
	r.HandleFunc("/delete/{id:[0-9]+}", h.DeleteURL).Methods("GET")
	r.HandleFunc("/settings", h.UpdateSettings).Methods("POST")
	r.HandleFunc("/api/chart", h.ChartAPI).Methods("GET")
	r.HandleFunc("/api/scheduler/history", h.SchedulerHistoryAPI).Methods("GET")
	r.HandleFunc("/api/urls", h.URLsAPI).Methods("GET")
	r.HandleFunc("/favicon.ico", func(w http.ResponseWriter, r *http.Request) {
		// Pakai logo.png sebagai favicon sederhana (hindari 404 di browser)
		w.Header().Set("Content-Type", "image/png")
		http.ServeFile(w, r, "./static/logo.png")
	}).Methods("GET")

	// Routing untuk file statis (CSS, JS, Gambar)
	fs := http.FileServer(http.Dir("./static/"))
	r.PathPrefix("/static/").Handler(http.StripPrefix("/static/", fs))

	port := ":8080"
	log.Printf("Server berjalan di http://localhost%s\n", port)
	log.Fatal(http.ListenAndServe(port, r))
}
