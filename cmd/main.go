package main

import (
	"fmt"
	"log/slog" // New import
	"os"       // New import
	"strconv"
	"strings"
	"time"

	"github.com/c-m3-codin/gsched/constants"
	"github.com/c-m3-codin/gsched/models"
	"github.com/c-m3-codin/gsched/tasks" // New import
	"github.com/c-m3-codin/gsched/utility"
)

type Config struct {
	*utility.Config
}

func main() {
	// Configure global slog logger
	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{AddSource: true, Level: slog.LevelDebug}))
	slog.SetDefault(logger)

	slog.Debug("Application starting")

	utconfig := utility.InitConfig()
	config := Config{&utconfig}
	config.WaitGroup.Add(1)
	// go changeEvent(config.ScheduleChangeChannel)
	go config.pulse()
	config.WaitGroup.Wait()

}

func (c Config) pulse() {
	defer c.WaitGroup.Done()
	var scheduleFile models.ScheduleFile
	scheduleFile = c.LoadScheduleFile()
	for {
		select {

		case <-c.ScheduleChangeChannel:
			scheduleFile = c.LoadScheduleFile()
		case <-c.Ticker.C:
			c.scheduleUpdateCheck()
			manageTasks(scheduleFile.Jobs)

		}

	}
}

func (c *Config) scheduleUpdateCheck() {

	hash, err := utility.Filemd5sum(c.ScheduleFile)
	if err != nil {
		slog.Error("Failed to calculate file hash", "error", err)
		return
	}
	slog.Debug("Schedule file hash check", "previous_hash", c.ScheduleFileHash, "current_hash", hash)
	if c.ScheduleFileHash != hash {
		c.ScheduleFileHash = hash
		c.ScheduleChangeChannel <- true
	}
}

func manageTasks(jobs []models.Job) {

	workersCount := constants.JobProducerWorkerCount
	inputChannel := make(chan models.Job, workersCount)
	go distirbuteTasks(inputChannel, jobs)

	for i := 0; i < workersCount; i++ {
		go taskProducers(inputChannel, i)
	}
	// close(inputChannel)

}

func distirbuteTasks(inputChannel chan models.Job, jobs []models.Job) {

	for i := 0; i < len(jobs); i++ {
		slog.Debug("Distributing job to input channel", "job_index", i, "job_name", jobs[i].JobName, "task_name", jobs[i].TaskName)
		inputChannel <- jobs[i]

	}
	close(inputChannel)

}

func taskProducers(inputChannel chan models.Job, workerNumber int) {
	for {
		select {
		case job, ok := <-inputChannel:
			if !ok {
				slog.Info("Input channel closed for worker, exiting", "worker_id", workerNumber)
				return
			}

			slog.Info("Worker picked job", "worker_id", workerNumber, "job_name", job.JobName, "task_name", job.TaskName)

			if matchCronExpression(job.CronTime, time.Now()) {
				task, err := tasks.GetTask(job.TaskName)
				if err != nil {
					slog.Error("Task not found for job", "worker_id", workerNumber, "task_name", job.TaskName, "job_name", job.JobName, "error", err)
					continue // Skip to the next job
				}

				if err := task.Execute(job.TaskParams); err != nil {
					slog.Error("Task execution failed for job", "worker_id", workerNumber, "task_name", job.TaskName, "job_name", job.JobName, "error", err)
					continue // Skip to the next job
				}

				// Consider adding a debug/info log here for successful execution before pushing to queue
				utility.PushToQueue(job) // Push original job struct to Kafka

			} else {
				// This log can be very frequent if jobs are checked often.
				// slog.Debug("Cron expression did not match for job", "worker_id", workerNumber, "job_name", job.JobName, "task_name", job.TaskName, "cron_time", job.CronTime)
			}
		}
	}
}

func matchCronExpression(expression string, t time.Time) bool {
	fields := strings.Fields(expression)
	if len(fields) != 5 {
		// Invalid cron expression
		return false
	}

	// Parse the cron fields
	minute, hour, day, month, weekday := fields[0], fields[1], fields[2], fields[3], fields[4]

	// Check if the current time matches the cron expression
	return matchField(minute, t.Minute()) &&
		matchField(hour, t.Hour()) &&
		matchField(day, t.Day()) &&
		matchField(month, int(t.Month())) &&
		matchField(weekday, int(t.Weekday()))
}

func matchField(field string, value int) bool {
	if field == "*" {
		return true
	}

	// Split the field into individual values
	values := strings.Split(field, ",")

	// Check if the value matches any of the individual values
	for _, v := range values {
		if matchSingleField(v, value) {
			return true
		}
	}

	return false
}

func matchSingleField(field string, value int) bool {
	// Check if the field contains a range
	if strings.Contains(field, "-") {
		rangeValues := strings.Split(field, "-")
		if len(rangeValues) != 2 {
			return false
		}
		start, err1 := strconv.Atoi(rangeValues[0])
		end, err2 := strconv.Atoi(rangeValues[1])
		if err1 != nil || err2 != nil {
			return false
		}
		return value >= start && value <= end
	}

	// Check if the field is a single value
	v, err := strconv.Atoi(field)
	if err != nil {
		return false
	}

	return value == v
}
