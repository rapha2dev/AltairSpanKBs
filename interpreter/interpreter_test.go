package interpreter

import (
	"fmt"
	"os"
	"strings"
	"testing"
	"time"
)

func TestExamples(t *testing.T) {
	files, _ := os.ReadDir("../examples")
	for _, file := range files {
		if !strings.Contains(file.Name(), ".rinha") {
			continue
		}
		prog := Build("../examples/" + file.Name())
		fmt.Println(">>>>>>>>>>>> ", file.Name())
		t := time.Now()
		prog.Execute()
		fmt.Println("<<<<<<<<<<<< time:", time.Now().Sub(t).Seconds())
		fmt.Println()
	}
}
