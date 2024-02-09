package main

import (
	"fmt"

	"github.com/c-m3-codin/gsched/models"
	"github.com/c-m3-codin/gsched/utility"
)

type Config struct {
	*utility.Config
}

func main() {

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
			emitTasks(scheduleFile.Jobs)

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

func emitTasks([]models.Job) {

	fmt.Println("Emmited Task")

}
