package main

import (
	"flag"
	"fmt"
	"net"
	"net/rpc"
	"strconv"
	"sync"
	"uk.ac.bris.cs/gameoflife/gol/stubs"
	"uk.ac.bris.cs/gameoflife/util"
)

type GolEngine struct{}

var world [][]byte
var turn int
var m sync.Mutex
var width int
var height int

func isAlive(cell byte) bool {
	if cell == 255 {
		return true
	}
	return false
}

func getLiveNeighbours(width, height int, world [][]byte, a, b int) int {
	var alive = 0
	var widthLeft int
	var widthRight int
	var heightUp int
	var heightDown int

	if a == 0 {
		widthLeft = width - 1
	} else {
		widthLeft = a - 1
	}
	if a == width-1 {
		widthRight = 0
	} else {
		widthRight = a + 1
	}

	if b == 0 {
		heightUp = height - 1
	} else {
		heightUp = b - 1
	}

	if b == height-1 {
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

func calculateNextState(width, height int, world [][]byte) [][]byte {
	newWorld := make([][]byte, width)
	for i := range newWorld {
		newWorld[i] = make([]byte, height)
	}
	for i := 0; i < height; i++ {
		for j := 0; j < width; j++ {
			neighbours := getLiveNeighbours(width, height, world, i, j)
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

func calculateAliveCells(width, height int, world [][]byte) []util.Cell {
	newCell := []util.Cell{}
	for i := 0; i < height; i++ {
		for j := 0; j < width; j++ {
			if world[i][j] == 0xff {
				newCell = append(newCell, util.Cell{i, j})
			}
		}
	}
	return newCell
}

func calculateAliveCount(world [][]byte) int {
	count := 0
	for x := range world {
		for y := range world[x] {
			if isAlive(world[x][y]) {
				count++
			}
		}
	}
	return count
}

func (g *GolEngine) ProcessTurns(args stubs.GolArgs, res *stubs.GolAliveCells) (err error) {
	fmt.Println("Processing turns... remotely.... so cool")
	turns := args.Turns
	turn = 0
	world = args.World
	width = args.Width
	height = args.Height

	for turn < turns {
		m.Lock()
		fmt.Println(turn)
		world = calculateNextState(width, height, world)
		turn++
		m.Unlock()
	}

	res.TurnsComplete = turns
	res.AliveCells = calculateAliveCells(width, height, world)
	fmt.Println("Returning info... so cool pt2")
	return
}

func (g *GolEngine) DoTick(_ bool, res *stubs.TickReport) (err error) {
	fmt.Println("Got do tick request...")
	m.Lock()
	res.AliveCount = calculateAliveCount(world)
	res.Turns = turn
	m.Unlock()
	return
}

func (g *GolEngine) PauseEngine(_ bool, res *stubs.CurrentTurn) (err error) {
	m.Lock()
	fmt.Println("pausing engine on turn " + strconv.Itoa(turn) + "...")
	res.Turn = turn
	return
}

func (g *GolEngine) ResumeEngine(_ bool, res *stubs.CurrentTurn) (err error) {
	fmt.Println("resuming engine from turn " + strconv.Itoa(turn))
	res.Turn = turn
	m.Unlock()
	return
}

func (g *GolEngine) InterruptEngine(_ bool, res *stubs.GolAliveCells) (err error) {
	m.Lock()
	fmt.Println("Interrupt triggered, returning current work to controller.")

	res.TurnsComplete = turn
	res.AliveCells = calculateAliveCells(width, height, world)
	m.Unlock()
	return
}

func main() {
	fmt.Println("Super Cool Distributed Game of Life server is running...")
	pAddr := flag.String("port", "8030", "Port to listen on")
	flag.Parse()
	rpc.Register(&GolEngine{})
	listener, _ := net.Listen("tcp", ":"+*pAddr)
	defer listener.Close()
	rpc.Accept(listener)
}
