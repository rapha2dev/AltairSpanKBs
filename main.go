package main

import (
	"altairspankbs/interpreter"
	"os"
)

func main() {
	var file string
	if len(os.Args) == 1 {
		file = "/var/rinha/source.rinha.json"
	} else {
		file = os.Args[1]
	}

	memo := interpreter.NewMemory()
	program := interpreter.Bake(file, memo)
	program()
	//memo.Dump()
}
