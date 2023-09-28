package interpreter

import (
	"encoding/json"
	"math"
	"os"
	"os/exec"
	"strings"
)

type Closure [6]interface{}
type Tuple [2]interface{}

func LoadAst(fileName string) (code string, ast map[string]interface{}) {
	if strings.Contains(fileName, ".json") {
		b, _ := os.ReadFile(fileName)
		ast = ParseAst(b)
		b, _ = os.ReadFile(strings.TrimSuffix(fileName, "json") + "rinha")
		code = string(b)
	} else if strings.Contains(fileName, ".rinha") {
		jsonFile := strings.TrimSuffix(fileName, "rinha") + "json"
		cmd := exec.Command("rinha.exe", fileName)
		out, _ := cmd.Output()
		ast = ParseAst(out)
		os.WriteFile(jsonFile, out, 0660)
		b, _ := os.ReadFile(fileName)
		code = string(b)
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
