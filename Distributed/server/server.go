package main

import (
	"flag"
	"fmt"
	"log"
	"net"
	"net/rpc"
	"sync"

	"uk.ac.bris.cs/gameoflife/stubs"
)

// RuleSet constants (must match Params definitions)
const (
	Conway      = 0 // B3/S23
	HighLife    = 1 // B36/S23
	DayAndNight = 2 // B3678/S34678
)

type GolWorker struct {
	mu sync.Mutex
}

// calculateNeighboursWithGhost counts live neighbours using ghost rows for boundaries.
func calculateNeighboursWithGhost(world [][]uint8, x, y, width int) int {
	count := 0
	for deltaY := -1; deltaY <= 1; deltaY++ {
		for deltaX := -1; deltaX <= 1; deltaX++ {
			if deltaX == 0 && deltaY == 0 {
				continue
			}
			nx := x + deltaX
			ny := y + deltaY

			if nx < 0 {
				nx = width - 1
			} else if nx >= width {
				nx = 0
			}

			if world[ny][nx] == 255 {
				count++
			}
		}
	}
	return count
}

// applyRules returns the next cell state based on the chosen rule set.
func applyRules(alive bool, neighbours, ruleSet int) uint8 {
	switch ruleSet {
	case HighLife:
		if alive {
			if neighbours == 2 || neighbours == 3 {
				return 255
			}
			return 0
		}
		if neighbours == 3 || neighbours == 6 {
			return 255
		}
		return 0
	case DayAndNight:
		if alive {
			if neighbours == 3 || neighbours == 4 || neighbours == 6 || neighbours == 7 || neighbours == 8 {
				return 255
			}
			return 0
		}
		if neighbours == 3 || neighbours == 6 || neighbours == 7 || neighbours == 8 {
			return 255
		}
		return 0
	default: // Conway
		if alive {
			if neighbours == 2 || neighbours == 3 {
				return 255
			}
			return 0
		}
		if neighbours == 3 {
			return 255
		}
		return 0
	}
}

func (g *GolWorker) CalculateNextState(req *stubs.WorkerRequest, res *stubs.WorkerResponse) error {
	g.mu.Lock()
	defer g.mu.Unlock()

	log.Printf("Worker processing slice: Y=%d to Y=%d, size=%dx%d, ruleSet=%d",
		req.StartY, req.EndY, req.ImageWidth, len(req.WorldSlice), req.RuleSet)

	worldSlice := req.WorldSlice
	height := len(worldSlice)
	width := req.ImageWidth

	actualHeight := height - 2
	newWorldSlice := make([][]uint8, actualHeight)

	aliveCellsCount := 0

	for y := 1; y < height-1; y++ {
		newRow := make([]uint8, width)
		for x := 0; x < width; x++ {
			neighbours := calculateNeighboursWithGhost(worldSlice, x, y, width)
			newRow[x] = applyRules(worldSlice[y][x] == 255, neighbours, req.RuleSet)
			if newRow[x] == 255 {
				aliveCellsCount++
			}
		}
		newWorldSlice[y-1] = newRow
	}

	log.Printf("Worker completed slice: Y=%d to Y=%d, alive cells: %d",
		req.StartY, req.EndY, aliveCellsCount)

	res.WorldSlice = newWorldSlice
	return nil
}

func main() {
	pAddr := flag.String("port", "8031", "Port to listen on")
	flag.Parse()

	golWorker := new(GolWorker)
	rpc.RegisterName("GolWorker", golWorker) // 워커로 등록

	listener, err := net.Listen("tcp", ":"+*pAddr)
	if err != nil {
		log.Fatal("Error starting Gol worker:", err)
	}
	defer listener.Close()
	fmt.Println("Gol Worker listening on port", *pAddr)
	rpc.Accept(listener)
}
