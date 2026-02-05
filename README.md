### Dashboard

Dashboard menampilkan statistik uptime, jumlah URL aktif, rata-rata response time, dan grafik performa 30 hari terakhir.

### Target URL Management

Kelola URL yang ingin dimonitor dengan mudah - tambah, hapus, dan lihat detail performa setiap URL.

### Scheduler Configuration

Atur interval pengecekan otomatis dari 1 menit hingga 30 menit sesuai kebutuhan.

## ✨ Features

- 📊 **Dashboard Real-time** - Statistik uptime, active URLs, dan average response time
- 📈 **Grafik Performa** - Visualisasi response time dalam 30 hari terakhir menggunakan Chart.js
- 🔗 **Multi-URL Monitoring** - Monitor unlimited URLs sekaligus
- ⏰ **Auto Scheduler** - Pengecekan otomatis dengan interval yang dapat dikustomisasi (1m, 5m, 10m, 30m)
- 📝 **History Tracking** - Simpan riwayat setiap pengecekan untuk analisis
- 🎨 **Modern UI** - Interface dark mode yang elegan dengan tema merah-putih
- 📱 **Responsive Design** - Optimized untuk desktop dan mobile
- 💾 **SQLite Database** - Lightweight database tanpa perlu setup kompleks

## 🛠️ Tech Stack

- **Backend**: Go (Golang) 1.20+
- **Database**: SQLite3
- **Frontend**: HTML5, CSS3, JavaScript (Vanilla)
- **Charts**: Chart.js
- **Router**: Gorilla Mux
- **Scheduler**: Robfig Cron

## 📋 Prerequisites

Sebelum menjalankan aplikasi, pastikan sistem Anda sudah terinstall:

### 1. **Go Programming Language**

- **Minimum Version**: Go 1.20 atau lebih tinggi
- **Download**: https://golang.org/dl/

**Cara Install:**

#### Windows:

1. Download installer `.msi` dari website Go
2. Jalankan installer dan ikuti instruksi
3. Verifikasi instalasi:

```cmd
go version
```

#### Linux (Ubuntu/Debian):

```bash
sudo apt update
sudo apt install golang-go

# Atau download manual:
wget https://go.dev/dl/go1.21.0.linux-amd64.tar.gz
sudo tar -C /usr/local -xzf go1.21.0.linux-amd64.tar.gz
echo 'export PATH=$PATH:/usr/local/go/bin' >> ~/.bashrc
source ~/.bashrc
go version
```

#### macOS:

```bash
# Menggunakan Homebrew:
brew install go

# Atau download dari website Go
go version
```

### 2. **Git** (Untuk clone repository)

- **Download**: https://git-scm.com/downloads

Verifikasi instalasi:

```bash
git --version
```

### 3. **GCC Compiler** (Diperlukan untuk SQLite driver)

#### Windows:

- Download **MinGW-w64**: https://sourceforge.net/projects/mingw-w64/
- Atau install **TDM-GCC**: https://jmeubank.github.io/tdm-gcc/

#### Linux:

```bash
sudo apt install build-essential
```

#### macOS:

```bash
xcode-select --install
```

Verifikasi:

```bash
gcc --version
```

## 🚀 Installation & Setup

### Step 1: Clone Repository

```bash
git clone https://github.com/fayata/probeMulti.git
cd probeMulti
```

### Step 2: Install Dependencies

```bash
go mod download
```

Dependencies yang akan diinstall:

- `github.com/gorilla/mux` - HTTP router
- `github.com/mattn/go-sqlite3` - SQLite driver
- `github.com/robfig/cron/v3` - Task scheduler

### Step 3: Persiapkan Static Files

Pastikan struktur folder sebagai berikut:

```
id-probe-status/
├── database/
│   └── database.go
├── handler/
│   └── handler.go
├── models/
│   └── url.go
├── probe/
│   └── probe.go
├── scheduler/
│   └── scheduler.go
├── static/
│   ├── style.css
│   ├── chart.js
│   └── logo.png          # Letakkan logo di sini
├── templates/
│   ├── layout.html
│   ├── dashboard.html
│   ├── urls.html
│   └── scheduler.html
├── go.mod
├── go.sum
├── main.go
└── README.md
```

**Important**: Pastikan file `static/logo.png` ada. Jika tidak ada, aplikasi tetap jalan tapi logo tidak tampil.

### Step 4: Build Application

```bash
go build -o fprobe
```

Atau untuk development (tanpa build):

```bash
go run main.go
```

### Step 5: Run Application

```bash
# Jika sudah build:
./probeMulti

# Atau langsung run:
go run main.go
```

Output yang diharapkan:

```
2025/10/29 13:38:26 Database terhubung dan tabel siap.
2025/10/29 13:38:26 Templates yang dimuat:
2025/10/29 13:38:26   - urls.html
2025/10/29 13:38:26   - dashboard.html
2025/10/29 13:38:26   - scheduler.html
2025/10/29 13:38:26   - layout
2025/10/29 13:38:26 Menjalankan scheduler (setiap @every 1m)...
2025/10/29 13:38:26 Server berjalan di http://localhost:8080
```

### Step 6: Access Application

Buka browser dan akses:

```
http://localhost:8080
```

## 📖 Usage Guide

### 1. **Dashboard** (`/`)

- Lihat statistik real-time: Total Uptime, Active URLs, Average Response Time
- Pilih URL dari dropdown untuk melihat grafik performa 30 hari
- Grafik menampilkan response time dalam milliseconds

### 2. **Target URL** (`/urls`)

- **Tambah URL Baru**: Masukkan domain (contoh: `google.com` atau `https://google.com`)
- **Monitor Status**:
  - ✅ **Up** (hijau) = Website online
  - ❌ **Down** (merah) = Website offline
- **View Details**: Status code, latency (last & average), uptime, last checked time
- **Delete URL**: Klik tombol "Hapus" untuk menghapus monitoring

### 3. **Scheduler** (`/scheduler`)

- **Atur Interval**: Pilih seberapa sering pengecekan dilakukan
  - `1 Menit` - Untuk testing/development
  - `5 Menit` - Untuk monitoring intensif
  - `10 Menit` - Balance antara akurasi dan resource
  - `30 Menit` - Untuk monitoring ringan
- **Riwayat Pembaruan**: Lihat log pengecekan terakhir dengan timestamp

## 🔧 Configuration

### Ubah Port Default

Edit file `main.go`:

```go
port := os.Getenv("PORT")
if port == "" {
    port = "8080"
}
```

### Ubah Database Location

Edit file `main.go`:

```go
store := database.NewStore("custom_path/probe.db")
```

### Ubah Scheduler Default

Edit file `database/database.go`:

```go
_, err = db.Exec("INSERT OR IGNORE INTO settings (key, value) VALUES ('schedule_interval', '@every 5m')")
```

## 📁 Project Structure

```
id-probe-status/
│
├── database/           # Database layer
│   └── database.go     # SQLite connection & queries
│
├── handler/            # HTTP handlers
│   └── handler.go      # Route handlers & logic
│
├── models/             # Data models
│   └── url.go          # TargetURL & ProbeHistory structs
│
├── probe/              # Probe engine
│   └── probe.go        # HTTP request & latency measurement
│
├── scheduler/          # Background scheduler
│   └── scheduler.go    # Cron job configuration
│
├── static/             # Static assets
│   ├── style.css       # Main stylesheet
│   ├── chart.js        # Chart initialization
│   └── logo.png        # Application logo
│
├── templates/          # HTML templates
│   ├── layout.html     # Base layout (sidebar, header)
│   ├── dashboard.html  # Dashboard page
│   ├── urls.html       # URL management page
│   └── scheduler.html  # Scheduler configuration page
│
├── go.mod              # Go module definition
├── go.sum              # Dependency checksums
├── main.go             # Application entry point
└── README.md           # This file
```

## 🐛 Troubleshooting

### Error: "gcc: command not found"

**Problem**: SQLite driver memerlukan GCC untuk compile.

**Solution**:

- **Windows**: Install MinGW atau TDM-GCC
- **Linux**: `sudo apt install build-essential`
- **macOS**: `xcode-select --install`

### Error: "template not found"

**Problem**: Template file tidak ditemukan.

**Solution**:

```bash
# Pastikan struktur folder benar:
ls templates/
# Output harus ada: layout.html, dashboard.html, urls.html, scheduler.html
```

### Error: "address already in use"

**Problem**: Port 8080 sudah digunakan aplikasi lain.

**Solution**:

```bash
# Windows: Cari process yang pakai port 8080
netstat -ano | findstr :8080
taskkill /PID <PID> /F

# Linux/Mac:
lsof -ti:8080 | xargs kill -9

# Atau ubah port di main.go
```

### Database error / corrupt

**Solution**:

```bash
# Hapus database dan buat baru:
rm probe.db
go run main.go
```

## 📊 Database Schema

### Table: `urls`

```sql
CREATE TABLE urls (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    url TEXT NOT NULL UNIQUE,
    last_status INTEGER DEFAULT 0,
    last_latency_ms INTEGER DEFAULT 0,
    last_checked DATETIME,
    first_up_time DATETIME DEFAULT NULL,
    total_probe_count INTEGER DEFAULT 0,
    total_latency_sum INTEGER DEFAULT 0
);
```

### Table: `settings`

```sql
CREATE TABLE settings (
    key TEXT NOT NULL PRIMARY KEY,
    value TEXT
);
```

### Table: `probe_history`

```sql
CREATE TABLE probe_history (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    url_id INTEGER,
    latency_ms INTEGER,
    timestamp DATETIME,
    FOREIGN KEY(url_id) REFERENCES urls(id) ON DELETE CASCADE
);
```

## 🤝 Contributing

Contributions are welcome! Silakan:

1. Fork repository ini
2. Buat branch baru (`git checkout -b feature/AmazingFeature`)
3. Commit changes (`git commit -m 'Add some AmazingFeature'`)
4. Push ke branch (`git push origin feature/AmazingFeature`)
5. Buat Pull Request

## 📝 License

This project is licensed under the MIT License.

## 🙏 Acknowledgments

- [Gorilla Mux](https://github.com/gorilla/mux) - HTTP router
- [go-sqlite3](https://github.com/mattn/go-sqlite3) - SQLite driver
- [robfig/cron](https://github.com/robfig/cron) - Cron scheduler
- [Chart.js](https://www.chartjs.org/) - Chart library

## 📞 Support

Jika ada pertanyaan atau masalah:

- Email: daffa12k@gmail.com

---

**Made with ❤️ in Indonesia** 🇮🇩
