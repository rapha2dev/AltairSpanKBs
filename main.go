package main

import (
	"altairspankbs/interpreter"
	"os"
)

func main() {
	memo := interpreter.NewMemory()
	program := interpreter.Bake(os.Args[1], memo)
	program()
	//memo.Dump()
}
