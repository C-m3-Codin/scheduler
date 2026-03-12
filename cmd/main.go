package main

import (
	"context"
	"flag"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/c-m3-codin/gsched/config"
	"github.com/c-m3-codin/gsched/constants"
	"github.com/c-m3-codin/gsched/models"
	"github.com/c-m3-codin/gsched/monitor"
	"github.com/c-m3-codin/gsched/tasks"
	"github.com/c-m3-codin/gsched/ui"
	"github.com/c-m3-codin/gsched/utility"
	"github.com/c-m3-codin/gsched/worker"
	"github.com/confluentinc/confluent-kafka-go/kafka"
)

// Config struct for the scheduler
type Config struct {
	*utility.Config
	Producer *kafka.Producer
	JobTopic string
	Monitor  *monitor.MonitorState
}

var appConfig config.AppConfig

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{AddSource: true, Level: slog.LevelDebug}))
	slog.SetDefault(logger)

	appConfig = config.LoadConfig("config/kafka.properties")
	slog.Info("Application configuration loaded", "config", appConfig)

	role := flag.String("role", "scheduler", "Application role: 'scheduler' or 'worker'")
	flag.Parse()
	slog.Info("Application starting", "role", *role)

	if *role == "worker" {
		slog.Info("Starting in WORKER mode")
		worker.StartWorkers(appConfig)
		slog.Info("Worker mode finished.")
	} else if *role == "scheduler" {
		slog.Info("Starting in SCHEDULER mode")

		utConfig := utility.InitConfig()

		producer, err := utility.NewProducer(appConfig.BootstrapServers)
		if err != nil {
			slog.Error("Failed to create Kafka producer for scheduler", "error", err)
			os.Exit(1)
		}
		defer producer.Close()

		monitorEnabled := os.Getenv("MONITOR_ENABLED") == "true"
		monitorState := monitor.NewMonitorState(constants.JobProducerWorkerCount, monitorEnabled)

		schedulerConfig := Config{
			Config:   &utConfig,
			Producer: producer,
			JobTopic: appConfig.JobTopic,
			Monitor:  monitorState,
		}

		if monitorEnabled {
			uiHandler := ui.NewHandler(monitorState)
			mux := http.NewServeMux()
			uiHandler.RegisterRoutes(mux)

			serverPort := os.Getenv("MONITOR_PORT")
			if serverPort == "" {
				serverPort = "8080"
			}

			server := &http.Server{
				Addr:    ":" + serverPort,
				Handler: mux,
			}

			go func() {
				slog.Info("Monitoring UI available", "port", serverPort, "url", fmt.Sprintf("http://localhost:%s", serverPort))
				if err := server.ListenAndServe(); err != http.ErrServerClosed {
					slog.Error("HTTP server error", "error", err)
				}
			}()

			go func() {
				sigChan := make(chan os.Signal, 1)
				signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
				<-sigChan
				slog.Info("Shutting down...")
				monitorState.Shutdown()
				ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
				defer cancel()
				server.Shutdown(ctx)
				os.Exit(0)
			}()
		}

		schedulerConfig.WaitGroup.Add(1)
		go schedulerConfig.pulse()
		schedulerConfig.WaitGroup.Wait()
		slog.Info("Scheduler mode finished.")
	} else {
		slog.Error("Invalid role specified. Exiting.", "role", *role)
		os.Exit(1)
	}
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
			c.ScheduleUpdateCheck()
			manageTasks(scheduleFile.Jobs, c.Producer, c.JobTopic, c.Monitor)
		}
	}
}

func manageTasks(jobs []models.Job, producer *kafka.Producer, jobTopic string, monitor *monitor.MonitorState) {
	workersCount := constants.JobProducerWorkerCount
	inputChannel := make(chan models.Job, workersCount)
	go distirbuteTasks(inputChannel, jobs)
	for i := 0; i < workersCount; i++ {
		go taskProducers(inputChannel, i, producer, jobTopic, monitor)
	}
}

func distirbuteTasks(inputChannel chan models.Job, jobs []models.Job) {
	for i := 0; i < len(jobs); i++ {
		slog.Debug("Distributing job to input channel", "job_index", i, "job_name", jobs[i].JobName, "task_name", jobs[i].TaskName)
		inputChannel <- jobs[i]
	}
	close(inputChannel)
}

func taskProducers(inputChannel chan models.Job, workerNumber int, producer *kafka.Producer, jobTopic string, monitor *monitor.MonitorState) {
	for {
		select {
		case job, ok := <-inputChannel:
			if !ok {
				slog.Info("Input channel closed for worker, exiting", "worker_id", workerNumber)
				return
			}

			slog.Info("Worker picked job", "worker_id", workerNumber, "job_name", job.JobName, "task_name", job.TaskName)
			monitor.ReportTaskPicked(workerNumber, job)

			if matchCronExpression(job.CronTime, time.Now()) {
				task, err := tasks.GetTask(job.TaskName)
				if err != nil {
					slog.Error("Task not found for job", "worker_id", workerNumber, "task_name", job.TaskName, "job_name", job.JobName, "error", err)
					continue
				}

				if err := task.Execute(job.TaskParams); err != nil {
					slog.Error("Task execution failed for job", "worker_id", workerNumber, "task_name", job.TaskName, "job_name", job.JobName, "error", err)
					monitor.ReportTaskFailed(workerNumber, job, err)
					continue
				}

				slog.Debug("Task executed successfully, pushing to Kafka",
					"worker_id", workerNumber,
					"job_name", job.JobName,
					"task_name", job.TaskName,
					"topic", jobTopic)
				utility.PushToQueue(producer, jobTopic, job)
				monitor.ReportTaskCompleted(workerNumber, job)
			} else {
				slog.Debug("Cron expression did not match for job", "worker_id", workerNumber, "job_name", job.JobName, "task_name", job.TaskName, "cron_time", job.CronTime)
			}
		}
	}
}

func matchCronExpression(expression string, t time.Time) bool {
	fields := strings.Fields(expression)
	if len(fields) != 5 {
		return false
	}
	minute, hour, day, month, weekday := fields[0], fields[1], fields[2], fields[3], fields[4]
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
			return false
		}
		start, err1 := strconv.Atoi(rangeValues[0])
		end, err2 := strconv.Atoi(rangeValues[1])
		if err1 != nil || err2 != nil {
			return false
		}
		return value >= start && value <= end
	}
	v, err := strconv.Atoi(field)
	if err != nil {
		return false
	}
	return value == v
}
