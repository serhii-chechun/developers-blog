/*
	01_pprof_cpu demonstrates writing a CPU profile with runtime/pprof.

$ go run main.go
$ go tool pprof -http=:8080 cpu.prof
*/
package main

import (
	"log"
	"os"
	"runtime/pprof"
)

// fib is deliberately naive: exponential time, the classic hot path.
func fib(n int) int {
	if n < 2 {
		return n
	}
	return fib(n-1) + fib(n-2)
}

func main() {
	f, err := os.Create("cpu.prof")
	if err != nil {
		log.Fatal(err)
	}
	defer f.Close()

	if err := pprof.StartCPUProfile(f); err != nil {
		log.Fatal(err)
	}
	defer pprof.StopCPUProfile()

	total := 0
	for i := range 37 {
		total += fib(i)
	}
	log.Printf("total = %d", total)
}
