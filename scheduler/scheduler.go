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

// CreateJob adalah fungsi yang mengembalikan fungsi job dengan FULL THREAD IMPLEMENTATION
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

		// Get scheduler thread count (berapa URL diproses bersamaan)
		schedulerThreadCount, err := store.GetSchedulerThreadCount()
		if err != nil {
			log.Printf("[CRON] Failed to get scheduler thread count, using default 1: %v\n", err)
			schedulerThreadCount = 1
		}

		log.Printf("[CRON] Processing %d URLs with scheduler thread count: %d\n", len(urls), schedulerThreadCount)

		// Semaphore untuk mengontrol berapa URL yang diproses bersamaan
		urlSemaphore := make(chan struct{}, schedulerThreadCount)
		var urlWaitGroup sync.WaitGroup

		// Process setiap URL
		for _, u := range urls {
			urlWaitGroup.Add(1)
			go func(targetURL models.TargetURL) {
				defer urlWaitGroup.Done()

				// Acquire semaphore untuk scheduler thread limit
				urlSemaphore <- struct{}{}
				defer func() { <-urlSemaphore }() // Release semaphore

				log.Printf("[CRON] Processing URL: %s with %d threads\n", targetURL.URL, targetURL.ThreadCount)

				// Jalankan probe sebanyak url.ThreadCount kali secara concurrent
				probeSemaphore := make(chan struct{}, targetURL.ThreadCount)
				var probeWaitGroup sync.WaitGroup

				// Channel untuk mengumpulkan hasil probe
				results := make(chan probe.ProbeResult, targetURL.ThreadCount)

				// Lakukan probe sebanyak ThreadCount kali
				for i := 0; i < targetURL.ThreadCount; i++ {
					probeWaitGroup.Add(1)
					go func(threadIndex int) {
						defer probeWaitGroup.Done()

						probeSemaphore <- struct{}{}
						defer func() { <-probeSemaphore }()

						// Pilih probe mode
						var result probe.ProbeResult
						switch targetURL.ProbeMode {
						case "tcp":
							result = probe.DoTCPPing(targetURL.URL)
						case "icmp":
							result = probe.DoICMPProbe(targetURL.URL)
						default:
							result = probe.DoHTTPProbe(targetURL.URL)
						}

						log.Printf("[CRON] Thread %d for %s -> Status: %d, Latency: %dms\n",
							threadIndex+1, targetURL.URL, result.StatusCode, result.LatencyMs)

						// Kirim result ke channel
						results <- result
					}(i)
				}

				// Wait semua probe selesai
				probeWaitGroup.Wait()
				close(results)

				// Kumpulkan semua hasil dan hitung average
				var totalLatency int64
				var successCount int
				var lastStatus int
				var hasSuccess bool

				for result := range results {
					totalLatency += result.LatencyMs

					// Track jika ada yang success
					if result.StatusCode > 0 {
						lastStatus = result.StatusCode
						hasSuccess = true
						if result.StatusCode == 200 {
							successCount++
						}
					}
				}

				// Hitung average latency dari semua thread
				avgLatency := totalLatency / int64(targetURL.ThreadCount)

				// Update database dengan hasil probe
				// Logic uptime: jika ada minimal 1 success, dianggap UP
				var newFirstUpTime sql.NullTime = targetURL.FirstUpTime
				wasUp := (targetURL.LastStatus == 200)
				isNowUp := hasSuccess && (lastStatus == 200)

				if !wasUp && isNowUp {
					newFirstUpTime = sql.NullTime{Time: time.Now(), Valid: true}
				} else if wasUp && !isNowUp {
					newFirstUpTime = sql.NullTime{Time: time.Time{}, Valid: false}
				}

				// Tentukan status dan deskripsi berdasarkan hasil
				var status, description string
				if hasSuccess {
					if lastStatus == 200 {
						status = "Up"
						description = "Succeed"
					} else if lastStatus == 429 {
						status = "Up"
						description = "Too Many Requests"
					} else {
						status = "Up"
						description = "Warning"
					}
				} else {
					status = "Down"
					description = "Network Error"
				}

				// Update stats di database
				if hasSuccess {
					err = store.UpdateProbeStats(targetURL.ID, lastStatus, avgLatency, newFirstUpTime)
				} else {
					err = store.UpdateProbeNetworkError(targetURL.ID, avgLatency, newFirstUpTime)
				}

				// Selalu catat history
				if err == nil {
					err = store.AddProbeHistory(targetURL.ID, avgLatency, lastStatus, status, description)
				}

				if err != nil {
					log.Printf("[CRON] Failed to update DB for %s: %v\n", targetURL.URL, err)
				} else {
					log.Printf("[CRON] Completed %s -> Avg Status: %d, Avg Latency: %dms (from %d threads, %d success)\n",
						targetURL.URL, lastStatus, avgLatency, targetURL.ThreadCount, successCount)
				}
			}(u)
		}

		// Wait semua URL selesai diproses
		urlWaitGroup.Wait()
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
