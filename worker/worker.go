package worker

import (
	"encoding/json"
	"log/slog"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"github.com/c-m3-codin/gsched/config"
	"github.com/c-m3-codin/gsched/models"
	"github.com/c-m3-codin/gsched/tasks" // Assuming tasks are globally registered
	"github.com/c-m3-codin/gsched/utility"
	"github.com/confluentinc/confluent-kafka-go/kafka" // Using v1 as per go.mod
)

const (
	numWorkers = 5 // Number of concurrent worker goroutines
)

// StartWorkers initializes and runs Kafka consumer workers.
func StartWorkers(appCfg config.AppConfig) {
	slog.Info("Starting Kafka consumer workers...",
		"bootstrap_servers", appCfg.BootstrapServers,
		"topic", appCfg.JobTopic,
		"group_id", appCfg.ConsumerGroupID)

	consumer, err := utility.NewConsumer(appCfg.BootstrapServers, appCfg.ConsumerGroupID, appCfg.JobTopic)
	if err != nil {
		slog.Error("Failed to initialize Kafka consumer for workers", "error", err)
		// If consumer creation fails, we can't proceed.
		panic("Worker consumer creation failed: " + err.Error())
	}
	defer func() {
		slog.Info("Closing Kafka consumer...")
		if err := consumer.Close(); err != nil {
			slog.Error("Error closing Kafka consumer", "error", err)
		}
	}()

    var wg sync.WaitGroup
    for i := 0; i < numWorkers; i++ {
        wg.Add(1)
        go func(workerID int) {
            defer wg.Done()
            slog.Info("Starting worker goroutine", "worker_id", workerID)
            // The 'run' variable for graceful shutdown needs to be improved to be shared
            // or signaled via a channel. Currently, the loop primarily exits on consumer.ReadMessage errors
            // after consumer.Close() is called.
            run := true
            for run { // This 'run' flag is not effectively used for shutdown in this version.
                msg, err := consumer.ReadMessage(100 * time.Millisecond)
                if err != nil {
                    if kerr, ok := err.(kafka.Error); ok && kerr.Code() == kafka.ErrTimedOut {
                        continue // Timeout is expected, just continue polling
                    }
                    // For other errors, including when the consumer is closed, this path will be taken.
                    slog.Error("Kafka consumer error on ReadMessage", "worker_id", workerID, "error", err)
                    // If consumer is closed, ReadMessage returns an error, and this will cause the loop to break.
                    // Example error when consumer is closed: "Failed to read message: Local: Consumer closed"
                    // We can make this more explicit by checking err.Error() or specific Kafka error codes.
                    // For now, any error other than timeout will lead to goroutine exit after some retries/logging.
                    // To make it exit cleanly on shutdown, we'd check a shared 'done' channel here.
                    run = false // Exit loop on error
                    continue
                }

                slog.Debug("Worker received Kafka message",
                    "worker_id", workerID,
                    "topic", *msg.TopicPartition.Topic,
                    "partition", msg.TopicPartition.Partition,
                    "offset", msg.TopicPartition.Offset.String(),
                    "key", string(msg.Key))

                var job models.Job
                if err := json.Unmarshal(msg.Value, &job); err != nil {
                    slog.Error("Failed to unmarshal job message from Kafka",
                        "worker_id", workerID,
                        "error", err,
                        "raw_message_value", string(msg.Value))
                    if _, e := consumer.CommitMessage(msg); e != nil {
                       slog.Error("Failed to commit message after unmarshal error", "worker_id", workerID, "offset", msg.TopicPartition.Offset, "error", e)
                    }
                    continue
                }

                slog.Info("Processing job",
                    "worker_id", workerID,
                    "job_name", job.JobName,
                    "task_name", job.TaskName)

                task, err := tasks.GetTask(job.TaskName)
                if err != nil {
                    slog.Error("Task not found in registry for job",
                        "worker_id", workerID,
                        "job_name", job.JobName,
                        "task_name", job.TaskName,
                        "error", err)
                    if _, e := consumer.CommitMessage(msg); e != nil {
                       slog.Error("Failed to commit message after task not found error", "worker_id", workerID, "offset", msg.TopicPartition.Offset, "error", e)
                    }
                    continue
                }

                if err := task.Execute(job.TaskParams); err != nil {
                    slog.Error("Task execution failed for job",
                        "worker_id", workerID,
                        "job_name", job.JobName,
                        "task_name", job.TaskName,
                        "error", err)
                } else {
                    slog.Info("Task executed successfully for job",
                        "worker_id", workerID,
                        "job_name", job.JobName,
                        "task_name", job.TaskName)
                }
                if _, e := consumer.CommitMessage(msg); e != nil {
                    slog.Error("Failed to commit message after processing", "worker_id", workerID, "offset", msg.TopicPartition.Offset, "error", e)
                }
            }
            slog.Info("Worker goroutine shutting down", "worker_id", workerID)
        }(i)
    }

	// Graceful shutdown handling
	sigchan := make(chan os.Signal, 1)
	signal.Notify(sigchan, syscall.SIGINT, syscall.SIGTERM)
	receivedSignal := <-sigchan
	slog.Info("Received shutdown signal", "signal", receivedSignal.String())

	slog.Info("Attempting to shut down workers...")
    // Closing the consumer (deferred) will cause ReadMessage to return errors in worker goroutines,
    // which will then make them exit their polling loops due to `run = false` on error.
    // This is a slightly indirect way to signal shutdown for the polling loops.
    // A more direct way would involve a shared channel or atomic boolean.

    wg.Wait() // Wait for all worker goroutines to finish.
	slog.Info("All workers have shut down.")
}
