package scheduler

import (
	"database/sql"
	"log"
	"sync"
	"test/database"
	"test/models"
	"test/probe"
	"time"

	"github.com/robfig/cron/v3"
)

// CreateJob mengembalikan fungsi job cron
func CreateJob(store *database.Store) func() {
	return func() {
		log.Println("[CRON] Starting probe...")

		// Ambil semua URL dari database
		urls, err := store.GetAllURLs()
		if err != nil {
			log.Printf("[CRON] Failed to retrieve URLs: %v\n", err)
			return
		}

		if len(urls) == 0 {
			log.Println("[CRON] No URLs to probe.")
			return
		}

		// === KONFIGURASI CONCURRENCY ===
		// Batasi hanya 50 ping bersamaan agar Windows/Linux tidak error socket
		maxConcurrent := 50
		sem := make(chan struct{}, maxConcurrent)
		var wg sync.WaitGroup

		for _, u := range urls {
			wg.Add(1)

			go func(target models.TargetURL) {
				defer wg.Done()

				// Acquire Token
				sem <- struct{}{}
				defer func() { <-sem }()

				// --- JALANKAN PROBE ---
				result := probe.DoProbe(target.URL)

				// --- LOGIKA UPDATE DATABASE ---
				var newFirstUpTime sql.NullTime = target.FirstUpTime
				wasUp := (target.LastStatus == 200)
				isNowUp := (result.StatusCode == 200)

				if !wasUp && isNowUp {
					newFirstUpTime = sql.NullTime{Time: time.Now(), Valid: true}
				} else if wasUp && !isNowUp {
					newFirstUpTime = sql.NullTime{Time: time.Time{}, Valid: false}
				}

				var dbErr error
				if result.StatusCode > 0 {
					// Update Stats di tabel URL
					dbErr = store.UpdateProbeStats(target.ID, result.StatusCode, result.LatencyMs, newFirstUpTime)

					// Update History jika berhasil
					if dbErr == nil {
						// FIX: Mengirim 5 parameter sesuai database.go yang baru
						// (urlID, latency, statusCode, statusStr, description)
						_ = store.AddProbeHistory(target.ID, result.LatencyMs, result.StatusCode, "UP", "OK")
					}
				} else {
					// Update jika error/down
					dbErr = store.UpdateProbeNetworkError(target.ID, result.LatencyMs, newFirstUpTime)

					// Opsional: Simpan history kalau DOWN (agar di grafik kelihatan merah/putus)
					if dbErr == nil {
						_ = store.AddProbeHistory(target.ID, 0, 0, "DOWN", "Timeout/Error")
					}
				}

				if dbErr != nil {
					log.Printf("[CRON] Failed to update DB for %s: %v\n", target.URL, dbErr)
				}

			}(u)
		}

		wg.Wait()
		log.Println("[CRON] Probe finished.")
	}
}

// StartScheduler starts the cron job
func StartScheduler(interval string, store *database.Store) (*cron.Cron, cron.EntryID) {
	log.Printf("Starting scheduler (every %s)...", interval)
	c := cron.New()

	id, _ := c.AddFunc(interval, CreateJob(store))
	c.Start()

	return c, id
}
