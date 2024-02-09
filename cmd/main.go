package main

import (
	"fmt"
	"sync"
	"time"

	"github.com/c-m3-codin/gsched/utility"
)

func main() {
	filePath := "../schedule/schedule.json"
	change := make(chan bool, 1)
	ticker := time.NewTicker(1 * time.Second)
	var wg sync.WaitGroup
	wg.Add(1)
	go changeEvent(change)
	go checkChange(change, filePath, ticker, &wg)
	wg.Wait()

}

func changeEvent(change chan bool) {
	for {
		if <-change {
			fmt.Println("fileChanged")
		}
	}
}

func checkChange(change chan bool, filePath string, ticker *time.Ticker, wg *sync.WaitGroup) {
	defer wg.Done()
	var hashPrev string
	for {
		select {
		case <-ticker.C:
			hash, err := utility.Filemd5sum(filePath)
			if err != nil {
				fmt.Println(err)
				return
			}
			if hashPrev != hash {
				hashPrev = hash
				change <- true
			}
			fmt.Println(hash)

		}

	}
}
