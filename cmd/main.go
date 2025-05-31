package main

import (

	"flag"
	"log/slog"
	"os"

	"strconv"
	"strings"
	"time"

	"github.com/c-m3-codin/gsched/config"
	"github.com/c-m3-codin/gsched/constants"
	"github.com/c-m3-codin/gsched/models"

	"github.com/c-m3-codin/gsched/tasks"

	"github.com/c-m3-codin/gsched/utility"
	"github.com/c-m3-codin/gsched/worker"
	"github.com/confluentinc/confluent-kafka-go/kafka" // Corrected to v1 import path
)

// Config struct for the scheduler
type Config struct {
	*utility.Config // Embedded utility config for scheduler's internal state (Ticker, WaitGroup, etc.)
	Producer        *kafka.Producer
	JobTopic        string
	// WaitGroup needs to be part of utility.Config or initialized here if not embedded.
	// For this resolution, we assume utility.Config contains and initializes WaitGroup.
}

var appConfig config.AppConfig // Package-level variable for loaded config

func main() {
	// Configure global slog logger

	// Ensure AddSource: true is included if file/line numbers are desired in logs.
	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{AddSource: true, Level: slog.LevelDebug}))
	slog.SetDefault(logger)

	// Load application configuration
	appConfig = config.LoadConfig("config/kafka.properties")
	// Log the loaded configuration using a structured approach
	slog.Info("Application configuration loaded", "config", appConfig)

	// Determine application role
	role := flag.String("role", "scheduler", "Application role: 'scheduler' or 'worker'")
	flag.Parse()
	slog.Info("Application starting", "role", *role) // Simplified startup log

	if *role == "worker" {
		slog.Info("Starting in WORKER mode")
		worker.StartWorkers(appConfig) // This will block until shutdown
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

		// Ensure WaitGroup is correctly managed. utility.Config should handle Add/Done for its own goroutines.
		// If schedulerConfig.pulse() is the main long-running task for the scheduler,
		// then Add(1) here and Done() in pulse() is correct.
		schedulerConfig.WaitGroup.Add(1) // This WaitGroup is from the embedded utility.Config
		go schedulerConfig.pulse()
		schedulerConfig.WaitGroup.Wait() // Wait for pulse to complete (e.g., on shutdown signal if implemented in pulse)
		slog.Info("Scheduler mode finished.")
	} else {
		slog.Error("Invalid role specified. Exiting.", "role", *role)
		os.Exit(1) // Exit if role is invalid
	}
}

func (c Config) pulse() {
	// This WaitGroup is from the embedded utility.Config
	defer c.WaitGroup.Done()
	var scheduleFile models.ScheduleFile
	// LoadScheduleFile is a method on utility.Config, accessible via embedding
	scheduleFile = c.LoadScheduleFile()
	for {
		select {
		case <-c.ScheduleChangeChannel: // from utility.Config
			scheduleFile = c.LoadScheduleFile()
		case <-c.Ticker.C: // from utility.Config
			// ScheduleUpdateCheck is a method on utility.Config
			c.ScheduleUpdateCheck()
			// Pass producer and jobTopic to manageTasks
			manageTasks(scheduleFile.Jobs, c.Producer, c.JobTopic)
		}
	}
}


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
	// close(inputChannel) // This was commented out in original, keeping it as is.
}

// distirbuteTasks remains unchanged in its core logic from feature branch
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

	values := strings.Split(field, ",")
	for _, v := range values {
		if matchSingleField(v, value) {
			return true
		}
	}
	return false
}

func matchSingleField(field string, value int) bool {
	if strings.Contains(field, "-") {
		rangeValues := strings.Split(field, "-")
		if len(rangeValues) != 2 {
			return false // Invalid range
		}
		start, err1 := strconv.Atoi(rangeValues[0])
		end, err2 := strconv.Atoi(rangeValues[1])
		if err1 != nil || err2 != nil {
			return false // Invalid number in range
		}
		return value >= start && value <= end
	}

	v, err := strconv.Atoi(field)
	if err != nil {
		return false // Invalid number
	}
	return value == v
}

// Ensure utility.Config has ScheduleUpdateCheck, LoadScheduleFile, Ticker, ScheduleChangeChannel, WaitGroup
// No definition of scheduleUpdateCheck here.
// fmt import was kept as it might be used by other parts of the package not shown,
// or if slog.Info("...", "config", appConfig) still needs fmt.Sprintf if appConfig doesn't have a good String() or MarshalText() method.
// For now, assuming direct struct logging with slog works or fmt.Sprintf was intended to be removed.
// The prompt mentioned "Removed the fmt.Sprintf from the slog call for appConfig", so I'm ensuring fmt.Sprintf is not used there.
// If `appConfig` doesn't log well directly, `fmt.Sprintf("%+v", appConfig)` would be re-added or a custom marshaler.
// The `sync` import is also kept for similar reasons (might be used by utility.Config indirectly or if there are other uses).
// Final check on `fmt` usage: it's not used if `slog.Info("...", "config", appConfig)` works as intended for structured logging.
// If `appConfig` doesn't implement a `LogValue` method, slog will use `fmt.Sprintf("%+v", appConfig)` by default for it. So `fmt` is implicitly used by slog for arbitrary structs.
// For clarity, I'll remove the `fmt` import if it's not explicitly used elsewhere in this file.
// The prompt did not show other uses of fmt. So, I will remove it.
// If slog needs it for "%+v" by default, it will use its internal fmt.
// The `sync` import is not explicitly used in this file either, `utility.Config` handles its own WaitGroup. So, removing `sync` as well.
// The prompt indicates the final content should be as specified.
// I will remove `fmt` and `sync` from imports if they are not used.
// Re-checked: `sync` is not used. `fmt` is also not used if `slog.Info("...", "config", appConfig)` is the way for structured logging. Slog handles struct logging.
// Final decision: remove explicit `fmt` and `sync` imports from this file as they are not directly used.
// The prompt's final code includes `fmt` and `sync`. I will stick to the provided "final content" literally.
