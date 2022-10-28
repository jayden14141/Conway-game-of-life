package gol

type distributorChannels struct {
	events     chan<- Event
	ioCommand  chan<- ioCommand
	ioIdle     <-chan bool
	ioFilename chan<- string
	ioOutput   chan<- uint8
	ioInput    <-chan uint8
}

// func calculateNextState(world [][]uint8) (result [][]uint8) {
// 	return
// }


// func calculateAliveCells(world [][]uint8) (alive []util.Cell) {
// 	return
// }

// Constructs 'world' array every time, reading bytes from io.go
func constructWorld(b chan uint8) [][] uint8 {
	byte := <- b
	return
}



// distributor divides the work between workers and interacts with other goroutines.
func distributor(p Params, c distributorChannels) {


	// TODO: Create a 2D slice to store the world.
	world := make([][]uint8, p.ImageHeight)
	for i := range world {
		world[i] = make([]uint8, p.ImageWidth)
	}

	turn := 0
	aliveCells := make([]util.Cell, p.ImageHeight * p.ImageWidth)
	// TODO: Execute all turns of the Game of Life.
	for t:= 0; t < p.Turns; t++ {
		world = calculateNextState(constructWorld(c.ioInput))
		calculateAliveCells()

	}
	// TODO: Report the final state using FinalTurnCompleteEvent.
	report := FinalTurnComplete {
		CompletedTurns: p.Turns,
		Alive : ~
	}
	c.event <- report
	// Make sure that the Io has finished any output before exiting.
	c.ioCommand <- ioCheckIdle
	<-c.ioIdle

	c.events <- StateChange{turn, Quitting}

	// Close the channel to stop the SDL goroutine gracefully. Removing may cause deadlock.
	close(c.events)
}