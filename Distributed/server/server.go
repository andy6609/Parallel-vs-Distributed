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

type GolWorker struct {
	mu sync.Mutex
}

// Ghost rows를 사용하는 새로운 이웃 계산 함수
func calculateNeighboursWithGhost(world [][]uint8, x, y, width int) int {
	count := 0
	for deltaY := -1; deltaY <= 1; deltaY++ {
		for deltaX := -1; deltaX <= 1; deltaX++ {
			if deltaX == 0 && deltaY == 0 {
				continue
			}
			nx := x + deltaX
			ny := y + deltaY

			// x 방향 경계 처리 (world wrapping)
			if nx < 0 {
				nx = width - 1
			} else if nx >= width {
				nx = 0
			}

			// ghost rows를 사용하므로 y 방향은 직접 접근
			if world[ny][nx] == 255 {
				count++
			}
		}
	}
	return count
}

func (g *GolWorker) CalculateNextState(req *stubs.WorkerRequest, res *stubs.WorkerResponse) error {
	g.mu.Lock()
	defer g.mu.Unlock()

	log.Printf("Worker processing slice: Y=%d to Y=%d, size=%dx%d",
		req.StartY, req.EndY, req.ImageWidth, len(req.WorldSlice))

	worldSlice := req.WorldSlice
	height := len(worldSlice)
	width := req.ImageWidth

	// Ghost rows를 제외한 실제 처리할 행 수
	actualHeight := height - 2
	newWorldSlice := make([][]uint8, actualHeight)

	aliveCellsCount := 0

	// ghost rows를 제외하고 중간 부분만 처리 (인덱스 1부터 height-2까지)
	for y := 1; y < height-1; y++ {
		newRow := make([]uint8, width)
		for x := 0; x < width; x++ {
			neighbours := calculateNeighboursWithGhost(worldSlice, x, y, width)
			if worldSlice[y][x] == 255 {
				// 살아있는 셀
				if neighbours == 2 || neighbours == 3 {
					newRow[x] = 255
					aliveCellsCount++
				} else {
					newRow[x] = 0
				}
			} else {
				// 죽은 셀
				if neighbours == 3 {
					newRow[x] = 255
					aliveCellsCount++
				} else {
					newRow[x] = 0
				}
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
