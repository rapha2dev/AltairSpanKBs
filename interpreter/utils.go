package interpreter

import (
	"encoding/json"
	"math"
	"os"
	"os/exec"
	"strings"
)

func LoadAst(fileName string) (ast map[string]interface{}) {
	if strings.Contains(fileName, ".json") {
		b, _ := os.ReadFile(fileName)
		ast = ParseAst(b)
	} else {
		jsonFile := strings.TrimSuffix(fileName, "rinha") + "json"
		cmd := exec.Command("rinha.exe", fileName)
		out, _ := cmd.Output()
		ast = ParseAst(out)
		os.WriteFile(jsonFile, out, 0660)
	}
	return
}

func ParseAst(b []byte) map[string]interface{} {
	ast := map[string]interface{}{}
	json.Unmarshal(b, &ast)
	return ast
}

func isAddOverflow(a, b int64) bool {
	signA := int64(1)
	if a < 0 {
		signA = -1
		a = -a
	}

	deltaMax := math.MaxInt64 - a
	return deltaMax-(signA*b) < 0
}
