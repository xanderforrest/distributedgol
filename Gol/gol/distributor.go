package gol

import (
	"net/rpc"
	"strconv"
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

//Counts the alive neighbours of a cell
func getLiveNeighbours(p Params, world [][]byte, a, b int) int {
	var alive = 0
	var widthLeft int
	var widthRight int
	var heightUp int
	var heightDown int

	if a == 0 {
		widthLeft = p.ImageWidth - 1
	} else {
		widthLeft = a - 1
	}
	if a == p.ImageWidth-1 {
		widthRight = 0
	} else {
		widthRight = a + 1
	}

	if b == 0 {
		heightUp = p.ImageHeight - 1
	} else {
		heightUp = b - 1
	}

	if b == p.ImageHeight-1 {
		heightDown = 0
	} else {
		heightDown = b + 1
	}

	if isAlive(world[widthLeft][b]) {
		alive = alive + 1
	}
	if isAlive(world[widthRight][b]) {
		alive = alive + 1
	}
	if isAlive(world[widthLeft][heightUp]) {
		alive = alive + 1
	}
	if isAlive(world[a][heightUp]) {
		alive = alive + 1
	}
	if isAlive(world[widthRight][heightUp]) {
		alive = alive + 1
	}
	if isAlive(world[widthLeft][heightDown]) {
		alive = alive + 1
	}
	if isAlive(world[a][heightDown]) {
		alive = alive + 1
	}
	if isAlive(world[widthRight][heightDown]) {
		alive = alive + 1
	}
	return alive
}

//check if cell is alive
func isAlive(cell byte) bool {
	if cell == 255 {
		return true
	}
	return false
}

//calculates the next state of the world for single threaded
func calculateNextState(p Params, world [][]byte) [][]byte {
	newWorld := make([][]byte, p.ImageWidth)
	for i := range newWorld {
		newWorld[i] = make([]byte, p.ImageHeight)
	}
	for i := 0; i < p.ImageHeight; i++ {
		for j := 0; j < p.ImageWidth; j++ {
			neighbours := getLiveNeighbours(p, world, i, j)
			if world[i][j] == 0xff && (neighbours < 2 || neighbours > 3) {
				newWorld[i][j] = 0x0
			} else if world[i][j] == 0x0 && neighbours == 3 {
				newWorld[i][j] = 0xff
			} else {
				newWorld[i][j] = world[i][j]
			}
		}
	}
	return newWorld
}

//calculates the next state of the world for multiple threaded
func calculateNextStateByThread(p Params, world [][]byte, startY, endY int) [][]byte {

	newWorld := make([][]byte, p.ImageWidth)
	for i := range newWorld {
		newWorld[i] = make([]byte, endY+1)
	}

	for i := startY; i < endY; i++ {
		for j := 0; j < p.ImageWidth; j++ {
			neighbours := getLiveNeighbours(p, world, i, j)
			if world[i][j] == 0xff && (neighbours < 2 || neighbours > 3) {
				newWorld[i][j] = 0x0
			} else if world[i][j] == 0x0 && neighbours == 3 {
				newWorld[i][j] = 0xff
			} else {
				newWorld[i][j] = world[i][j]
			}
		}
	}
	return newWorld
}

//create a list of alive cells
func calculateAliveCells(p Params, world [][]byte) []util.Cell {
	newCell := []util.Cell{}
	for i := 0; i < p.ImageHeight; i++ {
		for j := 0; j < p.ImageWidth; j++ {
			if world[i][j] == 0xff {
				newCell = append(newCell, util.Cell{j, i})
			}
		}
	}
	return newCell
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
func worker(p Params, world [][]byte, startX, endX, startY, endY int, out chan<- [][]uint8) {
	imagePart := calculateNextStateByThread(p, world, startY, endY)
	out <- imagePart
}

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

	//calculate next state depending on the number of threads

	if p.Threads == 1 {

		client, _ := rpc.Dial("tcp", "127.0.0.1:8030")
		defer client.Close()

		golArgs := stubs.GolArgs{Height: p.ImageHeight, Width: p.ImageWidth, Turns: p.Turns, World: world}
		response := new(stubs.GolAliveCells)
		client.Call(stubs.ProcessTurns, golArgs, response)
		c.events <- FinalTurnComplete{response.TurnsComplete, response.AliveCells}

	} else {
		return
	}

	//write final state of the world to pgm image

	c.ioCommand <- ioOutput
	c.ioFilename <- filename + "x" + strconv.Itoa(turns)

	for i := 0; i < imageHeight; i++ {
		for j := 0; j < imageWidth; j++ {
			c.ioOutput <- world[i][j]
		}
	}

	// Make sure that the Io has finished any output before exiting.
	c.ioCommand <- ioCheckIdle
	<-c.ioIdle

	c.events <- StateChange{turn, Quitting}

	// Close the channel to stop the SDL goroutine gracefully. Removing may cause deadlock.
	close(c.events)
}

/*
// distributor divides the work between workers and interacts with other goroutines.
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

	for _, cell := range calculateAliveCells(p, world) {
		c.events <- CellFlipped{0, cell}
	}

	var m sync.Mutex

	//ticker that reports the number of alive cells every two seconds

	ticker := time.NewTicker(2 * time.Second)
	done := make(chan bool)

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

	//keypress
	var ok = 1
	go func() {
		for {
			switch <-c.keyPresses {
			case 'p':
				if ok == 1 {
					fmt.Printf("The current turn that is being processed: %d\n", turn)
					ok = 0
					m.Lock()
				} else if ok == 0 {
					fmt.Println("Continuing... \n")
					ok = 1
					m.Unlock()
				}
			case 'q':
				if ok == 1 {
					c.ioCommand <- ioOutput
					c.ioFilename <- filename + "x" + strconv.Itoa(p.Turns)

					for i := 0; i < imageHeight; i++ {
						for j := 0; j < imageWidth; j++ {
							c.ioOutput <- world[i][j]
						}
					}

					time.Sleep(1 * time.Second)

					alive := calculateAliveCells(p, world)
					c.events <- FinalTurnComplete{turn, alive}

					ticker.Stop()
					done <- true
					// Make sure that the Io has finished any output before exiting.
					c.ioCommand <- ioCheckIdle
					<-c.ioIdle

					c.events <- StateChange{turn, Quitting}

					// Close the channel to stop the SDL goroutine gracefully. Removing may cause deadlock.
					close(c.events)
				} else {
					fmt.Println("Pressed the wrong key. try again \n")
				}
			case 's':
				if ok == 1 {
					c.ioCommand <- ioOutput
					c.ioFilename <- filename + "x" + strconv.Itoa(turns)

					for i := 0; i < imageHeight; i++ {
						for j := 0; j < imageWidth; j++ {
							c.ioOutput <- world[i][j]
						}
					}
					time.Sleep(1 * time.Second)
				} else {
					fmt.Println("Pressed wrong key. try again \n")
				}
			}
		}
	}()

	//calculate next state depending on the number of threads

	if p.Threads == 1 {

		for turn < turns {
			m.Lock()
			oldWorld := append(world)
			world = calculateNextState(p, world)
			compareWorlds(oldWorld, world, &c, turn+1, p)
			turn++
			c.events <- TurnComplete{CompletedTurns: turn}
			m.Unlock()
		}
	} else {
		for turn < turns {
			workerHeight := p.ImageHeight / p.Threads
			out := make([]chan [][]uint8, p.Threads)
			for i := range out {
				out[i] = make(chan [][]uint8)
			}
			m.Lock()
			for i := 0; i < p.Threads; i++ {
				go worker(p, world, i*workerHeight, (i+1)*workerHeight, 0, p.ImageWidth, out[i])
			}

			var newPixelData [][]uint8

			newPixelData = makeMatrix(0, 0)

			for i := 0; i < p.Threads; i++ {
				part := <-out[i]
				newPixelData = append(newPixelData, part...)
			}
			compareWorlds(world, newPixelData, &c, turn, p)
			world = newPixelData
			turn++
			c.events <- TurnComplete{CompletedTurns: turn}
			m.Unlock()
		}

	}

	c.events <- FinalTurnComplete{turns, calculateAliveCells(p, world)}

	ticker.Stop()
	done <- true

	//write final state of the world to pgm image

	c.ioCommand <- ioOutput
	c.ioFilename <- filename + "x" + strconv.Itoa(turns)

	for i := 0; i < imageHeight; i++ {
		for j := 0; j < imageWidth; j++ {
			c.ioOutput <- world[i][j]
		}
	}

	// Make sure that the Io has finished any output before exiting.
	c.ioCommand <- ioCheckIdle
	<-c.ioIdle

	c.events <- StateChange{turn, Quitting}

	// Close the channel to stop the SDL goroutine gracefully. Removing may cause deadlock.
	close(c.events)
}
*/
