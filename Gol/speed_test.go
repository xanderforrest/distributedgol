package main

import (
	"fmt"
	"os"
	"testing"
	"uk.ac.bris.cs/gameoflife/gol"
)

// Benchmark applies the filter to the ship.png b.N times.
// The time taken is carefully measured by go.
// The b.N  repetition is needed because benchmark results are not always constant.
func BenchmarkGol(b *testing.B) {
	// Disable all program output apart from benchmark results
	os.Stdout = nil

	p := gol.Params{ImageWidth: 5120, ImageHeight: 5120, Turns: 5}

	// Use a for-loop to run 5 sub-benchmarks, with 1, 2, 4, 8 and 16 workers.
	engines := []int{8, 4, 2, 1}
	for _, engineCount := range engines {
		for threads := 1; threads < 3; threads++ {
			name := fmt.Sprintf("%dx%dx%d-%d-%d", p.ImageWidth, p.ImageHeight, p.Turns, engineCount, threads)
			p.Engines = engineCount
			p.Threads = threads

			b.Run(name, func(b *testing.B) {
				for i := 0; i < b.N; i++ {
					events := make(chan gol.Event)
					go gol.Run(p, events, nil)
					for range events {
					}
				}
			})
		}
	}
}
