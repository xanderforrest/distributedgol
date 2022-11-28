package stubs

import "uk.ac.bris.cs/gameoflife/util"

var ProcessTurns = "GolEngine.ProcessTurns"
var DoTick = "GolEngine.DoTick"

type GolArgs struct {
	World                [][]byte
	Width, Height, Turns int
}

type GolAliveCells struct {
	TurnsComplete int
	AliveCells    []util.Cell
}

type TickReport struct {
	Turns      int
	AliveCount int
}
