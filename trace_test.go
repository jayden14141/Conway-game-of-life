package main

import (
	"os"
	"runtime/trace"
	"testing"

	"uk.ac.bris.cs/gameoflife/gol"
	"uk.ac.bris.cs/gameoflife/util"
)

// TestTrace is a special test to be used to generate traces - not a real test
func TestTrace(t *testing.T) {
	traceParams := gol.Params{
		Turns:       100,
		Threads:     8,
		ImageWidth:  512,
		ImageHeight: 512,
	}
	f, _ := os.Create("trace.out")
	events := make(chan gol.Event)
	err := trace.Start(f)
	util.Check(err)
	go gol.Run(traceParams, events, nil)
	complete := false
	for !complete {
		event := <-events
		switch event.(type) {
		case gol.FinalTurnComplete:
			complete = true
		}
	}
	trace.Stop()
	err = f.Close()
	util.Check(err)

}
