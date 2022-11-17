package gol

import (
	"log"
	"net/rpc"
	"strconv"
	"sync"
	"time"
	"uk.ac.bris.cs/gameoflife/stubs"
)

type distributorChannels struct {
	events     chan<- Event
	ioCommand  chan<- ioCommand
	ioIdle     <-chan bool
	ioFilename chan<- string
	ioOutput   chan<- uint8
	ioInput    <-chan uint8
}

func makeCall(client *rpc.Client, world [][]byte, turns, height, width int) {
	request := stubs.Request{InitialWorld: world, Turns: turns, ImageHeight: height, ImageWidth: width}
	//this is a pointer
	response := new(stubs.Response)
	client.Call(stubs.ProcessTurnsHandler, request, response)
}

func loadWorld(p Params, c distributorChannels) [][]byte {
	heightString := strconv.Itoa(p.ImageHeight)
	widthString := strconv.Itoa(p.ImageWidth)

	filename := heightString + "x" + widthString

	c.ioCommand <- ioInput

	c.ioFilename <- filename

	initialWorld := make([][]uint8, p.ImageHeight)
	for i := 0; i < p.ImageHeight; i++ {
		initialWorld[i] = make([]uint8, p.ImageWidth)
		for j := range initialWorld[i] {
			byte := <-c.ioInput
			initialWorld[i][j] = byte
		}
	}
	return initialWorld
}

// distributor divides the work between workers and interacts with other goroutines.
func distributor(p Params, c distributorChannels) {

	// TODO: Create a 2D slice to store the world.

	//board1 := allocateBoard(p.ImageHeight, p.ImageWidth) do i need this? idk - keep the comment for later.

	initialWorld := loadWorld(p, c)

	turn := 0

	server := "127.0.0.1:8030"
	client, err := rpc.Dial("tcp", server)
	if err != nil {
		log.Fatal("dialing: ", err)
	}
	defer client.Close()

	//makeCall(client, initialWorld, p.Turns, p.ImageHeight, p.ImageWidth)

	request := stubs.Request{InitialWorld: initialWorld, Turns: p.Turns, ImageHeight: p.ImageHeight, ImageWidth: p.ImageWidth}
	response := new(stubs.Response)
	client.Call(stubs.ProcessTurnsHandler, request, response)

	ticker := time.NewTicker(2 * time.Second)
	done := make(chan bool)

	var m sync.Mutex

	go func() {
		for {
			select {
			case <-done:
				return
			case <-ticker.C:
				m.Lock()
				c.events <- AliveCellsCount{turn, calculateCount(p, world)}
				m.Unlock()
			}
		}
	}()

	// TODO: Report the final state using FinalTurnCompleteEvent.

	c.events <- FinalTurnComplete{
		CompletedTurns: response.CompletedTurns,
		Alive:          response.AliceCells,
	}

	// Make sure that the Io has finished any output before exiting.
	c.ioCommand <- ioCheckIdle
	<-c.ioIdle

	c.events <- StateChange{turn, Quitting}

	// Close the channel to stop the SDL goroutine gracefully. Removing may cause deadlock.
	close(c.events)
}
