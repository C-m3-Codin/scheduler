package monitor

import (
	"sync"
	"time"

	"github.com/c-m3-codin/gsched/models"
)

// WorkerStatus represents the current status of a worker
type WorkerStatus struct {
	WorkerID       int        `json:"worker_id"`
	Status         string     `json:"status"` // "idle", "processing", "error"
	CurrentJob     *JobInfo   `json:"current_job,omitempty"`
	PickedAt       *time.Time `json:"picked_at,omitempty"`
	TasksCompleted int        `json:"tasks_completed"`
	LastCompleted  *time.Time `json:"last_completed,omitempty"`
}

// JobInfo contains simplified job information for UI display
type JobInfo struct {
	JobName    string `json:"job_name"`
	TaskName   string `json:"task_name"`
	Priority   int    `json:"priority"`
	CronTime   string `json:"cron_time"`
}

// WorkerEvent represents an event emitted by a worker
type WorkerEvent struct {
	Type      string    `json:"type"` // "task_picked", "task_completed", "task_failed"
	WorkerID  int       `json:"worker_id"`
	Job       JobInfo   `json:"job"`
	Timestamp time.Time `json:"timestamp"`
}

// MonitorState holds the shared state for all workers
type MonitorState struct {
	mu             sync.RWMutex
	Workers        map[int]*WorkerStatus
	Events         chan WorkerEvent
	RecentEvents   []WorkerEvent
	Enabled        bool
	subscribers    []chan<- WorkerEvent
	subscribersMu  sync.Mutex
}

// NewMonitorState creates a new monitor state instance
func NewMonitorState(workerCount int, enabled bool) *MonitorState {
	ms := &MonitorState{
		Workers:      make(map[int]*WorkerStatus),
		Events:       make(chan WorkerEvent, 100),
		RecentEvents: make([]WorkerEvent, 0, 100),
		Enabled:      enabled,
		subscribers:  make([]chan<- WorkerEvent, 0),
	}

	// Initialize worker statuses
	for i := 0; i < workerCount; i++ {
		ms.Workers[i] = &WorkerStatus{
			WorkerID: i,
			Status:   "idle",
		}
	}

	// Start event processor
	go ms.processEvents()

	return ms
}

// processEvents handles events and broadcasts to subscribers
func (ms *MonitorState) processEvents() {
	for event := range ms.Events {
		ms.mu.Lock()

		// Update worker status based on event
		if worker, ok := ms.Workers[event.WorkerID]; ok {
			switch event.Type {
			case "task_picked":
				worker.Status = "processing"
				jobInfo := event.Job
				worker.CurrentJob = &jobInfo
				pickedAt := event.Timestamp
				worker.PickedAt = &pickedAt

			case "task_completed", "task_failed":
				worker.Status = "idle"
				worker.CurrentJob = nil
				worker.PickedAt = nil
				completedAt := event.Timestamp
				worker.LastCompleted = &completedAt

				if event.Type == "task_completed" {
					worker.TasksCompleted++
				}
			}
		}

		// Store recent events (keep last 100)
		ms.RecentEvents = append(ms.RecentEvents, event)
		if len(ms.RecentEvents) > 100 {
			ms.RecentEvents = ms.RecentEvents[1:]
		}

		ms.mu.Unlock()

		// Broadcast to subscribers
		ms.broadcastEvent(event)
	}
}

// broadcastEvent sends event to all SSE subscribers
func (ms *MonitorState) broadcastEvent(event WorkerEvent) {
	ms.subscribersMu.Lock()
	defer ms.subscribersMu.Unlock()

	for _, ch := range ms.subscribers {
		select {
		case ch <- event:
		default:
			// Channel full, skip
		}
	}
}

// GetWorkerStatus returns the status of a specific worker
func (ms *MonitorState) GetWorkerStatus(workerID int) (*WorkerStatus, bool) {
	if !ms.Enabled {
		return nil, false
	}

	ms.mu.RLock()
	defer ms.mu.RUnlock()

	worker, ok := ms.Workers[workerID]
	if !ok {
		return nil, false
	}

	// Return a copy to avoid race conditions
	copy := *worker
	if worker.CurrentJob != nil {
		jobCopy := *worker.CurrentJob
		copy.CurrentJob = &jobCopy
	}
	if worker.PickedAt != nil {
		timeCopy := *worker.PickedAt
		copy.PickedAt = &timeCopy
	}
	if worker.LastCompleted != nil {
		timeCopy := *worker.LastCompleted
		copy.LastCompleted = &timeCopy
	}

	return &copy, true
}

// GetAllWorkers returns all worker statuses
func (ms *MonitorState) GetAllWorkers() map[int]*WorkerStatus {
	if !ms.Enabled {
		return make(map[int]*WorkerStatus)
	}

	ms.mu.RLock()
	defer ms.mu.RUnlock()

	result := make(map[int]*WorkerStatus)
	for id, worker := range ms.Workers {
		copy := *worker
		if worker.CurrentJob != nil {
			jobCopy := *worker.CurrentJob
			copy.CurrentJob = &jobCopy
		}
		if worker.PickedAt != nil {
			timeCopy := *worker.PickedAt
			copy.PickedAt = &timeCopy
		}
		if worker.LastCompleted != nil {
			timeCopy := *worker.LastCompleted
			copy.LastCompleted = &timeCopy
		}
		result[id] = &copy
	}

	return result
}

// GetRecentEvents returns recent worker events
func (ms *MonitorState) GetRecentEvents() []WorkerEvent {
	if !ms.Enabled {
		return []WorkerEvent{}
	}

	ms.mu.RLock()
	defer ms.mu.RUnlock()

	result := make([]WorkerEvent, len(ms.RecentEvents))
	copy(result, ms.RecentEvents)
	return result
}

// ReportTaskPicked reports that a worker has picked up a task
func (ms *MonitorState) ReportTaskPicked(workerID int, job models.Job) {
	if !ms.Enabled {
		return
	}

	ms.Events <- WorkerEvent{
		Type: "task_picked",
		Job: JobInfo{
			JobName:  job.JobName,
			TaskName: job.TaskName,
			Priority: job.Priority,
			CronTime: job.CronTime,
		},
		WorkerID:  workerID,
		Timestamp: time.Now(),
	}
}

// ReportTaskCompleted reports that a worker has completed a task
func (ms *MonitorState) ReportTaskCompleted(workerID int, job models.Job) {
	if !ms.Enabled {
		return
	}

	ms.Events <- WorkerEvent{
		Type: "task_completed",
		Job: JobInfo{
			JobName:  job.JobName,
			TaskName: job.TaskName,
			Priority: job.Priority,
			CronTime: job.CronTime,
		},
		WorkerID:  workerID,
		Timestamp: time.Now(),
	}
}

// ReportTaskFailed reports that a worker failed to process a task
func (ms *MonitorState) ReportTaskFailed(workerID int, job models.Job, err error) {
	if !ms.Enabled {
		return
	}

	ms.Events <- WorkerEvent{
		Type: "task_failed",
		Job: JobInfo{
			JobName:  job.JobName,
			TaskName: job.TaskName,
			Priority: job.Priority,
			CronTime: job.CronTime,
		},
		WorkerID:  workerID,
		Timestamp: time.Now(),
	}
}

// Subscribe creates a new event subscription for SSE
func (ms *MonitorState) Subscribe() <-chan WorkerEvent {
	if !ms.Enabled {
		ch := make(chan WorkerEvent)
		close(ch)
		return ch
	}

	ch := make(chan WorkerEvent, 100)
	ms.subscribersMu.Lock()
	ms.subscribers = append(ms.subscribers, ch)
	ms.subscribersMu.Unlock()

	return ch
}

// Unsubscribe removes an event subscription
func (ms *MonitorState) Unsubscribe(ch chan<- WorkerEvent) {
	ms.subscribersMu.Lock()
	defer ms.subscribersMu.Unlock()

	for i, sub := range ms.subscribers {
		if sub == ch {
			ms.subscribers = append(ms.subscribers[:i], ms.subscribers[i+1:]...)
			close(ch)
			return
		}
	}
}

// Shutdown closes the event channel and cleans up
func (ms *MonitorState) Shutdown() {
	close(ms.Events)
	ms.subscribersMu.Lock()
	for _, ch := range ms.subscribers {
		close(ch)
	}
	ms.subscribersMu.Unlock()
}
