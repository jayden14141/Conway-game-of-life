package main

import (
	"fmt"
	"os"
	"runtime/trace"
	"testing"

	"uk.ac.bris.cs/gameoflife/gol"
	"uk.ac.bris.cs/gameoflife/util"
)

// TestTrace is a special test to be used to generate traces - not a real test
// func TestTracing(t *testing.T) {
// 	for threads := 1; threads <= 16; threads++ {
// 		traceParams := gol.Params{
// 			Turns:       100,
// 			Threads:     threads,
// 			ImageWidth:  512,
// 			ImageHeight: 512,
// 		}
// 		filename := "trace.out-" + strconv.Itoa(threads)
// 		f, _ := os.Create("/" + filename)
// 		events := make(chan gol.Event)
// 		err := trace.Start(f)
// 		util.Check(err)
// 		go gol.Run(traceParams, events, nil)
// 		for range events {
// 		}
// 		trace.Stop()
// 		err = f.Close()
// 		util.Check(err)
// 	}
// }

func BenchmarkFilter(b *testing.B) {
	// Disable all program output apart from benchmark results
	os.Stdout = nil

	for threads := 1; threads <= 16; threads++ {
		b.Run(fmt.Sprintf("%d_workers", threads), func(b *testing.B) {
			traceParams := gol.Params{
				Turns:       10,
				Threads:     threads,
				ImageWidth:  512,
				ImageHeight: 512,
			}
			f, _ := os.Create("results.out")
			events := make(chan gol.Event)
			err := trace.Start(f)
			util.Check(err)
			go gol.Run(traceParams, events, nil)
			for range events {
			}
			trace.Stop()
			err = f.Close()
			util.Check(err)
		})
	}
}
