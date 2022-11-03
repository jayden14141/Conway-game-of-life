package gol

import (
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

// startY <= target < endY,
// startX <= target < endX (Same for every worker since we slice horizontally)
// Modify params in calculateNextState
func worker(p Params, startY, endY, startX, endX int, world [][]uint8, out chan<- [][]uint8) {
	newPart := make([][]uint8, endY-startY)
	for i := range newPart {
		newPart[i] = make([]uint8, endX)
		// copy(newPart[i], world[startY+i])
	}
	newPart = calculateNextState(p.ImageHeight, p.ImageWidth, startY, endY, world)
	out <- newPart
}

// distributor divides the work between workers and interacts with other goroutines.
func distributor(p Params, c distributorChannels) {
	// -----------------------Tracing----------------------------------
	// "go tool trace out/trace.out"
	// f, err := os.Create("out/trace.out")
	// if err != nil {
	// 	log.Fatalf("failed to create trace output file: %v", err)
	// }
	// defer func() {
	// 	if err := f.Close(); err != nil {
	// 		log.Fatalf("failed to close trace file: %v", err)
	// 	}
	// }()

	// if err := trace.Start(f); err != nil {
	// 	log.Fatalf("failed to start trace: %v", err)
	// }
	// defer trace.Stop()
	// -----------------------------------------------------------------

	// TODO: Create a 2D slice to store the world.
	world := make([][]uint8, p.ImageHeight)
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
		}
	}

	c.ioCommand <- 0
	outFilename := strconv.Itoa(p.ImageHeight) + "x" + strconv.Itoa(p.ImageWidth) + "x" + strconv.Itoa(0)
	c.ioFilename <- outFilename
	for y := 0; y < p.ImageHeight; y++ {
		for x := 0; x < p.ImageWidth; x++ {
			c.ioOutput <- world[y][x]
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

	for t := 0; t < p.Turns; t++ {
		turn = t
		if p.Threads == 1 {
			world = calculateNextState(p.ImageHeight, p.ImageWidth, 0, p.ImageHeight, world)
		} else {
			var worldFragment [][]uint8
			channels := make([]chan [][]uint8, p.Threads)
			unit := int(p.ImageHeight / p.Threads)
			for i := 0; i < p.Threads; i++ {
				channels[i] = make(chan [][]uint8)
				if i == p.Threads-1 {
					// Handling with problems if threads division goes with remainders
					go worker(p, i*unit, p.ImageHeight, 0, p.ImageWidth, world, channels[i])
				} else {
					go worker(p, i*unit, (i+1)*unit, 0, p.ImageWidth, world, channels[i])
				}
				worldFragment = append(worldFragment, <-channels[i]...)
			}
			for j := range worldFragment {
				copy(world[j], worldFragment[j])
			}
		}
		if t == 0 || t == p.Turns-1 {
			c.ioCommand <- 0
			outFilename := strconv.Itoa(p.ImageHeight) + "x" + strconv.Itoa(p.ImageWidth) + "x" + strconv.Itoa(t+1)
			c.ioFilename <- outFilename
			for y := 0; y < p.ImageHeight; y++ {
				for x := 0; x < p.ImageWidth; x++ {
					c.ioOutput <- world[y][x]
				}
			}
		}

		c.events <- TurnComplete{
			CompletedTurns: t,
		}
	}
	ticker.Stop()
	done <- true

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
