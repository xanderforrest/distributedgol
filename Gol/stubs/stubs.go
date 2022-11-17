package stubs

//syntax meaning ---- handler = "exported_type.exported_method"
var ProcessTurnsHandler = "GameOfLifeOperations.ProcessTurns"

type Request struct {
	InitialWorld                   [][]byte
	Turns, ImageHeight, ImageWidth int
}

type Response struct {
	FinalWorld [][]byte
}
