package stubs

import "uk.ac.bris.cs/gameoflife/util"

var ProcessTurns = "GolEngine.ProcessTurns"

type GolArgs struct {
	World                [][]byte
	Width, Height, Turns int
}

type GolAliveCells struct {
	TurnsComplete int
	AliveCells    []util.Cell
}
