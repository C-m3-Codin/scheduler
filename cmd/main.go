package main

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/c-m3-codin/gsched/constants"
	"github.com/c-m3-codin/gsched/models"
	"github.com/c-m3-codin/gsched/utility"
)

type Config struct {
	*utility.Config
}

func main() {
	fmt.Println("there")

	// utconfig := utility.InitConfig()
	// config := Config{&utconfig}
	// config.WaitGroup.Add(1)
	// // go changeEvent(config.ScheduleChangeChannel)
	// go config.pulse()
	// config.WaitGroup.Wait()

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
				// Channel is closed, exit the loop
				fmt.Println("Worker", workerNumber, "received signal to stop.")
				return
			}

			// Process the received job
			fmt.Println("Picking job:", job, " by worker ", workerNumber)
			if matchCronExpression(job.CronTime, time.Now()) {
				// produce task to queue
				utility.PushToQueue(job)
				fmt.Println("\n\n\n\n Cron expression matches the current time.")
			} else {
				fmt.Println("Cron expression does not match the current time.")
			}
			// produce the job into the queue
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

	fmt.Println("Current ", t.Minute(), t.Hour())
	fmt.Println("Previous ", minute, hour)

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
