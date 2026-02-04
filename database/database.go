package database

import (
	"database/sql"
	"log"
	"test/models"
	"time"

	_ "modernc.org/sqlite"
)

type Store struct {
	Db *sql.DB
}

func NewStore(dbPath string) *Store {
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		log.Fatalf("Gagal membuka database: %v", err)
	}

	// --- TABEL URLS ---
	createTableSQL := `
	CREATE TABLE IF NOT EXISTS urls (
		"id" INTEGER NOT NULL PRIMARY KEY AUTOINCREMENT,
		"url" TEXT NOT NULL UNIQUE,
		"last_status" INTEGER DEFAULT 0,
		"last_latency_ms" INTEGER DEFAULT 0,
		"last_checked" DATETIME,
		"first_up_time" DATETIME DEFAULT NULL,
		"total_probe_count" INTEGER DEFAULT 0,
		"total_latency_sum" INTEGER DEFAULT 0
	);`
	_, err = db.Exec(createTableSQL)
	if err != nil {
		log.Fatalf("Gagal membuat tabel urls: %v", err)
	}

	// Add probe_mode column if it doesn't exist
	_, err = db.Exec("ALTER TABLE urls ADD COLUMN probe_mode TEXT NOT NULL DEFAULT 'http'")
	if err != nil {
		// Better to check for specific error, but for now we assume it might be "duplicate column"
		log.Printf("Could not add 'probe_mode' column, it might already exist: %v", err)
	}

	// --- TABEL SETTINGS ---
	createSettingsTableSQL := `
	CREATE TABLE IF NOT EXISTS settings (
		"key" TEXT NOT NULL PRIMARY KEY,
		"value" TEXT
	);`
	_, err = db.Exec(createSettingsTableSQL)
	if err != nil {
		log.Fatalf("Gagal membuat tabel settings: %v", err)
	}
	// Isi nilai default untuk interval (jika belum ada)
	_, err = db.Exec("INSERT OR IGNORE INTO settings (key, value) VALUES ('schedule_interval', '@every 1m')") // Set default 1m
	if err != nil {
		log.Fatalf("Gagal set default interval: %v", err)
	}

	// --- TABEL PROBE HISTORY ---
	createHistoryTableSQL := `
	CREATE TABLE IF NOT EXISTS probe_history (
		"id" INTEGER NOT NULL PRIMARY KEY AUTOINCREMENT,
		"url_id" INTEGER,
		"latency_ms" INTEGER,
		"timestamp" DATETIME,
		FOREIGN KEY(url_id) REFERENCES urls(id) ON DELETE CASCADE
	);`
	_, err = db.Exec(createHistoryTableSQL)
	if err != nil {
		log.Fatalf("Gagal membuat tabel probe_history: %v", err)
	}

	// Inisialisasi kolom probe_mode untuk data yang sudah ada
	_, err = db.Exec("UPDATE urls SET probe_mode = 'http' WHERE probe_mode IS NULL")
	if err != nil {
		log.Fatalf("Gagal mengupdate probe_mode untuk data yang ada: %v", err)
	}

	return &Store{Db: db}
}

// --- FUNGSI SETTINGS ---
func (s *Store) GetScheduleInterval() (string, error) {
	var interval string
	err := s.Db.QueryRow("SELECT value FROM settings WHERE key = 'schedule_interval'").Scan(&interval)
	return interval, err
}

func (s *Store) SetScheduleInterval(interval string) error {
	_, err := s.Db.Exec("UPDATE settings SET value = ? WHERE key = 'schedule_interval'", interval)
	return err
}

// --- FUNGSI URLS ---
func (s *Store) GetAllURLs() ([]models.TargetURL, error) {
	rows, err := s.Db.Query("SELECT id, url, probe_mode, last_status, last_latency_ms, last_checked, first_up_time, total_probe_count, total_latency_sum FROM urls ORDER BY id DESC")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var urls []models.TargetURL
	for rows.Next() {
		var u models.TargetURL
		var lastChecked sql.NullTime
		if err := rows.Scan(&u.ID, &u.URL, &u.ProbeMode, &u.LastStatus, &u.LastLatencyMs, &lastChecked, &u.FirstUpTime, &u.TotalProbeCount, &u.TotalLatencySum); err != nil {
			return nil, err
		}
		if lastChecked.Valid {
			u.LastChecked = lastChecked.Time
		}
		u.IsUp = (u.LastStatus == 200)
		urls = append(urls, u)
	}
	return urls, nil
}

func (s *Store) AddURLWithMode(url string, mode string) error {
	_, err := s.Db.Exec("INSERT INTO urls (url, probe_mode, last_checked) VALUES (?, ?, ?)", url, mode, time.Now())
	return err
}

func (s *Store) DeleteURL(id int) error {
	_, err := s.Db.Exec("DELETE FROM urls WHERE id = ?", id)
	return err
}

// --- FUNGSI PROBE STATS ---
func (s *Store) UpdateProbeStats(id int, status int, latency int64, firstUpTime sql.NullTime) error {
	_, err := s.Db.Exec(`
		UPDATE urls SET
			last_status = ?,
			last_latency_ms = ?,
			last_checked = ?,
			first_up_time = ?,
			total_probe_count = total_probe_count + 1,
			total_latency_sum = total_latency_sum + ?
		WHERE id = ?`,
		status, latency, time.Now(), firstUpTime, latency, id)
	return err
}

func (s *Store) UpdateProbeNetworkError(id int, latency int64, firstUpTime sql.NullTime) error {
	_, err := s.Db.Exec(`
		UPDATE urls SET
			last_status = 0,
			last_latency_ms = ?,
			last_checked = ?,
			first_up_time = ?
		WHERE id = ?`,
		latency, time.Now(), firstUpTime, id)
	return err
}

// --- FUNGSI PROBE HISTORY (Diperbarui) ---

// AddProbeHistory menyimpan satu log probe
func (s *Store) AddProbeHistory(urlID int, latencyMs int64) error {
	_, err := s.Db.Exec("INSERT INTO probe_history (url_id, latency_ms, timestamp) VALUES (?, ?, ?)",
		urlID, latencyMs, time.Now())
	// Juga membersihkan history lama agar DB tidak penuh
	// Simpan sampai 1.000.000 baris terbaru, sisanya dihapus
	_, _ = s.Db.Exec("DELETE FROM probe_history WHERE id NOT IN (SELECT id FROM probe_history ORDER BY timestamp DESC LIMIT 1000000)")
	return err
}

// DeleteProbeHistory membersihkan history saat URL dihapus
func (s *Store) DeleteProbeHistory(urlID int) error {
	_, err := s.Db.Exec("DELETE FROM probe_history WHERE url_id = ?", urlID)
	return err
}

// GetProbeHistory mengambil N probe terakhir untuk SATU URL (untuk Dashboard)
func (s *Store) GetProbeHistory(urlID int, limit int) ([]models.ProbeHistory, error) {
	// Diperbarui: Menggunakan JOIN untuk mengambil urls.url
	rows, err := s.Db.Query(`
		SELECT h.url_id, u.url, h.latency_ms, h.timestamp 
		FROM probe_history h
		JOIN urls u ON h.url_id = u.id
		WHERE h.url_id = ? 
		ORDER BY h.timestamp DESC 
		LIMIT ?`, urlID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var history []models.ProbeHistory
	for rows.Next() {
		var h models.ProbeHistory
		if err := rows.Scan(&h.URLID, &h.URL, &h.LatencyMs, &h.Timestamp); err != nil {
			return nil, err
		}
		history = append(history, h)
	}
	return history, nil
}

// GetAllProbeHistory mengambil N probe terakhir dari SEMUA URL (untuk Scheduler)
func (s *Store) GetAllProbeHistory(limit int) ([]models.ProbeHistory, error) {
	rows, err := s.Db.Query(`
        SELECT h.url_id, u.url, h.latency_ms, h.timestamp 
        FROM probe_history h
        JOIN urls u ON h.url_id = u.id
        ORDER BY h.timestamp DESC 
        LIMIT ?`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var history []models.ProbeHistory
	for rows.Next() {
		var h models.ProbeHistory
		if err := rows.Scan(&h.URLID, &h.URL, &h.LatencyMs, &h.Timestamp); err != nil {
			return nil, err
		}
		history = append(history, h)
	}
	return history, nil
}

// GetAllProbeHistoryPaged mengambil probe_history dengan limit dan offset (untuk pagination)
func (s *Store) GetAllProbeHistoryPaged(limit int, offset int) ([]models.ProbeHistory, error) {
	rows, err := s.Db.Query(`
        SELECT h.url_id, u.url, h.latency_ms, h.timestamp
        FROM probe_history h
        JOIN urls u ON h.url_id = u.id
        ORDER BY h.timestamp DESC
        LIMIT ? OFFSET ?`, limit, offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var history []models.ProbeHistory
	for rows.Next() {
		var h models.ProbeHistory
		if err := rows.Scan(&h.URLID, &h.URL, &h.LatencyMs, &h.Timestamp); err != nil {
			return nil, err
		}
		history = append(history, h)
	}
	return history, nil
}

// CountProbeHistory menghitung total baris probe_history
func (s *Store) CountProbeHistory() (int64, error) {
	var total int64
	err := s.Db.QueryRow(`SELECT COUNT(1) FROM probe_history`).Scan(&total)
	return total, err
}

// CountProbeHistoryByRange menghitung baris probe_history sejak waktu tertentu
func (s *Store) CountProbeHistoryByRange(since time.Time) (int64, error) {
	var total int64
	err := s.Db.QueryRow(`SELECT COUNT(1) FROM probe_history WHERE timestamp >= ?`, since).Scan(&total)
	return total, err
}

// GetAllProbeHistoryByRangePaged mengambil probe_history sejak waktu tertentu (semua URL), paged
func (s *Store) GetAllProbeHistoryByRangePaged(since time.Time, limit int, offset int) ([]models.ProbeHistory, error) {
	rows, err := s.Db.Query(`
        SELECT h.url_id, u.url, h.latency_ms, h.timestamp
        FROM probe_history h
        JOIN urls u ON h.url_id = u.id
        WHERE h.timestamp >= ?
        ORDER BY h.timestamp DESC
        LIMIT ? OFFSET ?`, since, limit, offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var history []models.ProbeHistory
	for rows.Next() {
		var h models.ProbeHistory
		if err := rows.Scan(&h.URLID, &h.URL, &h.LatencyMs, &h.Timestamp); err != nil {
			return nil, err
		}
		history = append(history, h)
	}
	return history, nil
}

// GetProbeHistoryByRange mengambil probe untuk SATU URL dalam interval waktu tertentu (ASC)
func (s *Store) GetProbeHistoryByRange(urlID int, since time.Time) ([]models.ProbeHistory, error) {
	rows, err := s.Db.Query(`
		SELECT h.url_id, u.url, h.latency_ms, h.timestamp 
		FROM probe_history h
		JOIN urls u ON h.url_id = u.id
		WHERE h.url_id = ? AND h.timestamp >= ?
		ORDER BY h.timestamp ASC`, urlID, since)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var history []models.ProbeHistory
	for rows.Next() {
		var h models.ProbeHistory
		if err := rows.Scan(&h.URLID, &h.URL, &h.LatencyMs, &h.Timestamp); err != nil {
			return nil, err
		}
		history = append(history, h)
	}
	return history, nil
}
