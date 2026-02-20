package main

import (
	"fmt"
	"os"
	"testing"

	"uk.ac.bris.cs/gameoflife/gol"
)

// go test -run ^$ -bench . -benchtime 1x -count 5 | tee result/results.out
// go run golang.org/x/perf/cmd/benchstat -csv result/results.out | tee result/results.csv

func BenchmarkGol(b *testing.B) {
	// Disable all program output apart from benchmark results
	os.Stdout = nil

	gridSizes := []int{128, 256, 512}
	threadCounts := []int{1, 2, 4, 8, 16}

	for _, size := range gridSizes {
		for _, threads := range threadCounts {
			name := fmt.Sprintf("%dx%d/t%d", size, size, threads)
			b.Run(name, func(b *testing.B) {
				p := gol.Params{
					Turns:       1000,
					Threads:     threads,
					ImageWidth:  size,
					ImageHeight: size,
				}
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

// BenchmarkSerial explicitly benchmarks the serial (single-threaded) path
// using Threads=0, which bypasses the Worker Pool entirely.
func BenchmarkSerial(b *testing.B) {
	os.Stdout = nil

	gridSizes := []int{128, 256, 512}

	for _, size := range gridSizes {
		name := fmt.Sprintf("%dx%d/serial", size, size)
		b.Run(name, func(b *testing.B) {
			p := gol.Params{
				Turns:       1000,
				Threads:     0, // 0 = explicit serial path (no worker pool)
				ImageWidth:  size,
				ImageHeight: size,
			}
			for i := 0; i < b.N; i++ {
				events := make(chan gol.Event)
				go gol.Run(p, events, nil)
				for range events {
				}
			}
		})
	}
}
