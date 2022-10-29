package gol

import (
	"strconv"

	"uk.ac.bris.cs/gameoflife/util"
)

type distributorChannels struct {
	events     chan<- Event
	ioCommand  chan<- ioCommand
	ioIdle     <-chan bool
	ioFilename chan<- string
	ioOutput   chan<- uint8
	ioInput    <-chan uint8
}

// startY <= target < endY,
// startX <= target < endX (Same for every worker since we slice horizontally)
// Modify params in calculateNextState
func worker(p Params, startY, endY, startX, endX int, world [][]uint8, out chan<- [][]uint8) {
	newPart := make([][]uint8, endY-startY)
	for i := range newPart {
		newPart[i] = make([]uint8, endX-startX)
		copy(newPart[i], world[startY+i])
	}
	newPart = calculateNextState(p, world)
	out <- newPart
}

// distributor divides the work between workers and interacts with other goroutines.
func distributor(p Params, c distributorChannels) {
	// TODO: Create a 2D slice to store the world.
	world := make([][]uint8, p.ImageHeight)
	for i := range world {
		world[i] = make([]uint8, p.ImageWidth)
	}

	turn := 0
	filename := strconv.Itoa(p.ImageHeight) + "x" + strconv.Itoa(p.ImageWidth)

	// Commands IO to read the initial file, giving the filename via the channel.
	c.ioCommand <- 1
	c.ioFilename <- filename
	for y := 0; y < p.ImageHeight; y++ {
		for x := 0; x < p.ImageWidth; x++ {
			num := <-c.ioInput
			world[y][x] = num
		}
	}

	// TODO: Execute all turns of the Game of Life.

	for t := 0; t < p.Turns; t++ {
		if p.Threads == 1 {
			world = calculateNextState(p, world)
		} else {
			var worldFragment [][]uint8
			channels := make([]chan [][]uint8, p.Threads)
			unit := p.ImageHeight / p.Threads
			for i := 0; i < p.Threads; i++ {
				channels[i] = make(chan [][]uint8)
				go worker(p, i*unit, (i+1)*unit, 0, p.ImageWidth, world, channels[i])
				worldFragment = append(worldFragment, <-channels[i]...)
			}
		}
		// Send the output and invoke writePgmImage() in io.go
		// Sends the world slice to io.go
		c.ioCommand <- 0
		c.ioFilename <- filename
		for y := 0; y < p.ImageHeight; y++ {
			for x := 0; x < p.ImageWidth; x++ {
				c.ioOutput <- world[y][x]
			}
		}
	}

	// TODO: Report the final state using FinalTurnCompleteEvent.

	aliveCells := make([]util.Cell, p.ImageHeight*p.ImageWidth)
	aliveCells = calculateAliveCells(p, world)
	report := FinalTurnComplete{
		CompletedTurns: p.Turns,
		Alive:          aliveCells,
	}
	c.events <- report
	// Make sure that the Io has finished any output before exiting.
	c.ioCommand <- ioCheckIdle
	<-c.ioIdle

	c.events <- StateChange{turn, Quitting}

	// Close the channel to stop the SDL goroutine gracefully. Removing may cause deadlock.
	close(c.events)
}
