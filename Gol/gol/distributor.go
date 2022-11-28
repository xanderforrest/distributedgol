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

	turns := p.Turns
	turn := 0

	ticker := time.NewTicker(2 * time.Second)

	//calculate next state depending on the number of threads

	client, _ := rpc.Dial("tcp", "127.0.0.1:8030")
	defer client.Close()

	golArgs := stubs.GolArgs{Height: p.ImageHeight, Width: p.ImageWidth, Turns: p.Turns, World: world}
	response := new(stubs.GolAliveCells)

	rpcCall := client.Go(stubs.ProcessTurns, golArgs, response, nil)
	var returnedCells []util.Cell
	for {
		select {
		case <-ticker.C:
			fmt.Println("Ticker has ticked client side")
			//yada
			tickResponse := new(stubs.TickReport)
			client.Call(stubs.DoTick, true, tickResponse)
			c.events <- AliveCellsCount{tickResponse.Turns, tickResponse.AliveCount}
		case <-rpcCall.Done:
			fmt.Println("RPC call is done")
			returnedCells = response.AliveCells
			c.events <- FinalTurnComplete{response.TurnsComplete, response.AliveCells}
			goto Exit
		}
	}
Exit:
	fmt.Println("Escaped")

	//write final state of the world to pgm image

	c.ioCommand <- ioOutput
	c.ioFilename <- filename + "x" + strconv.Itoa(turns)

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

	c.events <- StateChange{turn, Quitting}

	// Close the channel to stop the SDL goroutine gracefully. Removing may cause deadlock.
	close(c.events)
}
