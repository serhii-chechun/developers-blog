/*
	03_tracer demonstrates the execution tracer with runtime/trace.

$ go run main.go
$ go tool trace trace.out
*/
package main

import (
	"fmt"
	"os"
	"runtime/trace"
	"sync"
)

// worker does a bounded amount of work, then hands off through a channel.
func worker(jobs <-chan int, done chan<- int, wg *sync.WaitGroup) {
	defer wg.Done()
	for j := range jobs {
		// Simulate work with a small in-CPU computation.
		sum := 0
		for i := range 1_000_000 {
			sum += i ^ j
		}
		done <- sum
	}
}

func main() {
	f, err := os.Create("trace.out")
	if err != nil {
		panic(err)
	}
	defer f.Close()

	if err := trace.Start(f); err != nil {
		panic(err)
	}
	defer trace.Stop()

	jobs := make(chan int, 100)
	done := make(chan int, 100)

	var wg sync.WaitGroup
	for range 4 {
		wg.Add(1)
		go worker(jobs, done, &wg)
	}

	go func() {
		for i := range 100 {
			jobs <- i
		}
		close(jobs)
	}()

	go func() {
		wg.Wait()
		close(done)
	}()

	count := 0
	for range done {
		count++
	}
	fmt.Println("completed jobs:", count)
}
