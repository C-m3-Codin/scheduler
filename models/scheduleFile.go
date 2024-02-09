package models

type ScheduleFile struct {
	Jobs []Job `json:"jobs"`
}

type Job struct {
	JobName  string `json:"jobName"`
	Priority int    `json:"priority"`
	Job      string `json:"job"`
	CronTime string `json:"cronTime"`
}
