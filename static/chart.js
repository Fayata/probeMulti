// Chart seperti referensi: skala waktu 24h (per jam) vs 1W (per hari), response time kiri real-time, refresh sesuai range
let latencyChartInstance = null;
let chartRefreshTimer = null;
let chartZoomActive = false;

function getRefreshMs(range) {
    const r = range || '1d';
    const ms = { '1s': 1000, '10s': 2000, '30s': 2000, '1min': 3000, '1h': 5000, '4h': 10000, '1d': 15000, '1w': 30000, '1m': 60000 };
    return ms[r] != null ? ms[r] : 15000;
}

function getTimeScaleOptions(range) {
    const r = range || '1d';
    const configs = {
        '1s':   { unit: 'millisecond', stepSize: 200,  maxTicks: 8 },
        '10s':  { unit: 'second', stepSize: 2,  maxTicks: 8 },
        '30s':  { unit: 'second', stepSize: 5,  maxTicks: 8 },
        '1min': { unit: 'second', stepSize: 10, maxTicks: 8 },
        '1h':   { unit: 'minute', stepSize: 5,  maxTicks: 10 },
        '4h':   { unit: 'minute', stepSize: 15, maxTicks: 10 },
        '1d':   { unit: 'hour',   stepSize: 2,  maxTicks: 12 }, 
        '1w':   { unit: 'day',    stepSize: 1,  maxTicks: 10 },  
        '1m':   { unit: 'day',    stepSize: 2,  maxTicks: 15 }
    };
    const c = configs[r] || configs['1d'];
    const hourFormat = (r === '1d') ? 'HH:mm' : 'h a';
    return {
        unit: c.unit,
        stepSize: c.stepSize,
        maxTicksLimit: c.maxTicks,
        displayFormats: {
            millisecond: 'HH:mm:ss.SSS',
            second: 'HH:mm:ss',
            minute: 'HH:mm',
            hour: hourFormat,
            day: 'd MMM',     // 4 Feb
            week: 'd MMM',
            month: 'MMM yyyy'
        },
        tooltipFormat: (c.unit === 'millisecond') ? 'dd MMM HH:mm:ss.SSS' : 'dd MMM HH:mm:ss'
    };
}

function pad2(n) {
    return String(n).padStart(2, '0');
}

function formatHourDot(ms) {
    const d = new Date(ms);
    return pad2(d.getHours()) + '.00';
}

function formatDateMonthHour(ms) {
    const d = new Date(ms);
    const day = d.getDate();
    const month = d.toLocaleString('id-ID', { month: 'short' });
    return day + ' ' + month + ' ' + pad2(d.getHours()) + '.00';
}

function getBucketMs(range) {
    const r = range || '1d';
    const map = {
        '1m': 6 * 60 * 60 * 1000,   // 30 hari -> 6 jam
        '1w': 2 * 60 * 60 * 1000,   // 7 hari -> 2 jam
        '1d': 2 * 60 * 60 * 1000,   // 24 jam -> 2 jam
        '4h': 5 * 60 * 1000,        // 4 jam -> 5 menit
        '1h': 5 * 60 * 1000,        // 1 jam -> 5 menit
        
        '1min': 1 * 1000,
        '30s': 1 * 1000,
        '10s': 1 * 1000,            
        '1s': 1 * 1000             
    };
    return map[r] != null ? map[r] : 2 * 60 * 60 * 1000;
}

function bucketAverage(points, bucketMs) {
    if (!points || points.length === 0) return [];
    if (!bucketMs || bucketMs <= 0) return points;

    const out = [];
    let i = 0;
    while (i < points.length) {
        const start = Math.floor(points[i].x / bucketMs) * bucketMs;
        const end = start + bucketMs;
        let sum = 0;
        let count = 0;
        let lastX = points[i].x;

        while (i < points.length && points[i].x < end) {
            sum += points[i].y;
            count += 1;
            lastX = points[i].x;
            i += 1;
        }
        if (count > 0) {
            out.push({ x: lastX, y: Math.round(sum / count) });
        }
    }
    return out;
}

function buildChartData(historyData) {
    if (!historyData || historyData.length === 0) return { points: [], values: [] };
    const sorted = [...historyData].sort((a, b) => new Date(a.Timestamp) - new Date(b.Timestamp));
    const points = sorted.map(d => ({ x: new Date(d.Timestamp).getTime(), y: d.LatencyMs }));
    const values = sorted.map(d => d.LatencyMs);
    return { points, values };
}

function getDecimationSamples(range) {
    const r = range || '1d';
    const samples = {
        '1s': 30,
        '10s': 60,
        '30s': 120,
        '1min': 180,
        '1h': 240,
        '4h': 360,
        '1d': 480,
        '1w': 700,
        '1m': 900
    };
    return samples[r] != null ? samples[r] : 480;
}

function pickUniformColor(values) {
    if (!values || values.length < 2) return { line: '#25c17e', fillTop: 'rgba(37, 193, 126, 0.2)', fillBottom: 'rgba(37, 193, 126, 0)' };

    const first = values[0]; 
    const last = values[values.length - 1]; 

    const useGreen = first > last;
    if (!useGreen) {
        return { line: '#c62828', fillTop: 'rgba(198, 40, 40, 0.22)', fillBottom: 'rgba(198, 40, 40, 0)' };
    }
    return { line: '#25c17e', fillTop: 'rgba(37, 193, 126, 0.2)', fillBottom: 'rgba(37, 193, 126, 0)' };
}

function getYRangePadding(values) {
    if (!values || values.length === 0) return { min: undefined, max: undefined };
    let min = values[0];
    let max = values[0];
    for (let i = 1; i < values.length; i++) {
        if (values[i] < min) min = values[i];
        if (values[i] > max) max = values[i];
    }
    if (min === max) {
        const pad = Math.max(5, Math.round(min * 0.1));
        return { min: Math.max(0, min - pad), max: max + pad };
    }
    const span = max - min;
    const pad = Math.max(5, Math.round(span * 0.15));
    return { min: Math.max(0, min - pad), max: max + pad };
}

function getZoomSpeed(range) {
    const r = range || '1d';
    // Range kecil -> zoom lebih halus; range besar -> zoom sedikit lebih cepat
    if (r === '10s' || r === '30s' || r === '1min') return 0.04;
    if (r === '1h' || r === '4h') return 0.06;
    return 0.08; // 1d, 1w, 1m
}

function bindPanZoomToggle(canvas) {
    if (!canvas) return;
    if (canvas.__zoomToggleBound) return;
    // Double-click = toggle mode zoom ON/OFF
    canvas.addEventListener('dblclick', function () {
        try {
            chartZoomActive = !chartZoomActive;
            if (!latencyChartInstance) return;
            if (!chartZoomActive) {
                // Saat mematikan zoom, sekalian reset view supaya balik ke "full range"
                if (typeof latencyChartInstance.resetZoom === 'function') {
                    latencyChartInstance.resetZoom();
                } else {
                    latencyChartInstance.options.scales.x.min = undefined;
                    latencyChartInstance.options.scales.x.max = undefined;
                    latencyChartInstance.options.scales.y.min = undefined;
                    latencyChartInstance.options.scales.y.max = undefined;
                    latencyChartInstance.update('none');
                }
            }
        } catch (e) {}
    });
    canvas.__zoomToggleBound = true;
}

function initChart(historyData, urlId, chartRange) {
    const canvas = document.getElementById('latencyChart');
    if (!canvas) return;

    if (chartRefreshTimer) {
        clearInterval(chartRefreshTimer);
        chartRefreshTimer = null;
    }
    if (latencyChartInstance) {
        latencyChartInstance.destroy();
        latencyChartInstance = null;
    }

    const range = chartRange || '1d';
    const raw = buildChartData(historyData);
    const bucketMs = getBucketMs(range);
    const points = bucketAverage(raw.points, bucketMs);
    const values = points.map(p => p.y);
    const timeOpts = getTimeScaleOptions(range);
    const palette = pickUniformColor(values);
    const decimationSamples = getDecimationSamples(range);
    const yPad = getYRangePadding(values);
    const showPoints = points.length <= 2;

    const config = {
        type: 'line',
        data: {
            datasets: [{
                label: 'Response time (ms)',
                data: points,
                fill: true,
                borderColor: palette.line,
                borderWidth: 2,
                tension: 0.2,
                cubicInterpolationMode: 'monotone',
                showLine: true,
                spanGaps: true,
                // Range sangat kecil sering hanya 1 titik. Kalau pointRadius=0, terlihat "hilang".
                pointRadius: showPoints ? 3 : 0,
                pointHoverRadius: 6,
                backgroundColor: (ctx) => {
                    const { chart } = ctx;
                    if (!chart.chartArea) return palette.fillTop;
                    const g = chart.ctx.createLinearGradient(0, chart.chartArea.top, 0, chart.chartArea.bottom);
                    g.addColorStop(0, palette.fillTop);
                    g.addColorStop(1, palette.fillBottom);
                    return g;
                }
            }]
        },
        options: {
            responsive: true,
            maintainAspectRatio: true,
            animation: false,
            interaction: { intersect: false, mode: 'index' },
            plugins: {
                legend: { display: false },
                decimation: {
                    enabled: true,
                    algorithm: 'lttb',
                    samples: decimationSamples
                },
                zoom: {
                    pan: {
                        enabled: function() { return chartZoomActive; },
                        mode: 'x',
                        threshold: 2
                    },
                    zoom: {
                        wheel: {
                            enabled: function() { return chartZoomActive; },
                            speed: getZoomSpeed(range)
                        },
                        pinch: {
                            enabled: function() { return chartZoomActive; }
                        },
                        mode: 'x',
                        onZoom: function({ chart }) {
                            try {
                                const pts = chart?.data?.datasets?.[0]?.data || [];
                                const xScale = chart?.scales?.x;
                                const win = (xScale && xScale.min != null && xScale.max != null) ? (xScale.max - xScale.min) : null;
                                // Saat window waktu makin kecil, tampilkan titik biar gap "terbaca"
                                const showDots = (pts.length <= 2) || (win != null && win <= 2 * 60 * 1000);
                                chart.data.datasets[0].pointRadius = showDots ? 2 : 0;
                                chart.update('none');
                            } catch (e) {}
                        }
                    },
                    limits: {
                        x: { min: 'original', max: 'original' }
                    }
                },
                tooltip: {
                    backgroundColor: 'rgba(18, 20, 23, 0.95)',
                    titleColor: '#fff',
                    bodyColor: '#fff',
                    borderColor: 'rgba(255,255,255,0.08)',
                    borderWidth: 1,
                    padding: 10,
                    displayColors: false,
                    callbacks: {
                        title: function(items) {
                            if (items.length && items[0].parsed && items[0].parsed.x != null)
                                return new Date(items[0].parsed.x).toLocaleString('id-ID', { dateStyle: 'short', timeStyle: 'medium' });
                            return 'Response time';
                        },
                        label: function(ctx) { return (ctx.parsed.y || 0) + ' ms'; }
                    }
                }
            },
            scales: {
                x: {
                    type: 'time',
                    time: {
                        unit: timeOpts.unit,
                        stepSize: timeOpts.stepSize,
                        displayFormats: timeOpts.displayFormats,
                        tooltipFormat: timeOpts.tooltipFormat
                    },
                    ticks: {
                        color: 'rgba(255, 255, 255, 0.6)',
                        maxRotation: 0,
                        minRotation: 0,
                        maxTicksLimit: timeOpts.maxTicksLimit
                    },
                    grid: { display: false }
                },
                y: {
                    // Pindahkan axis ms ke kanan
                    position: 'right',
                    beginAtZero: false,
                    suggestedMin: yPad.min,
                    suggestedMax: yPad.max,
                    ticks: {
                        color: 'rgba(255, 255, 255, 0.7)',
                        callback: function(v) { return v + ' ms'; }
                    },
                    grid: { color: 'rgba(255, 255, 255, 0.06)' }
                }
            }
        }
    };

    latencyChartInstance = new Chart(canvas.getContext('2d'), config);
    // Default: zoom OFF, user double-click untuk ON/OFF zoom
    chartZoomActive = false;
    bindPanZoomToggle(canvas);

    const refreshMs = getRefreshMs(range);
    if (urlId && urlId > 0 && refreshMs > 0) {
        chartRefreshTimer = setInterval(function() {
            fetch('/api/chart?url_id=' + encodeURIComponent(urlId) + '&range=' + encodeURIComponent(range))
                .then(function(res) { return res.ok ? res.json() : Promise.reject(); })
                .then(function(data) {
                    const rawNext = buildChartData(data);
                    const nextPoints = bucketAverage(rawNext.points, getBucketMs(range));
                    if (latencyChartInstance && nextPoints.length) {
                        const nextValues = nextPoints.map(p => p.y);
                        const nextPalette = pickUniformColor(nextValues);
                        latencyChartInstance.data.datasets[0].data = nextPoints;
                        latencyChartInstance.data.datasets[0].borderColor = nextPalette.line;
                        latencyChartInstance.data.datasets[0].pointRadius = (nextPoints.length <= 2) ? 3 : 0;
                        latencyChartInstance.data.datasets[0].backgroundColor = function(ctx) {
                            const { chart } = ctx;
                            if (!chart.chartArea) return nextPalette.fillTop;
                            const g = chart.ctx.createLinearGradient(0, chart.chartArea.top, 0, chart.chartArea.bottom);
                            g.addColorStop(0, nextPalette.fillTop);
                            g.addColorStop(1, nextPalette.fillBottom);
                            return g;
                        };
                        // Update juga target samples sesuai range (kalau range berubah via reload)
                        latencyChartInstance.options.plugins.decimation.samples = getDecimationSamples(range);
                        const nextYPad = getYRangePadding(nextValues);
                        latencyChartInstance.options.scales.y.suggestedMin = nextYPad.min;
                        latencyChartInstance.options.scales.y.suggestedMax = nextYPad.max;
                        latencyChartInstance.update('none');
                    }
                })
                .catch(function() {});
        }, refreshMs);
    }
}
