package main

import (
	"fmt"

	"github.com/c-m3-codin/gsched/utility"
)

type Config struct {
	*utility.Config
}

func main() {

	utconfig := utility.InitConfig()
	config := Config{&utconfig}
	config.WaitGroup.Add(1)
	go changeEvent(config.ScheduleChangeChannel)
	go config.pulse()
	config.WaitGroup.Wait()

}

func changeEvent(change chan bool) {
	for {
		if <-change {
			fmt.Println("fileChanged")
		}
	}
}

func (c Config) pulse() {
	defer c.WaitGroup.Done()
	for {
		select {
		case <-c.Ticker.C:
			c.scheduleUpdateCheck()
			emitTasks()

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

type ScheduleFile struct {
	Jobs []Job `json:"jobs"`
}

type Job struct {
	JobName  string `json:"jobName"`
	Priority int    `json:"priority"`
	Job      string `json:"job"`
	CronTime string `json:"cronTime"`
}

func emitTasks() {

	fmt.Println("Emmited Task")

}
