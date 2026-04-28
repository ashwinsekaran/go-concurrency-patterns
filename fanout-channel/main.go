package main

import "sync"

// Fan-out: N goroutines all send into a single channel.
// A separate goroutine closes the channel once all workers finish,
// allowing the main goroutine to drain with range.
func main() {
	wg := sync.WaitGroup{}
	ch := make(chan int, 1)

	for i := 1; i <= 5; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			println("worker", i)
			ch <- i
		}(i)
	}

	go func() {
		wg.Wait()
		close(ch)
	}()

	for i := range ch {
		println("received", i)
	}
}
