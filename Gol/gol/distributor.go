package gol

import (
	"fmt"
	"net/rpc"
	"strconv"
	"time"
	"uk.ac.bris.cs/gameoflife/gol/stubs"
	"uk.ac.bris.cs/gameoflife/util"
)

type distributorChannels struct {
	events     chan<- Event
	ioCommand  chan<- ioCommand
	ioIdle     <-chan bool
	ioFilename chan<- string
	ioOutput   chan<- uint8
	ioInput    <-chan uint8
	keyPresses <-chan rune
}

//count the number of alive cells
func calculateCount(p Params, world [][]byte) int {
	sum := 0
	for i := 0; i < p.ImageHeight; i++ {
		for j := 0; j < p.ImageWidth; j++ {
			if world[i][j] != 0 {
				sum++
			}
		}
	}
	return sum
}

//worker function for multiple threaded case

//make an uninitialised matrix
func makeMatrix(height, width int) [][]uint8 {
	matrix := make([][]uint8, height)
	for i := range matrix {
		matrix[i] = make([]uint8, width)
	}
	return matrix
}

//report the CellFlipped event when a cell changes state
func compareWorlds(old, new [][]byte, c *distributorChannels, turn int, p Params) {
	for i := 0; i < p.ImageHeight; i++ {
		for j := 0; j < p.ImageWidth; j++ {
			if old[i][j] != new[i][j] {
				c.events <- CellFlipped{turn, util.Cell{j, i}}
			}
		}
	}
}

func distributor(p Params, c distributorChannels) {
	imageHeight := p.ImageHeight
	imageWidth := p.ImageWidth

	heightString := strconv.Itoa(imageHeight)
	widthString := strconv.Itoa(imageWidth)

	filename := heightString + "x" + widthString

	//read in the initial configuration of the world

	c.ioCommand <- ioInput

	c.ioFilename <- filename

	world := make([][]uint8, imageHeight)
	for i := 0; i < imageHeight; i++ {
		world[i] = make([]uint8, imageWidth)
		for j := range world[i] {
			byte := <-c.ioInput
			world[i][j] = byte
		}
	}

	ticker := time.NewTicker(2 * time.Second)

	client, _ := rpc.Dial("tcp", "127.0.0.1:8030")
	defer client.Close()

	golArgs := stubs.GolArgs{Height: p.ImageHeight, Width: p.ImageWidth, Turns: p.Turns, World: world}
	response := new(stubs.GolAliveCells)

	rpcCall := client.Go(stubs.ProcessTurns, golArgs, response, nil)

	var returnedCells []util.Cell
	var turnsComplete int
	var workersPaused = false

	for {
		select {
		case <-ticker.C:
			if workersPaused {
				fmt.Printf("Ignoring Tick as workers paused")
			} else {
				fmt.Println("Ticker has ticked client side")
				tickResponse := new(stubs.TickReport)
				client.Call(stubs.DoTick, true, tickResponse)
				c.events <- AliveCellsCount{tickResponse.Turns, tickResponse.AliveCount}
			}
		case kp := <-c.keyPresses:
			switch kp {
			case 'p':
				if workersPaused {
					fmt.Printf("Instructing workers to resume...")
					resumedTurn := new(stubs.CurrentTurn)
					client.Call(stubs.ResumeEngine, true, resumedTurn)
					workersPaused = false
					fmt.Println("Workers resumed at turn: " + strconv.Itoa(resumedTurn.Turn))
				} else {
					fmt.Println("Instructing workers to pause...")
					pausedTurn := new(stubs.CurrentTurn)
					client.Call(stubs.PauseEngine, true, pausedTurn)
					workersPaused = true
					fmt.Println("Workers paused at turn: " + strconv.Itoa(pausedTurn.Turn))
				}
			case 'q':
				if workersPaused {
					fmt.Println("All excecution currently paused. Please resume to quit the world.")
				} else {
					fmt.Println("Quitting, getting state from workers & saving PGM.")
					earlyResponse := new(stubs.GolAliveCells)
					client.Call(stubs.InterruptEngine, true, earlyResponse)
					returnedCells = earlyResponse.AliveCells
					turnsComplete = earlyResponse.TurnsComplete
					goto Exit
				}
			case 's':
				if workersPaused {
					fmt.Println("All excecution currently paused. Please resume to save the world.")
				} else {
					fmt.Println("Saving PGM...")
					earlyResponse := new(stubs.GolAliveCells)
					client.Call(stubs.InterruptEngine, true, earlyResponse)

					returnedCells = earlyResponse.AliveCells
					turnsComplete = earlyResponse.TurnsComplete

					c.ioCommand <- ioOutput
					c.ioFilename <- filename + "x" + strconv.Itoa(turnsComplete)

					for i := 0; i < imageHeight; i++ {
						for j := 0; j < imageWidth; j++ {
							var value byte = 0
							for _, cell := range returnedCells {
								if cell.X == i && cell.Y == j {
									value = 255
								}
							}
							c.ioOutput <- value
						}
					}
					fmt.Println("Finished saving PGM: " + filename + "x" + strconv.Itoa(turnsComplete))
				}
			}
		case <-rpcCall.Done:
			fmt.Println("RPC call is done")
			returnedCells = response.AliveCells
			turnsComplete = response.TurnsComplete

			c.events <- FinalTurnComplete{turnsComplete, returnedCells}
			goto Exit
		}
	}
Exit:
	fmt.Println("Saving PGM & shutting down controller.")

	//write final state of the world to pgm image

	c.ioCommand <- ioOutput
	c.ioFilename <- filename + "x" + strconv.Itoa(turnsComplete)

	for i := 0; i < imageHeight; i++ {
		for j := 0; j < imageWidth; j++ {
			var value byte = 0
			for _, cell := range returnedCells {
				if cell.X == i && cell.Y == j {
					value = 255
				}
			}
			c.ioOutput <- value
		}
	}

	// Make sure that the Io has finished any output before exiting.
	c.ioCommand <- ioCheckIdle
	<-c.ioIdle

	c.events <- StateChange{turnsComplete, Quitting}

	// Close the channel to stop the SDL goroutine gracefully. Removing may cause deadlock.
	close(c.events)
}
