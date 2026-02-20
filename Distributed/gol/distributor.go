package gol

import (
	"fmt"
	"log"
	"net/rpc"
	"time"

	"uk.ac.bris.cs/gameoflife/stubs"
	"uk.ac.bris.cs/gameoflife/util"
)

type distributorChannels struct {
	events     chan<- Event
	ioCommand  chan<- ioCommand
	ioIdle     <-chan bool
	ioFilename chan<- string
	ioOutput   chan<- uint8
	ioInput    <-chan uint8
}

func handleOutput(p Params, c distributorChannels, world [][]uint8, t int) {
	c.ioCommand <- ioOutput
	outFilename := fmt.Sprintf("%vx%vx%v", p.ImageWidth, p.ImageHeight, t)
	c.ioFilename <- outFilename
	for y := 0; y < p.ImageHeight; y++ {
		for x := 0; x < p.ImageWidth; x++ {
			c.ioOutput <- world[y][x]
		}
	}
	c.ioCommand <- ioCheckIdle
	<-c.ioIdle

	c.events <- ImageOutputComplete{
		CompletedTurns: t,
		Filename:       outFilename,
	}
}

func setupRPCConnection() *rpc.Client {
	//client, err := rpc.Dial("tcp", "localhost:8030") // Connect to local Broker for testing
	client, err := rpc.Dial("tcp", "13.48.147.48:8030") // Connect to AWS Broker (Public IP)
	if err != nil {
		log.Fatal("Failed connecting:", err)
	}
	return client
}

func initializeWorld(p Params, c distributorChannels) [][]uint8 {
	// TODO: Create a 2D slice to store the world.
	world := make([][]uint8, p.ImageHeight)
	for i := range world {
		world[i] = make([]uint8, p.ImageWidth)
	}

	filename := fmt.Sprintf("%vx%v", p.ImageWidth, p.ImageHeight)

	// Request the io goroutine to read the image file
	c.ioCommand <- ioInput
	c.ioFilename <- filename
	for y := 0; y < p.ImageHeight; y++ {
		for x := 0; x < p.ImageWidth; x++ {
			num := <-c.ioInput
			world[y][x] = num
		}
	}

	// Send initial CellFlipped events for alive cells (same as parallel version)
	for y := 0; y < p.ImageHeight; y++ {
		for x := 0; x < p.ImageWidth; x++ {
			if world[y][x] == 255 {
				c.events <- CellFlipped{
					CompletedTurns: 0,
					Cell:           util.Cell{X: x, Y: y},
				}
			}
		}
	}

	return world
}

func startGameProcessing(client *rpc.Client, world [][]uint8, p Params, c distributorChannels) {
	request := &stubs.EngineRequest{
		World:       world,
		ImageWidth:  p.ImageWidth,
		ImageHeight: p.ImageHeight,
		Turns:       p.Turns,
		RuleSet:     p.RuleSet,
	}
	response := new(stubs.EngineResponse)

	err := client.Call(stubs.Process, request, response)
	if err != nil {
		log.Fatal("Error calling Process:", err)
	}

	// Send StateChange event to indicate processing started
	c.events <- StateChange{
		CompletedTurns: 0,
		NewState:       Executing,
	}
}

func handleKeyInput(key rune, client *rpc.Client, p Params, c distributorChannels, paused *bool) bool {
	switch key {
	case 's':
		getWorldRequest := &stubs.GetWorldRequest{}
		getWorldResponse := new(stubs.GetWorldResponse)
		err := client.Call(stubs.GetWorld, getWorldRequest, getWorldResponse)
		if err != nil {
			log.Println("Error calling GetWorld:", err)
		} else {
			worldSnapshot := getWorldResponse.World
			turn := getWorldResponse.CompletedTurns
			handleOutput(p, c, worldSnapshot, turn)
		}
	case 'q':
		// Send stop command to broker
		stopRequest := &stubs.StopRequest{}
		stopResponse := new(stubs.StopResponse)
		err := client.Call(stubs.StopProcessing, stopRequest, stopResponse)
		if err != nil {
			log.Println("Error calling StopProcessing:", err)
		}
		return true // done = true
	case 'k':
		shutdownRequest := &stubs.ShutdownRequest{}
		shutdownResponse := new(stubs.ShutdownResponse)
		err := client.Call(stubs.Shutdown, shutdownRequest, shutdownResponse)
		if err != nil {
			log.Println("Error calling Shutdown:", err)
		}
		getWorldRequest := &stubs.GetWorldRequest{}
		getWorldResponse := new(stubs.GetWorldResponse)
		err = client.Call(stubs.GetWorld, getWorldRequest, getWorldResponse)
		if err != nil {
			log.Println("Error calling GetWorld:", err)
		} else {
			worldSnapshot := getWorldResponse.World
			turn := getWorldResponse.CompletedTurns
			handleOutput(p, c, worldSnapshot, turn)
		}
		return true // done = true
	case 'p':
		if !*paused {
			pauseRequest := &stubs.PauseRequest{}
			pauseResponse := new(stubs.PauseResponse)
			err := client.Call(stubs.Pause, pauseRequest, pauseResponse)
			if err != nil {
				log.Println("Error calling Pause:", err)
			} else {
				fmt.Printf("Paused at turn %d\n", pauseResponse.Turn)
				*paused = true
				c.events <- StateChange{
					CompletedTurns: pauseResponse.Turn,
					NewState:       Paused,
				}
			}
		} else {
			resumeRequest := &stubs.ResumeRequest{}
			resumeResponse := new(stubs.ResumeResponse)
			err := client.Call(stubs.Resume, resumeRequest, resumeResponse)
			if err != nil {
				log.Println("Error calling Resume:", err)
			} else {
				fmt.Println("Continuing")
				*paused = false
				getWorldRequest := &stubs.GetWorldRequest{}
				getWorldResponse := new(stubs.GetWorldResponse)
				err = client.Call(stubs.GetWorld, getWorldRequest, getWorldResponse)
				if err == nil {
					c.events <- StateChange{
						CompletedTurns: getWorldResponse.CompletedTurns,
						NewState:       Executing,
					}
				}
			}
		}
	}
	return false // done = false
}

func sendAliveCellsCount(client *rpc.Client, c distributorChannels) {
	// Send AliveCellsCount event every 2 seconds
	countRequest := &stubs.AliveCellsCountRequest{}
	countResponse := new(stubs.AliveCellsCountResponse)
	err := client.Call(stubs.GetAliveCells, countRequest, countResponse)
	if err == nil && countResponse.CompletedTurns > 0 {
		aliveReport := AliveCellsCount{
			CompletedTurns: countResponse.CompletedTurns,
			CellsCount:     countResponse.CellsCount,
		}
		c.events <- aliveReport
	}
}

func processGameState(client *rpc.Client, p Params, c distributorChannels, previousTurn *int, previousWorld *[][]uint8) bool {
	// Check processing status and handle events
	getWorldRequest := &stubs.GetWorldRequest{}
	getWorldResponse := new(stubs.GetWorldResponse)
	err := client.Call(stubs.GetWorld, getWorldRequest, getWorldResponse)
	if err != nil {
		time.Sleep(10 * time.Millisecond)
		return false
	}

	currentTurn := getWorldResponse.CompletedTurns
	currentWorld := getWorldResponse.World

	// When a new turn is completed
	if currentTurn > *previousTurn {
		// Compare previous world and current world to send only changed cells as CellsFlipped event
		if *previousWorld != nil && currentTurn > 1 {
			var flippedCells []util.Cell
			for y := 0; y < p.ImageHeight; y++ {
				for x := 0; x < p.ImageWidth; x++ {
					// Add only cells that changed state
					if currentWorld[y][x] != (*previousWorld)[y][x] {
						flippedCells = append(flippedCells, util.Cell{X: x, Y: y})
					}
				}
			}

			// Send event only when there are changed cells
			if len(flippedCells) > 0 {
				c.events <- CellsFlipped{
					CompletedTurns: currentTurn,
					Cells:          flippedCells,
				}
			}
		}

		// Send TurnComplete event
		c.events <- TurnComplete{
			CompletedTurns: currentTurn,
		}

		// Save current world state as previous world
		if currentWorld != nil {
			*previousWorld = make([][]uint8, p.ImageHeight)
			for i := 0; i < p.ImageHeight; i++ {
				(*previousWorld)[i] = make([]uint8, p.ImageWidth)
				copy((*previousWorld)[i], currentWorld[i])
			}
		}

		*previousTurn = currentTurn
	}

	// Check game completion (same logic as parallel version)
	if !getWorldResponse.Processing {
		return true // done = true (auto termination)
	}

	return false // done = false
}

func finalizeGame(client *rpc.Client, p Params, c distributorChannels) {
	finalWorldRequest := &stubs.GetWorldRequest{}
	finalWorldResponse := new(stubs.GetWorldResponse)
	err := client.Call(stubs.GetWorld, finalWorldRequest, finalWorldResponse)
	if err != nil {
		log.Println("Error calling GetWorld:", err)
	} else {
		world := finalWorldResponse.World
		turn := finalWorldResponse.CompletedTurns

		aliveCells := []util.Cell{}
		for y := 0; y < p.ImageHeight; y++ {
			for x := 0; x < p.ImageWidth; x++ {
				if world[y][x] == 255 {
					aliveCells = append(aliveCells, util.Cell{X: x, Y: y})
				}
			}
		}

		handleOutput(p, c, world, turn)

		c.events <- FinalTurnComplete{
			CompletedTurns: turn,
			Alive:          aliveCells,
		}

		c.events <- StateChange{
			CompletedTurns: turn,
			NewState:       Quitting,
		}
	}
}

// distributor divides the work between workers and interacts with other goroutines.
func distributor(p Params, c distributorChannels, keyPresses <-chan rune) {
	// Initialize world and read input data
	world := initializeWorld(p, c)

	// Setup RPC connection to broker
	client := setupRPCConnection()
	defer client.Close()

	// Start game processing on broker
	startGameProcessing(client, world, p, c)

	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()

	done := false
	paused := false

	// Variables for processing state checking
	previousTurn := 0
	var previousWorld [][]uint8

	// Main game loop (similar structure to parallel version)
	for !done {
		select {
		case key := <-keyPresses:
			done = handleKeyInput(key, client, p, c, &paused)

		case <-ticker.C:
			sendAliveCellsCount(client, c)

		default:
			done = processGameState(client, p, c, &previousTurn, &previousWorld)
			// Adjust CPU usage
			time.Sleep(10 * time.Millisecond)
		}
	}

	// Finalize game and send final events
	finalizeGame(client, p, c)

	close(c.events)
}
