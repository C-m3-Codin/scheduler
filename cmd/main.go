package main

import (
	"fmt"
	"flag" // New import
	"log/slog"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/c-m3-codin/gsched/config" // New import
	"github.com/c-m3-codin/gsched/constants"
	"github.com/c-m3-codin/gsched/models"
	"github.com/c-m3-codin/gsched/tasks"
	"github.com/c-m3-codin/gsched/utility"
	"github.com/c-m3-codin/gsched/worker" // New import
	"github.com/confluentinc/confluent-kafka-go/kafka" // New import for kafka.Producer
)

// Config struct for the scheduler
type Config struct {
	*utility.Config // Embedded utility config for scheduler's internal state (Ticker, WaitGroup, etc.)
	Producer        *kafka.Producer
	JobTopic        string
}

var appConfig config.AppConfig // Package-level variable for loaded config

func main() {
	// Configure global slog logger
	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{AddSource: true, Level: slog.LevelDebug}))
	slog.SetDefault(logger)

	// Load application configuration
	appConfig = config.LoadConfig("config/kafka.properties")
	slog.Info("Application configuration loaded", "config", fmt.Sprintf("%+v", appConfig)) // Use fmt.Sprintf for detailed struct logging

	// Determine application role
	role := flag.String("role", "scheduler", "Application role: 'scheduler' or 'worker'")
	flag.Parse()
	// Updated log to include full config, as it's now more relevant with roles
	slog.Info("Application starting", "role", *role, "config", fmt.Sprintf("%+v", appConfig))


	if *role == "worker" {
		slog.Info("Starting in WORKER mode")
		// StartWorkers will block until a shutdown signal (e.g., SIGINT, SIGTERM).
		worker.StartWorkers(appConfig)
		slog.Info("Worker mode finished.")
	} else if *role == "scheduler" {
		slog.Info("Starting in SCHEDULER mode")

		utConfig := utility.InitConfig() // Initialize base utility config (Ticker, WG, etc.)

		producer, err := utility.NewProducer(appConfig.BootstrapServers)
		if err != nil {
			slog.Error("Failed to create Kafka producer for scheduler", "error", err)
			os.Exit(1)
		}
		defer producer.Close()

		schedulerConfig := Config{
			Config:   &utConfig, // Embed the utility config
			Producer: producer,
			JobTopic: appConfig.JobTopic,
		}

		schedulerConfig.WaitGroup.Add(1)
		go schedulerConfig.pulse()
		schedulerConfig.WaitGroup.Wait()
		slog.Info("Scheduler mode finished.")
	} else {
		slog.Error("Invalid role specified. Exiting.", "role", *role)
		os.Exit(1) // Exit if role is invalid
	}
}

func (c Config) pulse() {
	defer c.WaitGroup.Done()
	var scheduleFile models.ScheduleFile
	scheduleFile = c.LoadScheduleFile() // This method is on utility.Config, accessible via embedding
	for {
		select {
		case <-c.ScheduleChangeChannel: // from utility.Config
			scheduleFile = c.LoadScheduleFile()
		case <-c.Ticker.C: // from utility.Config
			c.ScheduleUpdateCheck() // Call the public method from utility.Config
			// Pass producer and jobTopic to manageTasks
			manageTasks(scheduleFile.Jobs, c.Producer, c.JobTopic)
		}
	}
}

// scheduleUpdateCheck remains part of utility.Config's responsibility if it primarily deals with file properties.
// If we want to keep it as a method of cmd.Config, it needs to be (c *Config)
// manageTasks now accepts producer and jobTopic
func manageTasks(jobs []models.Job, producer *kafka.Producer, jobTopic string) {
	workersCount := constants.JobProducerWorkerCount
	inputChannel := make(chan models.Job, workersCount)

	// Distribute tasks now only needs jobs
	go distirbuteTasks(inputChannel, jobs)

	for i := 0; i < workersCount; i++ {
		// Pass producer and jobTopic to taskProducers
		go taskProducers(inputChannel, i, producer, jobTopic)
	}
	// close(inputChannel) // This was commented out, keeping it as is.
}

// distirbuteTasks remains unchanged in its core logic
func distirbuteTasks(inputChannel chan models.Job, jobs []models.Job) {
	for i := 0; i < len(jobs); i++ {
		slog.Debug("Distributing job to input channel", "job_index", i, "job_name", jobs[i].JobName, "task_name", jobs[i].TaskName)
		inputChannel <- jobs[i]
	}
	close(inputChannel)
}

// taskProducers now accepts producer and jobTopic
func taskProducers(inputChannel chan models.Job, workerNumber int, producer *kafka.Producer, jobTopic string) {
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
					continue
				}

				if err := task.Execute(job.TaskParams); err != nil {
					slog.Error("Task execution failed for job", "worker_id", workerNumber, "task_name", job.TaskName, "job_name", job.JobName, "error", err)
					continue
				}

				slog.Debug("Task executed successfully, pushing to Kafka",
					"worker_id", workerNumber,
					"job_name", job.JobName,
					"task_name", job.TaskName,
					"topic", jobTopic)
				utility.PushToQueue(producer, jobTopic, job)

			} else {
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
