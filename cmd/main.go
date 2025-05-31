package main

import (
	"fmt"
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
	fmt.Println("there")

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
		fmt.Println(err)
		return
	}
	fmt.Println("\n\n previous hash is", c.ScheduleFileHash)
	if c.ScheduleFileHash != hash {
		c.ScheduleFileHash = hash
		c.ScheduleChangeChannel <- true
	}
	fmt.Println(hash)

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

		fmt.Println("thorwing job into channel ", i)

		inputChannel <- jobs[i]

	}
	close(inputChannel)

}

func taskProducers(inputChannel chan models.Job, workerNumber int) {
	for {
		select {
		case job, ok := <-inputChannel:
			if !ok {
				fmt.Printf("Worker %d: Input channel closed. Exiting.\n", workerNumber)
				return
			}

			// Updated log message
			fmt.Printf("Worker %d: Picked job: %s (Task: %s)\n", workerNumber, job.JobName, job.TaskName)

			if matchCronExpression(job.CronTime, time.Now()) {
				// This log can be verbose, but useful for seeing cron activity
				// fmt.Printf("Worker %d: Cron expression matched for job: %s\n", workerNumber, job.JobName)

				task, err := tasks.GetTask(job.TaskName)
				if err != nil {
					fmt.Printf("Error in worker %d: Task '%s' not found. Skipping job: '%s'. Error: %v\n", workerNumber, job.TaskName, job.JobName, err)
					continue // Skip to the next job
				}

				// fmt.Printf("Worker %d: Executing task '%s' for job '%s'\n", workerNumber, job.TaskName, job.JobName)
				if err := task.Execute(job.TaskParams); err != nil {
					fmt.Printf("Error in worker %d executing task '%s' for job '%s': %v\n", workerNumber, job.TaskName, job.JobName, err)
					continue // Skip to the next job
				}

				// fmt.Printf("Worker %d: Task '%s' executed successfully for job '%s'. Pushing to queue.\n", workerNumber, job.TaskName, job.JobName)
				utility.PushToQueue(job) // Push original job struct to Kafka

			} else {
				// This part can be noisy if jobs are checked frequently; consider adjusting log level or removing if not essential for normal operation.
				// fmt.Printf("Worker %d: Cron expression did not match for job: %s\n", workerNumber, job.JobName)
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

	// fmt.Println("Current ", t.Minute(), t.Hour())
	// fmt.Println("Previous ", minute, hour)

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
