package stubs

import "uk.ac.bris.cs/gameoflife/util"

var ProcessTurns = "GolEngine.ProcessTurns"
var DoTick = "GolEngine.DoTick"
var PauseEngine = "GolEngine.PauseEngine"
var ResumeEngine = "GolEngine.ResumeEngine"
var InterruptEngine = "GolEngine.InterruptEngine"
var CheckStatus = "GolEngine.CheckStatus"
var KillEngine = "GolEngine.KillEngine"

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

type EngineStatus struct {
	Working bool
	Turn    int
}
