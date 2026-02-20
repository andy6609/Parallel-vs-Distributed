package main

import (
	"fmt"
	"os"
	"testing"

	"uk.ac.bris.cs/gameoflife/gol"
)

// NOTE: Distributed benchmarks require the broker and worker servers to be running.
// Start broker:  go run ./broker/broker.go
// Start workers: go run ./server/server.go -port 8031
//                go run ./server/server.go -port 8032
//
// Run benchmarks:
//   go test -run ^$ -bench . -benchtime 1x -count 5 | tee result/results.out
//   go run golang.org/x/perf/cmd/benchstat -csv result/results.out | tee result/results.csv

// BenchmarkGolGridSize measures distributed system performance across different grid sizes.
// This captures network transfer overhead, serialisation cost, and compute time.
func BenchmarkGolGridSize(b *testing.B) {
	os.Stdout = nil

	gridSizes := []int{128, 256, 512}

	for _, size := range gridSizes {
		name := fmt.Sprintf("%dx%d", size, size)
		b.Run(name, func(b *testing.B) {
			p := gol.Params{
				Turns:       100,
				Threads:     1,
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

// BenchmarkGolRuleSets compares performance of different GoL rule sets on a 512x512 grid.
// Different rules produce different cell densities, affecting computation per turn.
func BenchmarkGolRuleSets(b *testing.B) {
	os.Stdout = nil

	ruleSets := []struct {
		name    string
		ruleSet int
	}{
		{"Conway_B3S23", gol.Conway},
		{"HighLife_B36S23", gol.HighLife},
		{"DayAndNight_B3678S34678", gol.DayAndNight},
	}

	for _, rs := range ruleSets {
		b.Run(rs.name, func(b *testing.B) {
			p := gol.Params{
				Turns:       100,
				Threads:     1,
				ImageWidth:  512,
				ImageHeight: 512,
				RuleSet:     rs.ruleSet,
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
