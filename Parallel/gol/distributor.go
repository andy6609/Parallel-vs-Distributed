package gol

import (
	"fmt"
	"sync"
	"time"

	"uk.ac.bris.cs/gameoflife/util"
)

type DistributorChannels struct {
	events     chan<- Event
	ioCommand  chan<- ioCommand
	ioIdle     <-chan bool
	ioFilename chan<- string
	ioOutput   chan<- uint8
	ioInput    <-chan uint8
}

// Global Worker Pool
var globalWorkerPool *WorkerPool
var workerPoolMutex sync.Mutex

// Worker Pool structure
type WorkerPool struct {
	workers    int
	taskChan   chan workerTask
	resultChan chan workerResult
	quit       chan struct{}
	wg         sync.WaitGroup
}

// Worker Task
type workerTask struct {
	startY, endY int
	world        [][]byte
	params       Params
	turn         int
}

// Initialize Worker Pool
func NewWorkerPool(workers int) *WorkerPool {
	wp := &WorkerPool{
		workers:    workers,
		taskChan:   make(chan workerTask, workers*2),
		resultChan: make(chan workerResult, workers*2),
		quit:       make(chan struct{}),
	}

	for i := 0; i < workers; i++ {
		wp.wg.Add(1)
		go wp.worker()
	}

	return wp
}

// Worker function
func (wp *WorkerPool) worker() {
	defer wp.wg.Done()
	for {
		select {
		case task := <-wp.taskChan:
			result := wp.processTask(task)
			wp.resultChan <- result
		case <-wp.quit:
			return
		}
	}
}

// Task processing function
func (wp *WorkerPool) processTask(task workerTask) workerResult {
	newWorld := make([][]byte, task.endY-task.startY)
	for i := range newWorld {
		newWorld[i] = make([]byte, task.params.ImageWidth)
	}

	var flipped []util.Cell

	for y := task.startY; y < task.endY; y++ {
		for x := 0; x < task.params.ImageWidth; x++ {
			neighbors := countAliveNeighbors(x, y, task.world, task.params.ImageWidth, task.params.ImageHeight)
			localY := y - task.startY

			oldState := task.world[y][x]
			var newState byte
			if oldState == 255 {
				if neighbors < 2 || neighbors > 3 {
					newState = 0
				} else {
					newState = 255
				}
			} else {
				if neighbors == 3 {
					newState = 255
				} else {
					newState = 0
				}
			}
			newWorld[localY][x] = newState

			// if state changed, add to flipped
			if oldState != newState {
				flipped = append(flipped, util.Cell{X: x, Y: y})
			}
		}
	}

	return workerResult{task.startY, task.endY, newWorld, flipped}
}

// Stop Worker Pool
func (wp *WorkerPool) Stop() {
	close(wp.quit)
	wp.wg.Wait()
	close(wp.taskChan)
	close(wp.resultChan)
}

// golParams removed, only Params is used
func countAliveNeighbors(x, y int, world [][]byte, width, height int) int {
	count := 0
	for dx := -1; dx <= 1; dx++ {
		for dy := -1; dy <= 1; dy++ {
			if dx == 0 && dy == 0 {
				continue // Skip the cell itself
			}
			// Wrap around the edges (toroidal world) - using if statements instead of modulus
			nx := x + dx
			ny := y + dy

			// Handle edge wrapping with if statements
			if nx < 0 {
				nx = width - 1
			} else if nx >= width {
				nx = 0
			}

			if ny < 0 {
				ny = height - 1
			} else if ny >= height {
				ny = 0
			}

			if world[ny][nx] == 255 {
				count++
			}
		}
	}
	return count
}

type workerResult struct {
	startY  int
	endY    int
	world   [][]byte
	flipped []util.Cell
}

func CalculateNextState(p Params, threads int, world [][]byte, c DistributorChannels, turn int) [][]byte {
	height := p.ImageHeight
	workerHeight := height / threads

	// Use global Worker Pool
	workerPoolMutex.Lock()
	wp := globalWorkerPool
	workerPoolMutex.Unlock()

	// Send tasks
	for i := 0; i < threads; i++ {
		startY := i * workerHeight
		endY := startY + workerHeight
		if i == threads-1 {
			endY = height
		}

		task := workerTask{
			startY: startY,
			endY:   endY,
			world:  world,
			params: p,
			turn:   turn,
		}
		wp.taskChan <- task
	}

	// Collect results
	newWorld := make([][]byte, height)
	for i := range newWorld {
		newWorld[i] = make([]byte, p.ImageWidth)
	}

	var allFlipped [][]util.Cell

	for i := 0; i < threads; i++ {
		result := <-wp.resultChan

		for y := 0; y < result.endY-result.startY; y++ {
			copy(newWorld[result.startY+y], result.world[y])
		}
		if len(result.flipped) > 0 {
			allFlipped = append(allFlipped, result.flipped)
		}
	}

	// Send events
	for _, cells := range allFlipped {
		c.events <- CellsFlipped{CompletedTurns: turn, Cells: cells}
	}

	return newWorld
}

// distributor divides the work between workers and interacts with other goroutines.
func distributor(p Params, c DistributorChannels, keyPresses <-chan rune) {

	// Initialize global Worker Pool
	workerPoolMutex.Lock()
	globalWorkerPool = NewWorkerPool(p.Threads)
	workerPoolMutex.Unlock()

	// Clean up Worker Pool on program exit
	defer func() {
		workerPoolMutex.Lock()
		if globalWorkerPool != nil {
			globalWorkerPool.Stop()
			globalWorkerPool = nil
		}
		workerPoolMutex.Unlock()
	}()

	// TODO: Create a 2D slice to store the world.
	world := make([][]byte, p.ImageHeight)
	for i := range world {
		world[i] = make([]byte, p.ImageWidth)
	}

	// Request the io goroutine to read the image file
	c.ioCommand <- ioInput
	c.ioFilename <- fmt.Sprintf("%dx%d", p.ImageWidth, p.ImageHeight)

	// Read the image data from the io goroutine
	for y := 0; y < p.ImageHeight; y++ {
		for x := 0; x < p.ImageWidth; x++ {
			val := <-c.ioInput
			world[y][x] = val
		}
	}

	// cell flipped event
	for y := 0; y < p.ImageHeight; y++ {
		for x := 0; x < p.ImageWidth; x++ {
			if world[y][x] == 255 {
				c.events <- CellFlipped{0, util.Cell{X: x, Y: y}}
			}
		}
	}

	// AliveCellsCount event management timestamp
	lastAliveTime := time.Now()

	turn := 0
	completedTurns := 0
	isPaused := false
	done := false

	// Initial state notification
	c.events <- StateChange{turn, Executing}

	// Add logic to exit immediately if p.Turns is 0
	if p.Turns == 0 {
		done = true
	}

	// Remove separate ticker goroutine: AliveCellsCount is sent every 2 seconds in the main loop

	// Main game loop
	for !done {
		select {
		case key := <-keyPresses:
			switch key {
			case 'p':
				isPaused = !isPaused
				if isPaused {
					c.events <- StateChange{completedTurns, Paused}
				} else {
					c.events <- StateChange{completedTurns, Executing}
				}
			case 's':
				c.ioCommand <- ioOutput
				filename := fmt.Sprintf("%dx%dx%d", p.ImageWidth, p.ImageHeight, completedTurns)
				c.ioFilename <- filename
				for y := 0; y < p.ImageHeight; y++ {
					for x := 0; x < p.ImageWidth; x++ {
						c.ioOutput <- world[y][x]
					}
				}
				// Make sure the I/O has finished
				c.ioCommand <- ioCheckIdle
				<-c.ioIdle
				c.events <- ImageOutputComplete{completedTurns, filename}
			case 'q':
				done = true
			}
		default:
			// Send AliveCellsCount event every 2 seconds
			if time.Since(lastAliveTime) >= 2*time.Second {
				alive := 0
				for y := 0; y < p.ImageHeight; y++ {
					for x := 0; x < p.ImageWidth; x++ {
						if world[y][x] == 255 {
							alive++
						}
					}
				}
				c.events <- AliveCellsCount{completedTurns, alive}
				lastAliveTime = time.Now()
			}
			if !isPaused && turn < p.Turns {
				// 1. Calculate next state and send CellsFlipped event
				world = CalculateNextState(
					p,
					p.Threads,
					world,
					c,
					turn,
				)

				// 2. Handle turn completion
				completedTurns++

				// 3. Send TurnComplete event
				c.events <- TurnComplete{completedTurns}

				turn++

				// Check if all turns are completed
				if turn >= p.Turns {
					done = true
				}
			}
			if isPaused {
				time.Sleep(1 * time.Millisecond)
			}
		}
	}

	// Calculate alive cells
	var aliveCells []util.Cell
	for y := 0; y < p.ImageHeight; y++ {
		for x := 0; x < p.ImageWidth; x++ {
			if world[y][x] == 255 {
				aliveCells = append(aliveCells, util.Cell{X: x, Y: y})
			}
		}
	}

	// Final events in order
	// 1. FinalTurnComplete
	c.events <- FinalTurnComplete{CompletedTurns: completedTurns, Alive: aliveCells}

	// 2. Save final state
	c.ioCommand <- ioOutput
	filename := fmt.Sprintf("%dx%dx%d", p.ImageWidth, p.ImageHeight, completedTurns)
	c.ioFilename <- filename
	for y := 0; y < p.ImageHeight; y++ {
		for x := 0; x < p.ImageWidth; x++ {
			c.ioOutput <- world[y][x]
		}
	}
	c.ioCommand <- ioCheckIdle
	<-c.ioIdle

	// 3. ImageOutputComplete
	c.events <- ImageOutputComplete{completedTurns, filename}

	// 4. StateChange Quitting
	c.events <- StateChange{completedTurns, Quitting}

	// 5. Close event channel
	close(c.events)
}
