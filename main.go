package main

import (
	"altairspankbs/interpreter"
	"fmt"
	"os"
	"time"
)

func main() {
	var file string
	if len(os.Args) == 1 {
		file = "/var/rinha/source.rinha.json"
	} else {
		file = os.Args[1]
	}

	t := time.Now()
	memo := interpreter.NewMemory()
	program := interpreter.Bake(file, memo)
	program()
	if os.Args[len(os.Args)-1] == "time" {
		fmt.Printf("\ntime: %f secs\n\n", time.Now().Sub(t).Seconds())
	}
	//memo.Dump()
}
