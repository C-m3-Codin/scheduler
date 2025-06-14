# Job Dispatcher

This repository contains a job dispatcher written in Go that dispatches jobs to a Kafka queue based on a schedule defined in a configuration file. The dispatcher reads the schedule, monitors changes, and processes jobs according to their cron expressions.

## Features


- **Dual Role Operation**: Can run as a `scheduler` (producing jobs to Kafka based on a schedule) or as a `worker` (consuming and executing jobs from Kafka), configurable via a command-line flag.
- **Scheduler role: Executes tasks based on cron expressions. If execution is successful, the original job details are then dispatched to a Kafka queue.**
- **Kafka Integration**: The scheduler queues jobs to Kafka *after* successful local execution. Workers consume jobs from this queue.
- **Custom Task Execution (Worker Role)**: Workers process jobs that have been validated and queued by the scheduler, executing tasks based on `taskName` specified in the job. The system supports registration of custom task implementations.
- **Configuration File Monitoring (Scheduler Role)**: Monitors the schedule configuration file for any changes. If a change is detected (based on MD5 hash comparison), the schedule is reloaded automatically.
- **Concurrent Task Producers (Scheduler Role)**: Utilizes multiple goroutines to concurrently check job schedules and produce tasks to the Kafka queue.
- **Concurrent Worker Goroutines (Worker Role)**: Spawns multiple goroutines to consume and process jobs from Kafka concurrently.
- **Externalized Kafka Configuration**: Kafka connection details (`bootstrap.servers`, `job.topic`, `consumer.group.id`) are configured via a `config/kafka.properties` file, with sensible defaults.

## Configuration

The application uses a properties file located at `config/kafka.properties` to configure Kafka connection details and other parameters. If this file is not found at startup, or if specific keys are missing, the application will use default values and log a warning.

The following keys can be configured in `config/kafka.properties`:

-   `bootstrap.servers`: A comma-separated list of Kafka broker addresses (e.g., `localhost:9092,localhost:9093`).
    -   Default: `"localhost:29092"` (Note: The default in constants might differ from the example properties file, ensure consistency or document the specific default used on fallback).
-   `job.topic`: The Kafka topic where jobs are produced by the scheduler and consumed by workers.
    -   Default: `"JobQueue"`
-   `consumer.group.id`: The Kafka consumer group ID used by workers when consuming jobs. This allows multiple worker instances to share the load.
    -   Default: `"gsched-default-group"`

**Example `config/kafka.properties`:**
```properties
# Kafka Broker Addresses
bootstrap.servers=localhost:29092,localhost:29093

# Topic for Job Queue
job.topic=JobQueue

# Consumer Group ID for Workers
consumer.group.id=gsched-worker-group
```
Make sure this file exists at the path `config/kafka.properties` relative to the application's working directory, or adjust the path in `cmd/main.go` where `config.LoadConfig()` is called if you prefer a different location.


### Prerequisites

- Go (version 1.21 or higher)
- Kafka
- Zookeeper (for Kafka coordination)

## Getting Started

This section will guide you through setting up and running the Job Dispatcher project.

### 1. Set Up Go Environment

Ensure you have Go installed on your system. This project uses Go version `1.21` as specified in the `go.mod` file. You can download Go from the [official Go website](https://golang.org/dl/).

Verify your Go installation:
```bash
go version
```

### 2. Configure and Run Kafka & Zookeeper

The application requires Kafka for message queuing and Zookeeper for Kafka coordination. A `docker-compose.yaml` file is provided in the `docker/` directory to easily set up these services.

**Prerequisites for this step:**
- Docker ([Install Docker](https://docs.docker.com/get-docker/))
- Docker Compose ([Install Docker Compose](https://docs.docker.com/compose/install/))

To start Kafka and Zookeeper:
1. Navigate to the `docker` directory:
   ```bash
   cd docker
   ```
2. Run Docker Compose:
   ```bash
   docker-compose up -d
   ```
This will start Zookeeper (port `2181`), two Kafka brokers (ports `9092` and `9093`), and Kafka UI (port `8085`). You can access the Kafka UI by navigating to `http://localhost:8085` in your web browser.

To stop the services:
```bash
docker-compose down
```

### 3. Build and Run the Job Dispatcher

The project uses Go modules for dependency management and a `makefile` for common tasks.

1. **Clone the repository (if you haven't already):**
   ```bash
   git clone <URL_OF_THIS_REPOSITORY>
   cd <NAME_OF_THE_CLONED_DIRECTORY> # e.g., cd gsched
   ```
   (Replace `<URL_OF_THIS_REPOSITORY>` with the actual URL and `<NAME_OF_THE_CLONED_DIRECTORY>` with the directory name created by git clone, which is typically the repository name.)

2. **Build the application:**

   You can build the application using the standard Go build command. Navigate to the root of the project directory.
   ```bash
   go build -o bin/gsched cmd/main.go
   ```
   This will create an executable at `bin/gsched`.

3. **Run the application:**
   The application can run in one of two roles: `scheduler` or `worker`, specified using the `--role` command-line flag. The default role is `scheduler`.

   *   **Scheduler Role**: Reads `schedule/schedule.json`, monitors it for changes, and dispatches jobs to Kafka based on their cron expressions.
   *   **Worker Role**: Consumes jobs from the Kafka topic and executes the defined tasks.

   **Using `go run`:**
   ```bash
   # Run as scheduler (default role)
   go run cmd/main.go
   # Explicitly run as scheduler
   go run cmd/main.go --role=scheduler

   # Run as worker
   go run cmd/main.go --role=worker
   ```

   **Using the compiled binary:**
   After building the application (e.g., to `bin/gsched`):
   ```bash
   # Run as scheduler (default role)
   ./bin/gsched
   # Explicitly run as scheduler
   ./bin/gsched --role=scheduler

   # Run as worker
   ./bin/gsched --role=worker
   ```
   Note: The `make run` command (if used) will run the scheduler by default and does not support the `--role` flag. For specifying roles, use `go run cmd/main.go --role=<role>` or run the compiled binary directly with the flag.


### 4. Understanding the Schedule Configuration (`schedule/schedule.json`)

The Job Dispatcher reads its job schedule from a JSON file located at `schedule/schedule.json` by default. This file contains an array of job objects. Each job has the following fields:

- `jobName`: A descriptive name for the job (e.g., "DailyReportGenerator"). This is primarily for human readability and logging.
- `priority`: An integer priority for the job. While defined, this is not actively used by the current scheduler logic but is available for future enhancements or custom consumer logic.
- `taskName`: **(Important)** The string identifier of the task to be executed. This must match the name returned by the `Name()` method of a registered `models.Task` implementation (e.g., `"LogTask"`, `"EchoTask"`).
- `taskParams`: A JSON object containing parameters specific to the task. These parameters are passed as a `map[string]interface{}` to the task's `Execute` method. This field is optional; if omitted or empty, an empty map will be passed to the task.
- `cronTime`: A standard cron expression (e.g., `` `0 * * * *` `` for hourly execution) defining when the job should run. The format is `minute hour day-of-month month day-of-week`.

**Example `schedule.json` entry:**

```json
{
  "jobs": [
    {
      "jobName": "MyExampleLogger",
      "priority": 1,
      "taskName": "LogTask",
      "taskParams": {
        "message": "This is a scheduled log message for MyExampleLogger, running every 5 minutes."
      },
      "cronTime": "* * * * *"
    },
    {
      "jobName": "MyHourlyEcho",
      "priority": 2,
      "taskName": "EchoTask",
      "taskParams": {
        "source": "schedule.json",
        "details": "Echoing parameters for MyHourlyEcho job"
      },
      "cronTime": "* * * * *"
    }
    // ... more jobs
  ]
}
```
This example schedules `LogTask` to run every 5 minutes with a specific message, and `EchoTask` to run hourly with its own set of parameters.

The application monitors `schedule/schedule.json` for changes and reloads the schedule automatically if the file is modified.

## Implementing Custom Tasks

The scheduler now supports custom task implementations, allowing you to define specific actions to be executed.

### 1. Define Your Task

To create a custom task, your Go struct must implement the `models.Task` interface, which is defined in `models/task.go`:

```go
package models

// Task represents a job that can be scheduled and executed.
// Implementations of this interface define specific actions to be performed.
type Task interface {
	// Name returns the unique identifier for the task.
	// This name is used in the schedule.json to refer to this task.
	Name() string

	// Execute performs the action defined by the task.
	// params provides a way to pass task-specific parameters defined in schedule.json.
	// It returns an error if the execution fails, otherwise nil.
	Execute(params map[string]interface{}) error
}
```

-   `Name() string`: This method should return a unique string identifier for your task. This is the name you will use in the `taskName` field in `schedule/schedule.json`.
-   `Execute(params map[string]interface{}) error`: This method contains the core logic of your task. The `params` map will be populated from the `taskParams` JSON object in `schedule/schedule.json` for the corresponding job. Return `nil` if the task execution is successful, or an `error` if something goes wrong.

**Example Custom Task:**

Here's a basic example of how you might define a custom task in your own package (e.g., `mytasks/custom_task.go`):

```go
package mytasks

import (
	"fmt"
	"github.com/c-m3-codin/gsched/models"
	"github.com/c-m3-codin/gsched/tasks" // For task registration
)

type MyGreeterTask struct{}

func (mgt *MyGreeterTask) Name() string {
	return "MyGreeterTask" // This name is used in schedule.json
}

func (mgt *MyGreeterTask) Execute(params map[string]interface{}) error {
	target, ok := params["targetPerson"].(string)
	if !ok {
		// It's good practice to return an error if required parameters are missing or invalid.
		return fmt.Errorf("MyGreeterTask: 'targetPerson' parameter is missing or not a string")
	}

	greeting, _ := params["greetingMessage"].(string) // Optional parameter
	if greeting == "" {
		greeting = "Hello" // Default greeting
	}

	fmt.Printf("%s, %s! MyGreeterTask executed successfully.\n", greeting, target)
	return nil
}

// It's common practice to register your task(s) in an init function
// within the same package where they are defined.
func init() {
	// Create an instance of your task and register it.
	// The tasks.RegisterTask function is provided by the scheduler's task system.
	tasks.RegisterTask(&MyGreeterTask{})
}
```

### 2. Register Your Task

For the scheduler to be able to find and execute your custom task, it must be registered with the central task registry. This is typically done using an `init()` function in the Go package where your task is defined, as shown in the example above.

The `tasks.RegisterTask()` function takes an instance of your task (which must implement `models.Task`). When your application starts, Go will execute all `init()` functions, ensuring your tasks are registered before the scheduler needs them.

You can see further examples of task definitions and registrations in `tasks/example_tasks.go`, which includes `LogTask` and `EchoTask`.

## TODO

- [x] **Implement Consumer Workers**: Develop worker applications that subscribe to the Kafka topics, consume the dispatched job messages, and execute/process the actual tasks defined in the jobs.
- [ ] **Dynamic Consumer Worker Pooling**: Implement a mechanism to dynamically scale the number of consumer workers. This pool of consumer workers should be able to increase or decrease its size based on the load in the Kafka job queue (e.g., number of messages pending, processing rate).

