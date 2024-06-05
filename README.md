# Job Dispatcher

This repository contains a job dispatcher written in Go that dispatches jobs to a Kafka queue based on a schedule defined in a configuration file. The dispatcher reads the schedule, monitors changes, and processes jobs according to their cron expressions.

## Features

- **Scheduled Job Dispatching**: Dispatches jobs based on cron expressions.
- **Kafka Integration**: Pushes jobs to a Kafka queue.
- **Configuration Monitoring**: Monitors changes in the schedule configuration file.
- **Concurrent Task Management**: Utilizes worker goroutines for concurrent job processing.

### Prerequisites

- Go (version 1.16+ recommended)
- Kafka
- Zookeeper (for Kafka coordination)

