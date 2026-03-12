# Feature: Worker Task Monitoring UI

## Overview

Expose a real-time UI dashboard to monitor worker activity, showing which worker is processing which task, when it was picked up, and other relevant metrics.

## Requirements

### Core Features

- [ ] **Worker Status Panel**
  - Show all 5 workers (based on `JobProducerWorkerCount`)
  - Display current status: `Idle` / `Processing`
  - Show worker ID/number

- [ ] **Task Information Display**
  - Job name (`jobName`)
  - Job payload (`job`)
  - Priority level
  - Cron expression that triggered it

- [ ] **Timing Information**
  - Task pickup timestamp
  - Processing duration
  - Time since last task completed

- [ ] **Real-time Updates**
  - WebSocket or Server-Sent Events (SSE) for live updates
  - Auto-refresh dashboard without page reload

- [ ] **Historical Metrics** (Stretch Goal)
  - Tasks completed per worker
  - Average processing time
  - Success/failure rate

### Technical Implementation

#### Backend Changes

1. **Worker State Tracking**
   - Create a shared state structure to track worker activity
   - Use mutex-protected map or channels for thread-safe updates

2. **Event Emission**
   - Emit events on: task picked, task completed, task failed
   - Include timestamps and worker ID

3. **HTTP Server**
   - Add lightweight HTTP server (e.g., `net/http` or `gin`)
   - Expose endpoints:
     - `GET /` - Dashboard HTML
     - `GET /api/workers` - Current worker status (JSON)
     - `GET /api/events` - SSE stream or WebSocket

4. **Configuration**
   - Add flag/env var to enable/disable UI (default: disabled)
   - Configurable port (default: `8080` or similar)

#### Frontend Changes

1. **Dashboard Layout**
   - Worker cards/grid showing status
   - Color coding: Green (processing), Gray (idle), Red (error)
   - Live timestamp updates

2. **Technology Options**
   - Simple HTML/CSS/JS (minimal dependencies)
   - Or React/Vue if more interactivity needed
   - Consider HTMX for simplicity

### File Structure

```
scheduler/
├── cmd/
│   └── main.go              # Add HTTP server initialization
├── ui/                      # NEW: UI assets and handlers
│   ├── dashboard.html
│   ├── static/
│   │   ├── style.css
│   │   └── script.js
│   └── handler.go           # HTTP handlers
├── monitor/                 # NEW: Worker state tracking
│   └── state.go             # Shared state, events, mutex
└── tasks/
    └── feature-worker-monitoring-ui.md  # This file
```

### Data Structures

```go
// monitor/state.go
type WorkerStatus struct {
    WorkerID      int       `json:"worker_id"`
    Status        string    `json:"status"` // "idle", "processing", "error"
    CurrentJob    *Job      `json:"current_job,omitempty"`
    PickedAt      time.Time `json:"picked_at,omitempty"`
    TasksCompleted int      `json:"tasks_completed"`
    LastCompleted time.Time `json:"last_completed,omitempty"`
}

type MonitorState struct {
    mu       sync.RWMutex
    Workers  map[int]*WorkerStatus
    Events   chan WorkerEvent
}

type WorkerEvent struct {
    Type      string    `json:"type"` // "task_picked", "task_completed", "task_failed"
    WorkerID  int       `json:"worker_id"`
    Job       Job       `json:"job"`
    Timestamp time.Time `json:"timestamp"`
}
```

### Integration Points

1. **Modify `taskProducers`** in `cmd/main.go`:
   - Report when task is picked
   - Report when task is completed/failed

2. **Initialize Monitor** in `utility/init.go`:
   - Add monitor state to Config struct
   - Start event listener goroutine

3. **Start HTTP Server** in `main.go`:
   - Non-blocking server startup
   - Graceful shutdown handling

## Acceptance Criteria

- [ ] UI accessible at configurable port (default `:8080`)
- [ ] Shows all 5 workers with real-time status
- [ ] Displays task details when worker is processing
- [ ] Shows pickup timestamp and duration
- [ ] Updates automatically without page refresh
- [ ] Does not impact existing job dispatching performance
- [ ] Can be disabled via configuration

## Dependencies

- Go standard library (`net/http`, `html/template`)
- Optional: `github.com/gorilla/websocket` for WebSocket support
- No frontend framework required (keep it simple)

## Notes

- Keep the UI lightweight to avoid impacting job dispatching
- Consider rate-limiting UI updates if events are frequent
- Ensure thread-safety when updating worker state from multiple goroutines
- The feature should be opt-in (disabled by default in production)
