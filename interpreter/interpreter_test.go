package interpreter

import (
	"fmt"
	"os"
	"os/exec"
	"strings"
	"testing"
	"time"
)

func TestExamples(t *testing.T) {
	files, _ := os.ReadDir("../examples")
	for _, file := range files {
		var ast map[string]interface{}
		if strings.Contains(file.Name(), ".json") {
			//b, _ := os.ReadFile("../examples/" + file.Name())
			//ast = ParseAst(b)
			continue
		} else {
			jsonFile := strings.TrimSuffix("../examples/"+file.Name(), "rinha") + "json"
			// if _, err := os.Stat(jsonFile); err == nil {
			// 	continue
			// }
			cmd := exec.Command("rinha.exe", "../examples/"+file.Name())
			out, _ := cmd.Output()
			ast = ParseAst(out)
			os.WriteFile(jsonFile, out, 0660)
		}
		memo := NewMemory()
		prog := Bake(ast, memo)
		fmt.Println(">>>>>>>>>>>> ", file.Name())
		t := time.Now()
		prog()
		fmt.Println("<<<<<<<<<<<< time:", time.Now().Sub(t).Seconds())
		fmt.Println()
	}
}
