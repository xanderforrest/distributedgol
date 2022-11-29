package gol

import (
	"fmt"
	"net/rpc"
	"os"
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

func savePGM(p Params, c distributorChannels, aliveCells []util.Cell, turns int) {
	c.ioCommand <- ioOutput
	c.ioFilename <- strconv.Itoa(p.ImageWidth) + "x" + strconv.Itoa(p.ImageHeight) + "x" + strconv.Itoa(turns)

	for i := 0; i < p.ImageHeight; i++ {
		for j := 0; j < p.ImageWidth; j++ {
			var value byte = 0
			for _, cell := range aliveCells {
				if cell.X == j && cell.Y == i {
					value = 255
				}
			}
			c.ioOutput <- value
		}
	}
	fmt.Println("Finished saving PGM: " + strconv.Itoa(p.ImageWidth) + "x" + strconv.Itoa(p.ImageHeight) + "x" + strconv.Itoa(turns))
}

func distributor(p Params, c distributorChannels) {
	imageHeight := p.ImageHeight
	imageWidth := p.ImageWidth

	c.ioCommand <- ioInput
	c.ioFilename <- strconv.Itoa(p.ImageWidth) + "x" + strconv.Itoa(p.ImageHeight)

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

	status := new(stubs.EngineStatus)
	client.Call(stubs.CheckStatus, true, status)
	if status.Working {
		fmt.Println("Resuming connection to Engine, which is at turn: " + strconv.Itoa(status.Turn))
	} else {
		fmt.Println("Sending job to Engine.")
	}

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
					fmt.Printf("Instructing workers to resume... (continuing)")
					resumedTurn := new(stubs.EngineStatus)
					client.Call(stubs.ResumeEngine, true, resumedTurn)
					workersPaused = false
					fmt.Println("Workers resumed at turn: " + strconv.Itoa(resumedTurn.Turn))
				} else {
					fmt.Println("Instructing workers to pause...")
					pausedTurn := new(stubs.EngineStatus)
					client.Call(stubs.PauseEngine, true, pausedTurn)
					workersPaused = true
					fmt.Println("Workers paused at turn: " + strconv.Itoa(pausedTurn.Turn))
				}
			case 'q':
				if workersPaused {
					fmt.Println("All execution currently paused. Please resume to quit the world.")
				} else {
					fmt.Println("Quitting, closing client side.")
					c.ioCommand <- ioCheckIdle
					<-c.ioIdle
					close(c.events)
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

					savePGM(p, c, returnedCells, turnsComplete)
				}
			case 'k':
				if workersPaused {
					fmt.Println("All excecution currently paused. Please resume to shutdown Engines.")
				} else {
					fmt.Println("Saving PGM...")
					earlyResponse := new(stubs.GolAliveCells)
					client.Call(stubs.InterruptEngine, true, earlyResponse)

					returnedCells = earlyResponse.AliveCells
					turnsComplete = earlyResponse.TurnsComplete

					savePGM(p, c, returnedCells, turnsComplete)

					fmt.Println("Shutting down Engines...")
					client.Call(stubs.KillEngine, true, true)
					fmt.Println("Engine shut down.")
					c.ioCommand <- ioCheckIdle
					<-c.ioIdle
					os.Exit(0)
				}
			}
		case <-rpcCall.Done:
			fmt.Println("===== Engine has finished processing turns =====")
			returnedCells = response.AliveCells
			turnsComplete = response.TurnsComplete

			c.events <- FinalTurnComplete{turnsComplete, returnedCells}
			goto Exit
		}
	}

Exit:
	fmt.Println("Saving PGM & shutting down controller.")
	savePGM(p, c, returnedCells, turnsComplete)

	// Make sure that the Io has finished any output before exiting.
	c.ioCommand <- ioCheckIdle
	<-c.ioIdle

	c.events <- StateChange{turnsComplete, Quitting}
	close(c.events)

}
