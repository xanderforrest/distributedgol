package stubs

import "uk.ac.bris.cs/gameoflife/util"

var ProcessTurns = "GolEngine.ProcessTurns"
var DoTick = "GolEngine.DoTick"
var PauseEngine = "GolEngine.PauseEngine"
var ResumeEngine = "GolEngine.ResumeEngine"
var InterruptEngine = "GolEngine.InterruptEngine"

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

type CurrentTurn struct {
	Turn int
}
