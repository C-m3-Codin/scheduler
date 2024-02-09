package utility

import (
	"sync"
	"time"
)

type Config struct {
	ScheduleFile          string
	ScheduleChangeChannel chan bool
	WaitGroup             *sync.WaitGroup
	Ticker                *time.Ticker
	ScheduleFileHash      string
}

func InitConfig() (config Config) {
	filePath := "../schedule/schedule.json"
	change := make(chan bool, 1)
	ticker := time.NewTicker(2 * time.Second)
	var wg sync.WaitGroup
	config = Config{
		ScheduleFile:          filePath,
		ScheduleChangeChannel: change,
		WaitGroup:             &wg,
		Ticker:                ticker,
		ScheduleFileHash:      "nothing",
	}
	return
}
