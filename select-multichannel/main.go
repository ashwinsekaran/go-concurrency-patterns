package main

import (
	"sync"
	"time"
)

// select on multiple channels: reacts to whichever channel sends first.
// Worker 1 sends after 1s, worker 2 after 2s — each select picks the ready channel.
func main() {
	wg := sync.WaitGroup{}
	ch1 := make(chan string)
	ch2 := make(chan string)

	for i := 1; i <= 2; i++ {
		wg.Add(1)
		go func(workerID int) {
			defer wg.Done()
			println("worker", workerID)
			if workerID == 1 {
				time.Sleep(1 * time.Second)
				ch1 <- "hello"
			} else {
				time.Sleep(2 * time.Second)
				ch2 <- "world"
			}
		}(i)
	}

	select {
	case msg := <-ch1:
		println("ch1:", msg)
	case msg := <-ch2:
		println("ch2:", msg)
	}

	select {
	case msg := <-ch1:
		println("ch1:", msg)
	case msg := <-ch2:
		println("ch2:", msg)
	}

	wg.Wait()
}
