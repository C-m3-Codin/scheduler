# Job Dispatcher

This repository contains a job dispatcher written in Go that dispatches jobs to a Kafka queue based on a schedule defined in a configuration file. The dispatcher reads the schedule, monitors changes, and processes jobs according to their cron expressions.

## Features

- **Scheduled Job Dispatching**: Dispatches jobs based on cron expressions. The application reads a schedule configuration file and uses cron expressions (e.g., `` `minute hour day month weekday` ``) to determine when each job should be executed.
- **Kafka Integration**: Pushes jobs to a Kafka queue. Once a job's scheduled time arrives, it is pushed as a message to a specified Kafka topic, allowing distributed workers to consume and process these jobs.
- **Configuration File Monitoring**: Monitors the schedule configuration file for any changes. If a change is detected (based on MD5 hash comparison), the schedule is reloaded automatically.
- **Concurrent Task Producers**: Utilizes multiple goroutines to concurrently check job schedules and produce tasks to the Kafka queue. This allows for efficient handling of multiple jobs defined in the schedule.

### Prerequisites

- Go (version 1.20 or higher)
- Kafka
- Zookeeper (for Kafka coordination)

## Getting Started

This section will guide you through setting up and running the Job Dispatcher project.

### 1. Set Up Go Environment

Ensure you have Go installed on your system. This project uses Go version `1.20` as specified in the `go.mod` file. You can download Go from the [official Go website](https://golang.org/dl/).

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
   The `makefile` provides a convenient way to build the project. This command will also handle formatting and dependency vendoring.
   ```bash
   make build
   ```
   This will create an executable at `bin/main`.

3. **Run the application:**
   You can run the application using the `makefile`:
   ```bash
   make run
   ```
   Alternatively, after building, you can run the executable directly:
   ```bash
   ./bin/main
   ```
   Or, run directly using `go run`:
   ```bash
   go run cmd/main.go
   ```

### 4. Understanding the Schedule Configuration

The Job Dispatcher reads its job schedule from a JSON file located at `schedule/schedule.json` by default. This file contains an array of job objects.

**Structure of `schedule.json`:**
```json
{
   "jobs": [
    {
        "jobName": "uniqueJobNameIdentifier",
        "priority": 5,
        "job": "actualCommandOrIdentifierForWorker",
        "cronTime": "* * * * *"
    },
    {
        "jobName": "anotherJob",
        "priority": 10,
        "job": "anotherTaskDetails",
        "cronTime": "0 12 * * MON-FRI"
    }
    // ... more jobs
]
}
```

- `jobName`: A string to uniquely identify the job.
- `priority`: An integer representing the job's priority (higher value could mean higher priority, depending on consumer logic).
- `job`: A string field that can contain details about the task, such as a command to execute, a script name, or parameters for a worker. In the example `schedule/schedule.json` provided in the repository, this is set to `"fileName"`; this value is illustrative and its specific interpretation is determined by the consumer applications that will process these jobs from the Kafka queue.
- `cronTime`: A standard cron expression (e.g., `` `minute hour day_of_month month day_of_week` ``) that defines when the job should be dispatched. For example, `` `* * * * *` `` runs every minute, and `` `0 12 * * MON-FRI` `` runs at 12:00 PM on weekdays.

The application monitors `schedule/schedule.json` for changes and reloads the schedule automatically if the file is modified.

## TODO

- [ ] **Implement Consumer Workers**: Develop worker applications that subscribe to the Kafka topics, consume the dispatched job messages, and execute/process the actual tasks defined in the jobs.
- [ ] **Dynamic Consumer Worker Pooling**: Implement a mechanism to dynamically scale the number of consumer workers. This pool of consumer workers should be able to increase or decrease its size based on the load in the Kafka job queue (e.g., number of messages pending, processing rate).

