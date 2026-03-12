package ui

import (
	"encoding/json"
	"net/http"
	"text/template"
	"time"

	"github.com/c-m3-codin/gsched/monitor"
)

// Handler holds the HTTP handlers for the monitoring UI
type Handler struct {
	monitor *monitor.MonitorState
}

// NewHandler creates a new UI handler
func NewHandler(monitor *monitor.MonitorState) *Handler {
	return &Handler{
		monitor: monitor,
	}
}

// DashboardData holds data for the dashboard template
type DashboardData struct {
	WorkerCount int
	Enabled     bool
}

// RegisterRoutes registers all UI routes
func (h *Handler) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("/", h.DashboardHandler)
	mux.HandleFunc("/api/workers", h.WorkersAPIHandler)
	mux.HandleFunc("/api/events", h.EventsAPIHandler)
	mux.HandleFunc("/api/stats", h.StatsAPIHandler)
}

// DashboardHandler serves the main dashboard page
func (h *Handler) DashboardHandler(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}

	data := DashboardData{
		WorkerCount: 5,
		Enabled:     h.monitor.Enabled,
	}

	tmpl := template.Must(template.New("dashboard").Funcs(template.FuncMap{
		"formatTime": func(t *time.Time) string {
			if t == nil {
				return "-"
			}
			return t.Format("15:04:05")
		},
		"formatDateTime": func(t *time.Time) string {
			if t == nil {
				return "-"
			}
			return t.Format("2006-01-02 15:04:05")
		},
		"duration": func(start *time.Time) string {
			if start == nil {
				return "-"
			}
			return time.Since(*start).Round(time.Second).String()
		},
	}).Parse(dashboardHTML))

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := tmpl.Execute(w, data); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

// WorkersAPIHandler returns current worker status as JSON
func (h *Handler) WorkersAPIHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	workers := h.monitor.GetAllWorkers()
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(workers)
}

// EventsAPIHandler streams worker events via SSE
func (h *Handler) EventsAPIHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Set SSE headers
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("Access-Control-Allow-Origin", "*")

	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "Streaming not supported", http.StatusInternalServerError)
		return
	}

	// Subscribe to events
	eventChan := h.monitor.Subscribe()
	defer func() {
		// Drain the channel before closing to prevent deadlock
		for range eventChan {
		}
	}()

	// Send initial snapshot
	workers := h.monitor.GetAllWorkers()
	data, _ := json.Marshal(map[string]interface{}{
		"type":    "snapshot",
		"workers": workers,
	})
	w.Write([]byte("data: "))
	w.Write(data)
	w.Write([]byte("\n\n"))
	flusher.Flush()

	// Stream events
	for {
		select {
		case event, ok := <-eventChan:
			if !ok {
				return
			}
			data, _ := json.Marshal(map[string]interface{}{
				"type":  "event",
				"event": event,
			})
			w.Write([]byte("data: "))
			w.Write(data)
			w.Write([]byte("\n\n"))
			flusher.Flush()
		case <-r.Context().Done():
			return
		}
	}
}

// StatsAPIHandler returns aggregate statistics
func (h *Handler) StatsAPIHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	workers := h.monitor.GetAllWorkers()
	events := h.monitor.GetRecentEvents()

	totalCompleted := 0
	totalIdle := 0
	totalProcessing := 0

	for _, worker := range workers {
		if worker.Status == "idle" {
			totalIdle++
		} else if worker.Status == "processing" {
			totalProcessing++
		}
		totalCompleted += worker.TasksCompleted
	}

	stats := map[string]interface{}{
		"total_workers":      len(workers),
		"idle_workers":       totalIdle,
		"processing_workers": totalProcessing,
		"total_completed":    totalCompleted,
		"recent_events":      events,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(stats)
}

// dashboardHTML contains the embedded HTML template
const dashboardHTML = `<!DOCTYPE html>
<html lang="en">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>Worker Monitor Dashboard</title>
    <style>
        * { margin: 0; padding: 0; box-sizing: border-box; }
        body {
            font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, Oxygen, Ubuntu, sans-serif;
            background: linear-gradient(135deg, #1a1a2e 0%, #16213e 100%);
            min-height: 100vh;
            color: #eee;
        }
        .container { max-width: 1400px; margin: 0 auto; padding: 20px; }
        header {
            text-align: center;
            padding: 30px 0;
            border-bottom: 1px solid #333;
            margin-bottom: 30px;
        }
        h1 {
            font-size: 2.5rem;
            background: linear-gradient(90deg, #00d9ff, #00ff88);
            -webkit-background-clip: text;
            -webkit-text-fill-color: transparent;
            margin-bottom: 10px;
        }
        .status-bar {
            display: flex;
            justify-content: center;
            gap: 30px;
            margin-top: 20px;
        }
        .status-item {
            background: rgba(255,255,255,0.1);
            padding: 15px 25px;
            border-radius: 10px;
            text-align: center;
        }
        .status-item .value { font-size: 2rem; font-weight: bold; color: #00ff88; }
        .status-item .label { font-size: 0.9rem; color: #888; margin-top: 5px; }
        .workers-grid {
            display: grid;
            grid-template-columns: repeat(auto-fit, minmax(300px, 1fr));
            gap: 20px;
            margin-bottom: 30px;
        }
        .worker-card {
            background: rgba(255,255,255,0.05);
            border-radius: 15px;
            padding: 20px;
            border: 2px solid transparent;
            transition: all 0.3s ease;
        }
        .worker-card.idle { border-color: #444; }
        .worker-card.processing {
            border-color: #00ff88;
            box-shadow: 0 0 20px rgba(0,255,136,0.3);
        }
        .worker-card.error {
            border-color: #ff4757;
            box-shadow: 0 0 20px rgba(255,71,87,0.3);
        }
        .worker-header {
            display: flex;
            justify-content: space-between;
            align-items: center;
            margin-bottom: 15px;
        }
        .worker-id { font-size: 1.2rem; font-weight: bold; color: #00d9ff; }
        .worker-status {
            padding: 5px 15px;
            border-radius: 20px;
            font-size: 0.85rem;
            font-weight: bold;
            text-transform: uppercase;
        }
        .worker-status.idle { background: #444; color: #aaa; }
        .worker-status.processing { background: #00ff88; color: #000; }
        .worker-status.error { background: #ff4757; color: #fff; }
        .job-info {
            background: rgba(0,0,0,0.3);
            border-radius: 10px;
            padding: 15px;
            margin-top: 10px;
        }
        .job-info.hidden { display: none; }
        .job-row {
            display: flex;
            justify-content: space-between;
            padding: 8px 0;
            border-bottom: 1px solid rgba(255,255,255,0.1);
        }
        .job-row:last-child { border-bottom: none; }
        .job-label { color: #888; font-size: 0.85rem; }
        .job-value { color: #fff; font-weight: 500; }
        .job-value.priority-high { color: #ff6b6b; }
        .job-value.priority-medium { color: #feca57; }
        .job-value.priority-low { color: #54a0ff; }
        .timer { font-family: 'Courier New', monospace; font-size: 1.1rem; color: #00ff88; }
        .events-section {
            background: rgba(255,255,255,0.05);
            border-radius: 15px;
            padding: 20px;
        }
        .events-section h2 { margin-bottom: 15px; color: #00d9ff; }
        .events-list { max-height: 300px; overflow-y: auto; }
        .event-item {
            display: flex;
            justify-content: space-between;
            padding: 10px 15px;
            border-bottom: 1px solid rgba(255,255,255,0.05);
            font-size: 0.9rem;
        }
        .event-item:last-child { border-bottom: none; }
        .event-type { font-weight: bold; padding: 3px 10px; border-radius: 5px; }
        .event-type.task_picked { background: #00d9ff; color: #000; }
        .event-type.task_completed { background: #00ff88; color: #000; }
        .event-type.task_failed { background: #ff4757; color: #fff; }
        .event-time { color: #666; font-size: 0.8rem; }
        .event-job { color: #aaa; }
        .disabled-overlay {
            position: fixed;
            top: 0; left: 0; right: 0; bottom: 0;
            background: rgba(0,0,0,0.8);
            display: flex;
            align-items: center;
            justify-content: center;
            z-index: 1000;
        }
        .disabled-message {
            text-align: center;
            padding: 40px;
            background: rgba(255,255,255,0.1);
            border-radius: 20px;
        }
        .disabled-message h2 { color: #ff6b6b; margin-bottom: 15px; }
        @keyframes pulse { 0%, 100% { opacity: 1; } 50% { opacity: 0.5; } }
        .processing .pulse { animation: pulse 1.5s infinite; }
    </style>
</head>
<body>
    {{if not .Enabled}}
    <div class="disabled-overlay">
        <div class="disabled-message">
            <h2>Monitoring Disabled</h2>
            <p>Set MONITOR_ENABLED=true to enable the dashboard</p>
        </div>
    </div>
    {{else}}
    <div class="container">
        <header>
            <h1>Worker Monitor Dashboard</h1>
            <p>Real-time job dispatcher monitoring</p>
            <div class="status-bar">
                <div class="status-item">
                    <div class="value" id="total-workers">{{.WorkerCount}}</div>
                    <div class="label">Total Workers</div>
                </div>
                <div class="status-item">
                    <div class="value" id="idle-count">-</div>
                    <div class="label">Idle</div>
                </div>
                <div class="status-item">
                    <div class="value" id="processing-count">-</div>
                    <div class="label">Processing</div>
                </div>
                <div class="status-item">
                    <div class="value" id="total-completed">-</div>
                    <div class="label">Tasks Completed</div>
                </div>
            </div>
        </header>
        <main>
            <div class="workers-grid" id="workers-grid"></div>
            <div class="events-section">
                <h2>Recent Events</h2>
                <div class="events-list" id="events-list"></div>
            </div>
        </main>
    </div>
    <script>
        var workersData = {};
        var eventsData = [];

        function formatTime(date) {
            return date.toLocaleTimeString('en-US', { hour12: false });
        }

        function formatDuration(seconds) {
            var mins = Math.floor(seconds / 60);
            var secs = seconds % 60;
            return mins + 'm ' + (secs < 10 ? '0' : '') + secs + 's';
        }

        function getPriorityClass(priority) {
            if (priority >= 8) return 'priority-high';
            if (priority >= 4) return 'priority-medium';
            return 'priority-low';
        }

        function escapeHtml(str) {
            return String(str).replace(/[&<>"']/g, function(s) {
                return {'&':'&amp;','<':'&lt;','>':'&gt;','"':'&quot;',"'":'&#39;'}[s];
            });
        }

        function renderWorkerCard(workerId, worker) {
            var jobHtml = '';
            if (worker.current_job) {
                var job = worker.current_job;
                var priorityClass = getPriorityClass(job.priority);
                jobHtml = '<div class="job-info">' +
                    '<div class="job-row"><span class="job-label">Job Name</span><span class="job-value">' + escapeHtml(job.job_name) + '</span></div>' +
                    '<div class="job-row"><span class="job-label">Task</span><span class="job-value">' + escapeHtml(job.task_name) + '</span></div>' +
                    '<div class="job-row"><span class="job-label">Priority</span><span class="job-value ' + priorityClass + '">' + job.priority + '</span></div>' +
                    '<div class="job-row"><span class="job-label">Cron</span><span class="job-value">' + escapeHtml(job.cron_time) + '</span></div>' +
                    '<div class="job-row"><span class="job-label">Duration</span><span class="timer" id="timer-' + workerId + '">-</span></div>' +
                    '</div>';
            } else {
                jobHtml = '<div class="job-info hidden"><p style="color: #666; text-align: center;">No active job</p></div>';
            }

            var lastCompletedTime = worker.last_completed ? formatTime(new Date(worker.last_completed)) : '-';

            return '<div class="worker-card ' + worker.status + '" data-worker-id="' + workerId + '">' +
                '<div class="worker-header">' +
                '<span class="worker-id">Worker #' + workerId + '</span>' +
                '<span class="worker-status ' + worker.status + '">' + worker.status + '</span>' +
                '</div>' +
                '<div class="job-row"><span class="job-label">Tasks Completed</span><span class="job-value">' + worker.tasks_completed + '</span></div>' +
                '<div class="job-row"><span class="job-label">Last Completed</span><span class="job-value">' + lastCompletedTime + '</span></div>' +
                jobHtml +
                '</div>';
        }

        function renderWorkers() {
            var grid = document.getElementById('workers-grid');
            var sortedIds = Object.keys(workersData).sort(function(a, b) { return parseInt(a) - parseInt(b); });
            var html = '';
            for (var i = 0; i < sortedIds.length; i++) {
                html += renderWorkerCard(sortedIds[i], workersData[sortedIds[i]]);
            }
            grid.innerHTML = html;
        }

        function renderEvents(events) {
            var list = document.getElementById('events-list');
            var html = '';
            for (var i = Math.max(0, events.length - 20); i < events.length; i++) {
                var event = events[i];
                html += '<div class="event-item">' +
                    '<span class="event-type ' + event.type + '">' + event.type.replace('_', ' ') + '</span>' +
                    '<span class="event-job">' + escapeHtml(event.job.job_name) + ' (Worker #' + event.worker_id + ')</span>' +
                    '<span class="event-time">' + formatTime(new Date(event.timestamp)) + '</span>' +
                    '</div>';
            }
            list.innerHTML = html;
        }

        function updateStats() {
            var idle = 0, processing = 0, totalCompleted = 0;
            for (var id in workersData) {
                var w = workersData[id];
                if (w.status === 'idle') idle++;
                else if (w.status === 'processing') processing++;
                totalCompleted += w.tasks_completed;
            }
            document.getElementById('idle-count').textContent = idle;
            document.getElementById('processing-count').textContent = processing;
            document.getElementById('total-completed').textContent = totalCompleted;
        }

        function updateTimers() {
            for (var id in workersData) {
                var worker = workersData[id];
                if (worker.status === 'processing' && worker.picked_at) {
                    var timerEl = document.getElementById('timer-' + id);
                    if (timerEl) {
                        var seconds = Math.floor((Date.now() - new Date(worker.picked_at).getTime()) / 1000);
                        timerEl.textContent = formatDuration(seconds);
                    }
                }
            }
        }

        function fetchInitialData() {
            fetch('/api/workers')
                .then(function(res) { return res.json(); })
                .then(function(data) {
                    workersData = data;
                    renderWorkers();
                    updateStats();
                })
                .catch(function(err) { console.error('Failed to fetch workers:', err); });

            fetch('/api/stats')
                .then(function(res) { return res.json(); })
                .then(function(data) {
                    eventsData = data.recent_events || [];
                    renderEvents(eventsData);
                })
                .catch(function(err) { console.error('Failed to fetch stats:', err); });
        }

        function connectSSE() {
            var eventSource = new EventSource('/api/events');

            eventSource.onmessage = function(event) {
                var data = JSON.parse(event.data);
                if (data.type === 'snapshot') {
                    workersData = data.workers || {};
                    renderWorkers();
                    updateStats();
                }
            };

            eventSource.onerror = function() {
                console.log('SSE connection lost, reconnecting...');
                eventSource.close();
                setTimeout(connectSSE, 3000);
            };
        }

        function startPolling() {
            setInterval(function() {
                fetch('/api/stats')
                    .then(function(res) { return res.json(); })
                    .then(function(data) {
                        eventsData = data.recent_events || [];
                        renderEvents(eventsData);
                        updateStats();
                    })
                    .catch(function(err) { console.error('Polling failed:', err); });
            }, 2000);

            setInterval(updateTimers, 1000);
        }

        // Initialize
        fetchInitialData();
        connectSSE();
        startPolling();
    </script>
    {{end}}
</body>
</html>
`
