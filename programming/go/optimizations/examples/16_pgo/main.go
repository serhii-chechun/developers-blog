// 16_pgo is a minimal program for demonstrating Profile-Guided Optimization.
//
// PGO has been supported since Go 1.20; -pgo=auto is the default since Go 1.21.
// Docs: https://go.dev/doc/pgo
//
// Collect a profile:
//
// $ go run main.go
//
// Build with PGO (option A - explicit profile):
//
// $ go build -pgo=cpu.pprof -o pgo-demo main.go
//
// Build with PGO (option B - auto, the default since Go 1.21):
//
// $ cp cpu.pprof ./default.pgo
// $ go build -o pgo-demo main.go
//
// Disable PGO:
//
// $ go build -pgo=off -o pgo-demo main.go
package main

import (
	"fmt"
	"log"
	"os"
	"runtime/pprof"
)

// Shape is an interface with a single hot implementation,
// which PGO can devirtualize when the profile shows it dominates.
type Shape interface {
	Area() int
}

type Rect struct{ W, H int }

func (r Rect) Area() int { return r.W * r.H }

// TotalArea is the hot loop the profiler will capture.
func TotalArea(shapes []Shape) int {
	total := 0
	for _, s := range shapes {
		total += s.Area()
	}
	return total
}

func main() {
	f, err := os.Create("cpu.pprof")
	if err != nil {
		log.Fatal(err)
	}
	defer f.Close()

	if err := pprof.StartCPUProfile(f); err != nil {
		log.Fatal(err)
	}
	defer pprof.StopCPUProfile()

	shapes := make([]Shape, 1000)
	for i := range shapes {
		shapes[i] = Rect{W: i, H: i + 1}
	}

	total := 0
	for range 100_000 {
		total += TotalArea(shapes)
	}
	fmt.Println("total area:", total)
}
