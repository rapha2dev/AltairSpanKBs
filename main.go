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

	program := interpreter.Build(file)
	if os.Args[len(os.Args)-1] == "time" {
		t := time.Now()
		program.Execute()
		fmt.Printf("\ntime: %f secs\n\n", time.Now().Sub(t).Seconds())
	} else {
		program.Execute()
	}
}
