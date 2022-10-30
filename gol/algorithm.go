package gol

import (
	"uk.ac.bris.cs/gameoflife/util"
)

func mod(a, b int) int {
	return (a%b + b) % b
}

func calculateNeighbours(height, width int, world [][]byte, y int, x int) int {

	h := len(world)
	w := len(world[0])
	noOfNeighbours := 0

	neighbour := []byte{world[mod(y+1, h)][mod(x, w)], world[mod(y+1, h)][mod(x+1, w)], world[mod(y, h)][mod(x+1, w)],
		world[mod(y-1, h)][mod(x+1, w)], world[mod(y-1, h)][mod(x, w)], world[mod(y-1, h)][mod(x-1, w)],
		world[mod(y, h)][mod(x-1, w)], world[mod(y+1, h)][mod(x-1, w)]}

	for i := 0; i < 8; i++ {
		if neighbour[i] == 255 {
			noOfNeighbours++
		}
	}

	return noOfNeighbours
}

func calculateNextState(height, width, startY, endY int, world [][]byte) [][]byte {

	newWorld := make([][]byte, len(world))
	for i := range world {
		newWorld[i] = make([]byte, len(world[i]))
		copy(newWorld[i], world[i])
	}

	for y := startY; y < endY; y++ {
		for x := 0; x < width; x++ {
			noOfNeighbours := calculateNeighbours(height, width, world, y, x)
			if world[y][x] == 255 {
				if noOfNeighbours < 2 {
					newWorld[y][x] = 0
				} else if noOfNeighbours == 2 || noOfNeighbours == 3 {
					newWorld[y][x] = 255
				} else if noOfNeighbours > 3 {
					newWorld[y][x] = 0
				}
			} else if world[y][x] == 0 && noOfNeighbours == 3 {
				newWorld[y][x] = 255
			}
		}
	}

	return newWorld
}

func calculateAliveCells(p Params, world [][]byte) []util.Cell {

	var aliveCells []util.Cell

	for y := 0; y < p.ImageHeight; y++ {
		for x := 0; x < p.ImageWidth; x++ {
			if world[y][x] == 255 {
				aliveCells = append(aliveCells, util.Cell{X: x, Y: y})
			}
		}
	}
	return aliveCells
}
