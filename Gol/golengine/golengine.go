package main

import (
	"flag"
	"fmt"
	"math/rand"
	"net"
	"net/rpc"
	"time"
	"uk.ac.bris.cs/gameoflife/stubs"
	"uk.ac.bris.cs/gameoflife/util"
)

//these are functions - they can't be accessed by local controler via rpc

func getLiveNeighbours(p stubs.Request, world [][]byte, a, b int) int {
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
func calculateNextState(p stubs.Request, world [][]byte) [][]byte {
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

//create a list of alive cells
func calculateAliveCells(p stubs.Request, world [][]byte) []util.Cell {
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

type GameOfLifeOperations struct{}

//these are methods - can be accessed by local controller via rpc
func (s *GameOfLifeOperations) ProcessTurns(req stubs.Request, res *stubs.Response) (err error) {
	fmt.Println("Processing turns... remotely.... so cool")
	turns := req.Turns
	turn := 0
	world := append(req.InitialWorld)

	for turn < turns {
		world = calculateNextState(req, world)
		turn++
	}

	//res.TurnsComplete = turns
	//res.AliveCells = calculateAliveCells(args.Width, args.Height, world)
	fmt.Println("Returning info... so cool pt2")
	return
}

/* -------- template method
func (s *SecretStringOperations) FastReverse(req stubs.Request, res *stubs.Response) (err error) {
	if req.Message == "" {
		err = errors.New("A message must be specified")
		return
	}

	res.Message = ReverseString(req.Message, 2)
	return
}
*/
func main() {
	pAddr := flag.String("port", "8030", "Port to listen on")
	flag.Parse()
	rand.Seed(time.Now().UnixNano())

	//when you register this type, its methods will be able to be called remotely
	rpc.Register(&GameOfLifeOperations{})

	//this closes it when all the things are done
	listener, _ := net.Listen("tcp", ":"+*pAddr)
	defer listener.Close()
	rpc.Accept(listener)
}
