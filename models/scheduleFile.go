package models

type ScheduleFile struct {
	Jobs []Job `json:"jobs"`
}

type Job struct {
	JobName    string                 `json:"jobName"` // Remains for descriptive purposes
	Priority   int                    `json:"priority"`
	TaskName   string                 `json:"taskName"` // Renamed from Job, holds the identifier of the task
	TaskParams map[string]interface{} `json:"taskParams,omitempty"` // Parameters for the task
	CronTime   string                 `json:"cronTime"`
}
