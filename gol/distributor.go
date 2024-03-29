package gol

import (
	"fmt"
	"strconv"
	"sync"
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
const unPause int = 3

var wg sync.WaitGroup
var rWg sync.WaitGroup
var rMutex = new(sync.Mutex)
var mutex = new(sync.Mutex)
var cond = sync.NewCond(mutex)
var rCond = sync.NewCond(rMutex)
var read bool = false
var write bool = false

// startY <= target < endY,
// startX <= target < endX (Same for every worker since we slice horizontally)
// Modify params in calculateNextState
func worker(p Params, startY, endY, startX, endX int, world [][]uint8, c distributorChannels, turn int) {
	flipFragment := make([]util.Cell, (endY-startY)*endX/2)
	newPart := make([][]uint8, endY-startY)
	prevWorld := make([][]uint8, p.ImageHeight)
	for h := range world {
		prevWorld[h] = make([]uint8, endX)
	}
	for i := range newPart {
		newPart[i] = make([]uint8, endX)
	}

	rCond.L.Lock()
	for read == false {
		rCond.Wait()
	}
	for j := range world {
		copy(prevWorld[j], world[j])
	}
	rCond.L.Unlock()
	rWg.Done()
	newPart, flipFragment = calculateNextState(p.ImageHeight, p.ImageWidth, startY, endY, prevWorld)

	// Waits other goroutines to copy the previous world

	cond.L.Lock()
	for write == false {
		cond.Wait()
	}
	for j := range newPart {
		copy(world[startY+j], newPart[j])
	}
	for _, cell := range flipFragment {
		c.events <- CellFlipped{
			CompletedTurns: turn,
			Cell:           cell,
		}
	}
	cond.L.Unlock()
	wg.Done()
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
	c.events <- ImageOutputComplete{
		CompletedTurns: t,
		Filename:       outFilename,
	}
}

// Gets input from IO and initialises cellflip
func handleInput(p Params, c distributorChannels, world [][]uint8) [][]uint8 {
	filename := strconv.Itoa(p.ImageHeight) + "x" + strconv.Itoa(p.ImageWidth)
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
	return world
}

func handleKeyPress(p Params, c distributorChannels, keyPresses <-chan rune, world <-chan [][]uint8, t <-chan int, action chan int) {
	paused := false
	for {
		input := <-keyPresses

		switch input {
		case 's':
			action <- Save
			w := <-world
			turn := <-t
			go handleOutput(p, c, w, turn)

		case 'q':
			action <- Quit
			w := <-world
			turn := <-t
			go handleOutput(p, c, w, turn)

			newState := StateChange{CompletedTurns: turn, NewState: State(Quitting)}
			fmt.Println(newState.String())

			c.events <- newState
			c.events <- FinalTurnComplete{CompletedTurns: turn}
		case 'p':
			if paused {
				action <- unPause
				turn := <-t
				paused = false
				newState := StateChange{CompletedTurns: turn, NewState: State(Executing)}
				fmt.Println(newState.String())
				c.events <- newState
			} else {
				action <- Pause
				turn := <-t
				paused = true
				newState := StateChange{CompletedTurns: turn, NewState: State(Paused)}
				fmt.Println(newState.String())
				c.events <- newState
			}

		case 'k':
		}

	}

}

// distributor divides the work between workers and interacts with other goroutines.
func distributor(p Params, c distributorChannels, keyPresses <-chan rune) {
	world := make([][]uint8, p.ImageHeight)
	prevWorld := make([][]uint8, p.ImageHeight)
	sharedWorld := make([][]uint8, p.ImageHeight)
	for i := range world {
		world[i] = make([]uint8, p.ImageWidth)
		prevWorld[i] = make([]uint8, p.ImageWidth)
		sharedWorld[i] = make([]uint8, p.ImageWidth)
	}

	world = handleInput(p, c, world)

	// TODO: Execute all turns of the Game of Life.
	turn := 0
	ticker := time.NewTicker(2 * time.Second)
	done := make(chan bool)
	pause := false
	quit := false
	waitToUnpause := make(chan bool)
	go func() {
		for {
			if !quit {
				select {
				case <-done:
					return
				case <-ticker.C:
					aliveCount, _ := calculateAliveCells(p, prevWorld)
					aliveReport := AliveCellsCount{
						CompletedTurns: turn,
						CellsCount:     aliveCount,
					}
					c.events <- aliveReport
				}
			} else {
				return
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
					pause = true
					turnChan <- turn
				case unPause:
					pause = false
					turnChan <- turn
					waitToUnpause <- true
				case Quit:
					worldChan <- world
					turnChan <- turn
					quit = true
					//return
				case Save:
					worldChan <- world
					turnChan <- turn
				}
			}
			//}
		}
	}()

	// var worldFragment [][]uint8
	channels := make([]chan [][]uint8, p.Threads)
	cellFlip := make([]util.Cell, p.ImageHeight*p.ImageWidth)
	// flipChan := make([]chan []util.Cell, p.Threads)
	unit := int(p.ImageHeight / p.Threads)

	for t := 0; t < p.Turns; t++ {
		cellFlip = make([]util.Cell, p.ImageHeight*p.ImageWidth)
		if pause {
			<-waitToUnpause
		}
		if !pause && !quit {
			turn = t
			for j := range world {
				copy(prevWorld[j], world[j])
			}
			if p.Threads == 1 {
				world, cellFlip = calculateNextState(p.ImageHeight, p.ImageWidth, 0, p.ImageHeight, world)
				for _, cell := range cellFlip {
					c.events <- CellFlipped{
						CompletedTurns: turn,
						Cell:           cell,
					}
				}
			} else {
				rWg.Add(p.Threads)
				wg.Add(p.Threads)
				for i := 0; i < p.Threads; i++ {
					channels[i] = make(chan [][]uint8)
					// flipChan[i] = make(chan []util.Cell)
					if i == p.Threads-1 {
						// Handling with problems if threads division goes with remainders
						go worker(p, i*unit, p.ImageHeight, 0, p.ImageWidth, world, c, turn)
					} else {
						go worker(p, i*unit, (i+1)*unit, 0, p.ImageWidth, world, c, turn)
					}
				}
				rCond.L.Lock()
				read = true
				rCond.Broadcast()
				rCond.L.Unlock()
				rWg.Wait()

				cond.L.Lock()
				read = false
				write = true
				cond.Broadcast()
				cond.L.Unlock()
				wg.Wait()
				write = false
			}

			c.events <- TurnComplete{
				CompletedTurns: turn,
			}
		} else {
			if quit {
				break
			} else {
				continue
			}
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
