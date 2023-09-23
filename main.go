package main

import (
	"altairspankbs/interpreter"
	"os"
)

func main() {
	ast := interpreter.LoadAst(os.Args[1])
	memo := interpreter.NewMemory()
	program := interpreter.Bake(ast, memo)
	program()
	//memo.Dump()
}
