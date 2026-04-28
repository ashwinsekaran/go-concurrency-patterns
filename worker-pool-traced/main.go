package main

import (
	"fmt"
	"sync"
)

// Worker pool with step-by-step logging to observe goroutine scheduling
// and buffered channel behavior (all 10 jobs can be sent before workers start consuming).
func main() {
	wg := sync.WaitGroup{}
	jobs := make(chan int, 10)

	fmt.Println("starting workers")
	for i := 1; i <= 3; i++ {
		wg.Add(1)
		fmt.Println("starting worker", i)
		go func(workerID int) {
			defer wg.Done()
			fmt.Println("worker goroutine started", workerID)
			for job := range jobs {
				fmt.Printf("worker %d received job %d\n", workerID, job)
			}
		}(i)
	}

	fmt.Println("sending jobs")
	for i := 1; i <= 10; i++ {
		fmt.Println("sending job", i)
		jobs <- i
	}
	fmt.Println("closing jobs channel")
	close(jobs)

	wg.Wait()
	fmt.Println("done")
}
