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

func distributor(p Params, c distributorChannels, keyPresses <-chan rune) {
	world := make([][]uint8, p.ImageHeight)
	for i := range world {
		world[i] = make([]uint8, p.ImageWidth)
	}

	filename := fmt.Sprintf("%vx%v", p.ImageWidth, p.ImageHeight)

	c.ioCommand <- ioInput
	c.ioFilename <- filename
	for y := 0; y < p.ImageHeight; y++ {
		for x := 0; x < p.ImageWidth; x++ {
			num := <-c.ioInput
			world[y][x] = num
		}
	}

	// 초기 살아있는 셀들에 대한 CellFlipped 이벤트 전송 (parallel 버전과 동일)
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

	//client, err := rpc.Dial("tcp", "localhost:8030") // Connect to local Broker for testing
	client, err := rpc.Dial("tcp", "13.48.147.48:8030") // Connect to AWS Broker (Public IP)
	if err != nil {
		log.Fatal("Failed connecting:", err)
	}
	defer client.Close()

	request := &stubs.EngineRequest{
		World:       world,
		ImageWidth:  p.ImageWidth,
		ImageHeight: p.ImageHeight,
		Turns:       p.Turns,
	}
	response := new(stubs.EngineResponse)

	err = client.Call(stubs.Process, request, response)
	if err != nil {
		log.Fatal("Error calling Process:", err)
	}

	// 처리 시작을 알리는 StateChange 이벤트 전송
	c.events <- StateChange{
		CompletedTurns: 0,
		NewState:       Executing,
	}

	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()

	done := false
	paused := false

	// 처리 상태 체크용 변수들
	previousTurn := 0
	var previousWorld [][]uint8

	// 메인 게임 루프 (parallel 버전과 유사한 구조)
	for !done {
		select {
		case key := <-keyPresses:
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
				// 브로커에게 중지 명령 전송
				stopRequest := &stubs.StopRequest{}
				stopResponse := new(stubs.StopResponse)
				err := client.Call(stubs.StopProcessing, stopRequest, stopResponse)
				if err != nil {
					log.Println("Error calling StopProcessing:", err)
				}
				done = true
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
				done = true
			case 'p':
				if !paused {
					pauseRequest := &stubs.PauseRequest{}
					pauseResponse := new(stubs.PauseResponse)
					err := client.Call(stubs.Pause, pauseRequest, pauseResponse)
					if err != nil {
						log.Println("Error calling Pause:", err)
					} else {
						fmt.Printf("Paused at turn %d\n", pauseResponse.Turn)
						paused = true
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
						paused = false
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

		case <-ticker.C:
			// 2초마다 AliveCellsCount 이벤트 전송
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

		default:
			// 처리 상태 확인 및 이벤트 처리
			getWorldRequest := &stubs.GetWorldRequest{}
			getWorldResponse := new(stubs.GetWorldResponse)
			err := client.Call(stubs.GetWorld, getWorldRequest, getWorldResponse)
			if err != nil {
				time.Sleep(10 * time.Millisecond)
				continue
			}

			currentTurn := getWorldResponse.CompletedTurns
			currentWorld := getWorldResponse.World

			// 새로운 턴이 완료되었을 때
			if currentTurn > previousTurn {
				// 이전 world와 현재 world를 비교하여 변경된 셀들만 CellsFlipped 이벤트로 전송
				if previousWorld != nil && currentTurn > 1 {
					var flippedCells []util.Cell
					for y := 0; y < p.ImageHeight; y++ {
						for x := 0; x < p.ImageWidth; x++ {
							// 셀 상태가 변경된 경우만 추가
							if currentWorld[y][x] != previousWorld[y][x] {
								flippedCells = append(flippedCells, util.Cell{X: x, Y: y})
							}
						}
					}

					// 변경된 셀이 있을 때만 이벤트 전송
					if len(flippedCells) > 0 {
						c.events <- CellsFlipped{
							CompletedTurns: currentTurn,
							Cells:          flippedCells,
						}
					}
				}

				// TurnComplete 이벤트 전송
				c.events <- TurnComplete{
					CompletedTurns: currentTurn,
				}

				// 현재 world 상태를 이전 world로 저장
				if currentWorld != nil {
					previousWorld = make([][]uint8, p.ImageHeight)
					for i := 0; i < p.ImageHeight; i++ {
						previousWorld[i] = make([]uint8, p.ImageWidth)
						copy(previousWorld[i], currentWorld[i])
					}
				}

				previousTurn = currentTurn
			}

			// 게임 완료 체크 (parallel 버전과 동일한 로직)
			if !getWorldResponse.Processing {
				done = true // 자동 종료
			}

			// CPU 사용률 조절
			time.Sleep(10 * time.Millisecond)
		}
	}

	finalWorldRequest := &stubs.GetWorldRequest{}
	finalWorldResponse := new(stubs.GetWorldResponse)
	err = client.Call(stubs.GetWorld, finalWorldRequest, finalWorldResponse)
	if err != nil {
		log.Println("Error calling GetWorld:", err)
	} else {
		world = finalWorldResponse.World
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

	close(c.events)
}
