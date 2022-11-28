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

var world [][]byte
var turn = 0
var turns int
var m sync.Mutex
var width int
var height int
var started = false
var aliveCells []util.Cell
var engines = make(map[int]*rpc.Client)

type GolEngine struct{}

func startEngine(client *rpc.Client, id, engineHeight int, out chan<- []util.Cell) {
	args := stubs.EngineArgs{TotalWorld: world, TWidth: width, THeight: height, Height: engineHeight, Offset: engineHeight * id}
	response := new(stubs.EngineResponse)

	client.Call(stubs.ProcessTurn, args, response)
	out <- response.AliveCells
}

func emptyWorld() [][]byte {

	world = make([][]byte, width)
	for x := 0; x < width; x++ {
		world[x] = make([]byte, height)
	}

	for x := 0; x < width; x++ {
		for y := 0; y < height; y++ {
			world[x][y] = 0
		}
	}

	return world
}

func (g *GolEngine) ProcessTurns(args stubs.GolArgs, res *stubs.GolAliveCells) (err error) {
	if !started { // If ProcessTurns is called again, it's a new client connection, continue working on current job
		turns = args.Turns
		turn = 0
		world = args.World
		width = args.Width
		height = args.Height
		started = true

		n := 0
		for n < 10 {
			n++
			fmt.Println("========== STARTING PROCESSING " + strconv.Itoa(turn) + "/" + strconv.Itoa(args.Turns) + "TURNS ==========")
		}

	} else {
		fmt.Println("Client called ProcessTurns while still working, continuing work")
	}

	engineCount := len(engines)
	engineHeight := height / engineCount

	out := make([]chan []util.Cell, engineCount)
	for i := range out {
		out[i] = make(chan []util.Cell)
	}

	for turn < turns {
		m.Lock()

		for id := range engines {
			go startEngine(engines[id], id, engineHeight, out[id])
		}

		world = emptyWorld()
		aliveCells := []util.Cell{}

		for id := range engines {

			var engineCells = <-out[id]
			aliveCells = append(aliveCells, engineCells...)

			for _, cell := range engineCells {
				world[cell.X][cell.Y] = 255
			}

		}

		turn++
		m.Unlock()
	}

	res.TurnsComplete = turns
	res.AliveCells = aliveCells
	started = false

	return
}

func connectEngines() {
	var ips = []string{"127.0.0.1:8031", "127.0.0.1:8032"}
	for id, ip := range ips {
		fmt.Println("Connecting to Engine with IP: " + ip)
		engine, _ := rpc.Dial("tcp", ip)
		engines[id] = engine
	}
}

func main() {
	pAddr := flag.String("port", "8030", "Port to listen on")
	flag.Parse()
	fmt.Println("Game Of Life Broker V1 listening on port: " + *pAddr)

	connectEngines()

	rpc.Register(&GolEngine{})

	listener, _ := net.Listen("tcp", ":"+*pAddr)

	defer listener.Close()
	rpc.Accept(listener)
}
