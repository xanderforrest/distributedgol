package main

import (
	"flag"
	"fmt"
	"log"
	"net"
	"net/rpc"
	"os"
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
var working = false
var aliveCells []util.Cell
var engines = make(map[int]*rpc.Client)

type GolEngine struct{}

func startEngine(client *rpc.Client, world [][]byte, id, engineHeight int, out chan<- []util.Cell) {
	args := stubs.EngineArgs{TotalWorld: world, TWidth: width, THeight: height, Height: engineHeight, Offset: engineHeight * id}
	response := new(stubs.EngineResponse)

	err := client.Call(stubs.ProcessTurn, args, response)
	if err != nil {
		log.Fatal("Error when starting engine with ID: "+strconv.Itoa(id), err)
	}
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

func calculateAliveCells(width, height int, world [][]byte) []util.Cell {
	newCell := []util.Cell{}
	for x := 0; x < width; x++ {
		for y := 0; y < height; y++ {
			if world[x][y] == 0xff {
				newCell = append(newCell, util.Cell{y, x})
			}
		}
	}
	return newCell
}

func (g *GolEngine) ProcessTurns(args stubs.GolArgs, res *stubs.GolAliveCells) (err error) {
	turns = args.Turns
	turn = 0
	world = args.World
	width = args.Width
	height = args.Height
	working = true
	aliveCells = calculateAliveCells(width, height, world) // initialise with current alive for 0 turn tests

	engineCount := len(engines)
	engineHeight := height / engineCount

	out := make([]chan []util.Cell, engineCount)
	for i := range out {
		fmt.Println("Creating Channel for Engine with ID " + strconv.Itoa(i))
		out[i] = make(chan []util.Cell)
	}

	fmt.Println("Channel slice has length: " + strconv.Itoa(len(out)))
	for turn < turns {
		m.Lock()

		for id := range engines {
			go startEngine(engines[id], world, id, engineHeight, out[id])
		}

		nextWorld := emptyWorld()
		aliveCells = nil

		for id := range engines {

			var engineCells = <-out[id]
			aliveCells = append(aliveCells, engineCells...)

			fmt.Println("Processing " + strconv.Itoa(len(engineCells)) + " Alive Cells from Worker ID: " + strconv.Itoa(id))

			for _, cell := range engineCells {
				nextWorld[cell.Y][cell.X] = 255
			}

		}

		fmt.Println("Finished processing turn: " + strconv.Itoa(turn) + "\nWith " + strconv.Itoa(engineCount) + "engines" + "\nWho returned " + strconv.Itoa(len(aliveCells)) + " Alive Cells this turn")
		world = nextWorld

		turn++
		m.Unlock()
	}

	fmt.Println("Returning " + strconv.Itoa(len(aliveCells)) + " to local controller")
	res.TurnsComplete = turns
	res.AliveCells = aliveCells
	working = false

	return
}

func (g *GolEngine) DoTick(_ bool, res *stubs.TickReport) (err error) {
	fmt.Println("Got do tick request...")
	m.Lock()
	res.AliveCount = len(aliveCells)
	res.Turns = turn
	m.Unlock()
	return
}

func (g *GolEngine) PauseEngine(_ bool, res *stubs.EngineStatus) (err error) {
	m.Lock()
	fmt.Println("Pausing Engines on turn: " + strconv.Itoa(turn))
	res.Turn = turn
	res.Working = working
	return
}

func (g *GolEngine) ResumeEngine(_ bool, res *stubs.EngineStatus) (err error) {
	fmt.Println("Resuming Engines from turn: " + strconv.Itoa(turn))
	res.Turn = turn
	res.Working = working
	m.Unlock()
	return
}

func (g *GolEngine) InterruptEngine(_ bool, res *stubs.GolAliveCells) (err error) {
	m.Lock()
	fmt.Println("Interrupt triggered, returning current work to controller.")

	res.TurnsComplete = turn
	res.AliveCells = aliveCells
	m.Unlock()
	return
}

func (g *GolEngine) CheckStatus(_ bool, res *stubs.EngineStatus) (err error) {
	m.Lock()
	res.Turn = turn
	res.Working = working
	m.Unlock()
	return
}

func (g *GolEngine) KillEngine(_ bool, _ *bool) (err error) {
	fmt.Println("Starting shutdown process...")
	for id := range engines {
		fmt.Println("Shutting down Engine with ID: " + strconv.Itoa(id))
		engines[id].Call(stubs.KillEngine, true, true)
	}
	fmt.Println("Shutting down Broker...")
	os.Exit(0)
	return
}

func connectEngines() {
	var ips = []string{"127.0.0.1:8031", "127.0.0.1:8032"}
	for id, ip := range ips {
		fmt.Println("Connecting to Engine with IP: " + ip)
		engine, e := rpc.Dial("tcp", ip)
		if e != nil {
			log.Fatal("connecting to engine error:", e)
		} else {
			engines[id] = engine
			fmt.Println("Connected...")
		}
	}
}

func main() {
	pAddr := flag.String("port", "8030", "Port to listen on")
	flag.Parse()
	fmt.Println("Game Of Life Broker V1 listening on port: " + *pAddr)

	connectEngines()
	fmt.Println("Connected to " + strconv.Itoa(len(engines)) + " GOL Engines.")

	rpc.Register(&GolEngine{})

	listener, _ := net.Listen("tcp", ":"+*pAddr)

	defer listener.Close()
	rpc.Accept(listener)
}
