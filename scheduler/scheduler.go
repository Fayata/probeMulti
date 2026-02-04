package scheduler

import (
	"database/sql"
	"log"
	"test/database"
	"test/probe"
	"time"

	"github.com/robfig/cron/v3"
)

// CreateJob adalah fungsi yang mengembalikan fungsi job
func CreateJob(store *database.Store) func() {
	return func() {
		log.Println("[CRON] Starting probe...")
		urls, err := store.GetAllURLs()
		if err != nil {
			log.Printf("[CRON] Failed to retrieve URLs: %v\n", err)
			return
		}

		if len(urls) == 0 {
			log.Println("[CRON] No URLs to probe.")
			return
		}

		// Jalankan probe untuk setiap URL
		for _, u := range urls {
			var result probe.ProbeResult
			// Mode probe: "http" (default) atau "tcp" (opsi mirip ICMP)
			switch u.ProbeMode {
			case "tcp":
				result = probe.DoTCPPing(u.URL)
			case "icmp":
				result = probe.DoICMPProbe(u.URL)
			default:
				result = probe.DoHTTPProbe(u.URL)
			}

			// --- LOGIKA UPTIME ---
			var newFirstUpTime sql.NullTime = u.FirstUpTime
			wasUp := (u.LastStatus == 200)
			isNowUp := (result.StatusCode == 200)

			if !wasUp && isNowUp {
				newFirstUpTime = sql.NullTime{Time: time.Now(), Valid: true}
			} else if wasUp && !isNowUp {
				newFirstUpTime = sql.NullTime{Time: time.Time{}, Valid: false}
			}

			if result.StatusCode > 0 {
				err = store.UpdateProbeStats(u.ID, result.StatusCode, result.LatencyMs, newFirstUpTime)
				if err == nil {
					err = store.AddProbeHistory(u.ID, result.LatencyMs)
				}
			} else {
				err = store.UpdateProbeNetworkError(u.ID, result.LatencyMs, newFirstUpTime)
			}

			if err != nil {
				log.Printf("[CRON] Failed to update DB for %s: %v\n", u.URL, err)
			} else {
				log.Printf("[CRON] Probe %s -> Status: %d, Latency: %dms\n", u.URL, result.StatusCode, result.LatencyMs)
			}
		}
		log.Println("[CRON] Probe finished.")
	}
}

// StartScheduler starts the cron job
func StartScheduler(interval string, store *database.Store) (*cron.Cron, cron.EntryID) {
	c := cron.New()
	entryID, err := c.AddFunc(interval, CreateJob(store))
	if err != nil {
		log.Fatalf("Failed to add cron job: %v", err)
	}
	c.Start()
	log.Printf("Scheduler started with interval: %s\n", interval)
	return c, entryID
}
