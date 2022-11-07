package gol

import (
	"fmt"
	"strconv"
	"time"

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

const Save int = 0
const Quit int = 1
const Pause int = 2

// startY <= target < endY,
// startX <= target < endX (Same for every worker since we slice horizontally)
// Modify params in calculateNextState
func worker(p Params, startY, endY, startX, endX int, world [][]uint8, out chan<- [][]uint8, flip chan<- []util.Cell) {
	flipFragment := make([]util.Cell, (endY-startY)*endX)
	newPart := make([][]uint8, endY-startY)
	for i := range newPart {
		newPart[i] = make([]uint8, endX)
		// copy(newPart[i], world[startY+i])
	}
	newPart, flipFragment = calculateNextState(p.ImageHeight, p.ImageWidth, startY, endY, world)
	out <- newPart
	flip <- flipFragment
}

func handleOutput(p Params, c distributorChannels, world [][]uint8, t int) {
	c.ioCommand <- 0
	outFilename := strconv.Itoa(p.ImageHeight) + "x" + strconv.Itoa(p.ImageWidth) + "x" + strconv.Itoa(t)
	c.ioFilename <- outFilename
	for y := 0; y < p.ImageHeight; y++ {
		for x := 0; x < p.ImageWidth; x++ {
			c.ioOutput <- world[y][x]
		}
	}
}

func handleKeyPress(p Params, c distributorChannels, keyPresses <-chan rune, world <-chan [][]uint8, t <-chan int, action chan int) {
	for {
		input := <-keyPresses
		paused := false
		if paused {
			switch input {
			case 'p':
				newState := StateChange{
					CompletedTurns: <-t,
					NewState:       State(Executing),
				}
				fmt.Println("Continuing")
				c.events <- newState
				paused = false
			}
		} else {
			switch input {
			case 's':
				action <- Save
				w := <-world
				turn := <-t
				go handleOutput(p, c, w, turn)
			case 'q':
				// outputDone := make(chan bool)
				go handleOutput(p, c, <-world, <-t)
				// <-outputDone

				newState := StateChange{CompletedTurns: <-t, NewState: Quitting}
				fmt.Println(newState.String())
				c.events <- newState

				c.events <- FinalTurnComplete{CompletedTurns: <-t}
			case 'p':
				newState := StateChange{
					CompletedTurns: <-t,
					NewState:       State(Paused),
				}
				fmt.Println(newState.String())
				c.events <- newState
				paused = true
				//pause <- true
			case 'k':
			}
		}
	}

	/*for {
		key := <-keyPresses
		switch key {
		case 's':
			handleOutput(p, c, world, t)
			c.events <- StateChange{CompletedTurns: t, NewState: Executing}
		case 'q':
			handleOutput(p, c, world, t)
			c.events <- StateChange{CompletedTurns: t, NewState: Quitting}
			c.events <- FinalTurnComplete{CompletedTurns: t}
		case 'p':
			//c.events <-
		}
	}*/
}

// distributor divides the work between workers and interacts with other goroutines.
func distributor(p Params, c distributorChannels, keyPresses <-chan rune) {
	// TODO: Create a 2D slice to store the world.
	world := make([][]uint8, p.ImageHeight)
	cellFlip := make([]util.Cell, p.ImageHeight*p.ImageWidth)
	for i := range world {
		world[i] = make([]uint8, p.ImageWidth)
	}

	filename := strconv.Itoa(p.ImageHeight) + "x" + strconv.Itoa(p.ImageWidth)

	// Commands IO to read the initial file, giving the filename via the channel.
	c.ioCommand <- 1
	c.ioFilename <- filename
	for y := 0; y < p.ImageHeight; y++ {
		for x := 0; x < p.ImageWidth; x++ {
			num := <-c.ioInput
			world[y][x] = num
			if num == 255 {
				c.events <- CellFlipped{
					CompletedTurns: 0,
					Cell:           util.Cell{X: x, Y: y},
				}
			}
		}
	}

	// TODO: Execute all turns of the Game of Life.
	turn := 0
	ticker := time.NewTicker(2 * time.Second)
	done := make(chan bool)
	go func() {
		for {
			select {
			case <-done:
				return
			case <-ticker.C:
				aliveCount, _ := calculateAliveCells(p, world)
				aliveReport := AliveCellsCount{
					CompletedTurns: turn,
					CellsCount:     aliveCount,
				}
				c.events <- aliveReport
			}
		}
	}()

	turnChan := make(chan int)
	worldChan := make(chan [][]uint8)
	action := make(chan int)
	go handleKeyPress(p, c, keyPresses, worldChan, turnChan, action)
	go func() {
		for {
			select {
			case command := <-action:
				switch command {
				case Pause:
				case Quit:
				case Save:
					worldChan <- world
					turnChan <- turn
				}
			}
		}
	}()
	for t := 0; t < p.Turns; t++ {
		turn = t
		if p.Threads == 1 {
			world, cellFlip = calculateNextState(p.ImageHeight, p.ImageWidth, 0, p.ImageHeight, world)
		} else {
			var worldFragment [][]uint8
			channels := make([]chan [][]uint8, p.Threads)
			flipChan := make([]chan []util.Cell, p.Threads)
			unit := int(p.ImageHeight / p.Threads)
			for i := 0; i < p.Threads; i++ {
				channels[i] = make(chan [][]uint8)
				flipChan[i] = make(chan []util.Cell)
				if i == p.Threads-1 {
					// Handling with problems if threads division goes with remainders
					go worker(p, i*unit, p.ImageHeight, 0, p.ImageWidth, world, channels[i], flipChan[i])
				} else {
					go worker(p, i*unit, (i+1)*unit, 0, p.ImageWidth, world, channels[i], flipChan[i])
				}
			}
			for i := 0; i < p.Threads; i++ {
				worldPart := <-channels[i]
				worldFragment = append(worldFragment, worldPart...)
				cellPart := <-flipChan[i]
				cellFlip = append(cellFlip, cellPart...)
			}
			for j := range worldFragment {
				copy(world[j], worldFragment[j])
			}
		}

		for _, cell := range cellFlip {
			c.events <- CellFlipped{
				CompletedTurns: turn,
				Cell:           cell,
			}
		}

		c.events <- TurnComplete{
			CompletedTurns: turn,
		}
	}
	ticker.Stop()
	done <- true

	handleOutput(p, c, world, p.Turns)

	// Send the output and invoke writePgmImage() in io.go
	// Sends the world slice to io.go
	// TODO: Report the final state using FinalTurnCompleteEvent.

	aliveCells := make([]util.Cell, p.ImageHeight*p.ImageWidth)
	_, aliveCells = calculateAliveCells(p, world)
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
