package main

import "sync"

// Worker pool: fixed number of goroutines consume from a shared jobs channel.
// Sender closes the channel when done, causing all workers to exit their range loop.
func main() {
	wg := sync.WaitGroup{}
	jobs := make(chan int, 10)

	for i := 1; i <= 3; i++ {
		wg.Add(1)
		go func(workerID int) {
			defer wg.Done()
			for job := range jobs {
				println("worker", workerID, "received job", job)
			}
		}(i)
	}

	for i := 1; i <= 10; i++ {
		jobs <- i
	}
	close(jobs)

	wg.Wait()
}
