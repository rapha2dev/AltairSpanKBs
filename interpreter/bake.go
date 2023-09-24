package interpreter

import (
	"fmt"
	"math/big"
	"os"
	"reflect"
	"strconv"
	"strings"
)

type Backed func() interface{}
type Closure [4]interface{}
type Tuple [2]interface{}

func Bake(file string, memo *Memory) Backed {
	code, ast := LoadAst(file)
	if memo == nil {
		memo = NewMemory()
	}

	errorHandlers := []func(r interface{}){}
	errorTypeDict := map[string]string{
		"interpreter.Closure": "#closure",
		"interpreter.Tuple":   "tuple",
		"int64":               "int",
		"string":              "string",
		"bool":                "boolean",
	}
	currentErrorHandlerIndex := 0

	var bake func(term map[string]interface{}, memo *Memory) Backed
	bake = func(term map[string]interface{}, memo *Memory) Backed {
		if term["expression"] != nil {
			exp := term["expression"].(map[string]interface{})
			return func() interface{} {
				defer func() {
					if r := recover(); r != nil {
						errorHandlers[currentErrorHandlerIndex](r)
					}
				}()
				res := bake(exp, memo)()
				return res
			}
		}

		// ----------------
		errorHandlerIndex := len(errorHandlers)
		errorHandlers = append(errorHandlers, func(r interface{}) {
			loc := term["location"].(map[string]interface{})
			start := int(loc["start"].(float64))
			end := int(loc["end"].(float64))
			lines := strings.Split(code[:end], "\n")
			errorLine := len(lines)
			lineCol := -1
			for i := 0; i < errorLine-1; i++ {
				lineCol += len(lines[i]) + 1
			}
			fmt.Printf("\nerror in file: '%s', line: %d, start: %d, end: %d\n%s\n\n... %s ...\n\n\n", loc["filename"], errorLine, start-lineCol, end-lineCol, fmt.Sprint(r), code[start:end])
			os.Exit(0)
		})
		emitError := func(v interface{}) {
			currentErrorHandlerIndex = errorHandlerIndex
			panic(v)
		}

		// -----------------

		switch term["kind"] {

		case "Int":
			val := int64(term["value"].(float64))
			return func() interface{} { return val }

		case "Str", "Bool":
			val := term["value"]
			return func() interface{} { return val }

		case "First":
			tuple := bake(term["value"].(map[string]interface{}), memo)
			return func() interface{} {
				v := tuple()
				if t, ok := v.(Tuple); ok {
					return t[0]
				} else {
					emitError(fmt.Sprintf("Invalid tuple operation: first(<%s>)", errorTypeDict[fmt.Sprint(reflect.TypeOf(v))]))
					return nil
				}
			}

		case "Second":
			tuple := bake(term["value"].(map[string]interface{}), memo)
			return func() interface{} {
				v := tuple()
				if t, ok := v.(Tuple); ok {
					return t[1]
				} else {
					emitError(fmt.Sprintf("Invalid tuple operation: second(<%s>)", errorTypeDict[fmt.Sprint(reflect.TypeOf(v))]))
					return nil
				}
			}

		case "Tuple":
			first := bake(term["first"].(map[string]interface{}), memo)
			second := bake(term["second"].(map[string]interface{}), memo)
			return func() interface{} { return Tuple{first(), second()} }

		case "Let":
			name := memo.MakeStack(term["name"].(map[string]interface{})["text"].(string))
			val := bake(term["value"].(map[string]interface{}), memo)
			next := bake(term["next"].(map[string]interface{}), memo)
			return func() interface{} {
				g := val()
				switch h := g.(type) {
				case Closure:
					if refs, ok := h[0].(*int); ok { // function ref
						*refs++
						defer h[1].(func())()
					}
				}
				name.Push(g)
				v := next()
				name.Pop()
				return v
			}

		case "Var":
			name := memo.MakeStack(term["text"].(string))
			return func() interface{} {
				v := name.Value()
				if v == nil {
					emitError("var not found")
				}
				return v
			}

		case "Call":
			callee := bake(term["callee"].(map[string]interface{}), memo)
			args := make([]Backed, len(term["arguments"].([]interface{})))
			for i, t := range term["arguments"].([]interface{}) {
				args[i] = bake(t.(map[string]interface{}), memo)
			}
			argsLen := len(args)
			var params []*Stack
			var body Backed
			return func() interface{} {
				//fmt.Println("call:", term["callee"].(map[string]interface{})["text"])
				if body == nil {
					a := callee().(Closure)
					body = a[2].(Backed)
					params = a[3].([]*Stack)
					if len(params) != argsLen {
						emitError("Wrong number of arguments")
					}
				}

				for i, arg := range args {
					params[i].PushParam(arg())
				}
				for _, param := range params {
					param.PopParam()
				}
				v := body()
				for _, param := range params {
					param.Pop()
				}
				return v
			}

		case "Function":
			memo = memo.Fork()
			body := bake(term["value"].(map[string]interface{}), memo)
			params := make([]*Stack, len(term["parameters"].([]interface{})))
			for i, p := range term["parameters"].([]interface{}) {
				params[i] = memo.MakeStack(p.(map[string]interface{})["text"].(string))
			}
			references := 0
			onUnref := func() {
				references--
				if references == 0 {
					memo.OnUnreferenced()
				}
			}
			return func() interface{} {
				return Closure{&references, onUnref, body, params}
			}

		case "If":
			condition := bake(term["condition"].(map[string]interface{}), memo)
			then := bake(term["then"].(map[string]interface{}), memo)
			otherwise := bake(term["otherwise"].(map[string]interface{}), memo)
			return func() interface{} {
				if condition().(bool) {
					return then()
				}
				return otherwise()
			}

		case "Binary":
			lhs := bake(term["lhs"].(map[string]interface{}), memo)
			rhs := bake(term["rhs"].(map[string]interface{}), memo)

			// otimização quando o rhs é um literal Int ou Bool
			if rightKind := term["rhs"].(map[string]interface{})["kind"]; rightKind == "Int" {
				switch term["op"] {
				case "Sub":
					r := rhs().(int64)
					return func() interface{} {
						switch l := lhs().(type) {
						case int64:
							return l - r
						case *big.Int:
							return big.NewInt(0).Sub(l, big.NewInt(r))
						default:
							emitError(fmt.Sprintf("Invalid binary operation: <%s> - <%s>", errorTypeDict[fmt.Sprint(reflect.TypeOf(l))], errorTypeDict[fmt.Sprint(reflect.TypeOf(r))]))
						}
						return nil
					}
				case "Mul":
					r := rhs().(int64)
					return func() interface{} {
						switch l := lhs().(type) {
						case int64:
							return l * r
						case *big.Int:
							return big.NewInt(0).Mul(l, big.NewInt(r))
						default:
							emitError(fmt.Sprintf("Invalid binary operation: <%s> * <%s>", errorTypeDict[fmt.Sprint(reflect.TypeOf(l))], errorTypeDict[fmt.Sprint(reflect.TypeOf(r))]))
						}
						return nil
					}
				case "Div":
					r := rhs().(int64)
					return func() interface{} {
						switch l := lhs().(type) {
						case int64:
							return l / r
						case *big.Int:
							return big.NewInt(0).Div(l, big.NewInt(r))
						default:
							emitError(fmt.Sprintf("Invalid binary operation: <%s> / <%s>", errorTypeDict[fmt.Sprint(reflect.TypeOf(l))], errorTypeDict[fmt.Sprint(reflect.TypeOf(r))]))
						}
						return nil
					}
				case "Rem":
					r := rhs().(int64)
					return func() interface{} {
						switch l := lhs().(type) {
						case int64:
							return l % r
						case *big.Int:
							return big.NewInt(0).Rem(l, big.NewInt(r))
						default:
							emitError(fmt.Sprintf("Invalid binary operation: <%s> %s <%s>", errorTypeDict[fmt.Sprint(reflect.TypeOf(l))], "%", errorTypeDict[fmt.Sprint(reflect.TypeOf(r))]))
						}
						return nil
					}
				case "Lt":
					r := rhs().(int64)
					return func() interface{} {
						switch l := lhs().(type) {
						case int64:
							return l < r
						case *big.Int:
							return l.Cmp(big.NewInt(r)) == -1
						default:
							emitError(fmt.Sprintf("Invalid binary operation: <%s> < <%s>", errorTypeDict[fmt.Sprint(reflect.TypeOf(l))], errorTypeDict[fmt.Sprint(reflect.TypeOf(r))]))
						}
						return nil
					}
				case "Lte":
					r := rhs().(int64)
					return func() interface{} {
						switch l := lhs().(type) {
						case int64:
							return l <= r
						case *big.Int:
							return l.Cmp(big.NewInt(r)) <= 0
						default:
							emitError(fmt.Sprintf("Invalid binary operation: <%s> < <%s>", errorTypeDict[fmt.Sprint(reflect.TypeOf(l))], errorTypeDict[fmt.Sprint(reflect.TypeOf(r))]))
						}
						return nil
					}
				case "Gt":
					r := rhs().(int64)
					return func() interface{} {
						switch l := lhs().(type) {
						case int64:
							return l > r
						case *big.Int:
							return l.Cmp(big.NewInt(r)) == 1
						default:
							emitError(fmt.Sprintf("Invalid binary operation: <%s> > <%s>", errorTypeDict[fmt.Sprint(reflect.TypeOf(l))], errorTypeDict[fmt.Sprint(reflect.TypeOf(r))]))
						}
						return nil
					}
				case "Gte":
					r := rhs().(int64)
					return func() interface{} {
						switch l := lhs().(type) {
						case int64:
							return l >= r
						case *big.Int:
							return l.Cmp(big.NewInt(r)) >= 0
						default:
							emitError(fmt.Sprintf("Invalid binary operation: <%s> > <%s>", errorTypeDict[fmt.Sprint(reflect.TypeOf(l))], errorTypeDict[fmt.Sprint(reflect.TypeOf(r))]))
						}
						return nil
					}
				case "Eq":
					r := rhs().(int64)
					return func() interface{} {
						switch l := lhs().(type) {
						case int64:
							return l == r
						case *big.Int:
							return l.Cmp(big.NewInt(r)) == 0
						default:
							emitError(fmt.Sprintf("Invalid binary operation: <%s> == <%s>", errorTypeDict[fmt.Sprint(reflect.TypeOf(l))], errorTypeDict[fmt.Sprint(reflect.TypeOf(r))]))
						}
						return nil
					}
				}
			} else if rightKind == "Bool" {
				switch term["op"] {
				case "Eq":
					r := rhs().(bool)
					return func() interface{} {
						switch l := lhs().(type) {
						case bool:
							return l == r
						default:
							emitError(fmt.Sprintf("Invalid binary operation: <%s> == <%s>", errorTypeDict[fmt.Sprint(reflect.TypeOf(l))], errorTypeDict[fmt.Sprint(reflect.TypeOf(r))]))
						}
						return nil
					}
				}
			}

			switch term["op"] {
			case "Add":
				return func() interface{} {
					switch l := lhs().(type) {
					case int64:
						switch r := rhs().(type) {
						case int64:
							if isAddOverflow(l, r) {
								return big.NewInt(0).Add(big.NewInt(l), big.NewInt(r))
							}
							return l + r
						case *big.Int:
							return big.NewInt(0).Add(big.NewInt(l), r)
						case string:
							return strconv.FormatInt(l, 10) + r
						default:
							emitError(fmt.Sprintf("Invalid binary operation: <%s> + <%s>", errorTypeDict[fmt.Sprint(reflect.TypeOf(l))], errorTypeDict[fmt.Sprint(reflect.TypeOf(r))]))
						}
					case *big.Int:
						switch r := rhs().(type) {
						case int64:
							return big.NewInt(0).Add(l, big.NewInt(r))
						case *big.Int:
							return big.NewInt(0).Add(l, r)
						case string:
							return l.String() + r
						default:
							emitError(fmt.Sprintf("Invalid binary operation: <%s> + <%s>", errorTypeDict[fmt.Sprint(reflect.TypeOf(l))], errorTypeDict[fmt.Sprint(reflect.TypeOf(r))]))
						}
					case string:
						switch r := rhs().(type) {
						case int64:
							return l + strconv.FormatInt(r, 10)
						case *big.Int:
							return l + r.String()
						case string:
							return l + r
						default:
							emitError(fmt.Sprintf("Invalid binary operation: <%s> + <%s>", errorTypeDict[fmt.Sprint(reflect.TypeOf(l))], errorTypeDict[fmt.Sprint(reflect.TypeOf(r))]))
						}
					default:
						emitError(fmt.Sprintf("Invalid binary operation: <%s> + ...", errorTypeDict[fmt.Sprint(reflect.TypeOf(l))]))
					}
					return nil
				}
			case "Sub":
				return func() interface{} {
					switch l := lhs().(type) {
					case int64:
						switch r := rhs().(type) {
						case int64:
							if isAddOverflow(l, -r) {
								return big.NewInt(0).Sub(big.NewInt(l), big.NewInt(r))
							}
							return l - r
						case *big.Int:
							return big.NewInt(0).Sub(big.NewInt(l), r)
						default:
							emitError(fmt.Sprintf("Invalid binary operation: <%s> - <%s>", errorTypeDict[fmt.Sprint(reflect.TypeOf(l))], errorTypeDict[fmt.Sprint(reflect.TypeOf(r))]))
						}
					case *big.Int:
						switch r := rhs().(type) {
						case int64:
							return big.NewInt(0).Sub(l, big.NewInt(r))
						case *big.Int:
							return big.NewInt(0).Sub(l, r)
						default:
							emitError(fmt.Sprintf("Invalid binary operation: <%s> - <%s>", errorTypeDict[fmt.Sprint(reflect.TypeOf(l))], errorTypeDict[fmt.Sprint(reflect.TypeOf(r))]))
						}
					default:
						emitError(fmt.Sprintf("Invalid binary operation: <%s> - ...", errorTypeDict[fmt.Sprint(reflect.TypeOf(l))]))
					}
					return nil
				}

			case "Mul", "Div", "Rem":
				emitError("Invalid binary operation")
				// TODO: implementar essas operações para variaveis e bigint
				return nil

			case "Lt":
				return func() interface{} {
					switch l := lhs().(type) {
					case int64:
						switch r := rhs().(type) {
						case int64:
							return l < r
						case *big.Int:
							return r.Cmp(big.NewInt(l)) == 1
						default:
							emitError(fmt.Sprintf("Invalid binary operation: <%s> < <%s>", errorTypeDict[fmt.Sprint(reflect.TypeOf(l))], errorTypeDict[fmt.Sprint(reflect.TypeOf(r))]))
						}
					case *big.Int:
						switch r := rhs().(type) {
						case int64:
							return l.Cmp(big.NewInt(r)) == -1
						case *big.Int:
							return l.Cmp(r) == -1
						default:
							emitError(fmt.Sprintf("Invalid binary operation: <%s> < <%s>", errorTypeDict[fmt.Sprint(reflect.TypeOf(l))], errorTypeDict[fmt.Sprint(reflect.TypeOf(r))]))
						}
					default:
						emitError(fmt.Sprintf("Invalid binary operation: <%s> < ...", errorTypeDict[fmt.Sprint(reflect.TypeOf(l))]))
					}
					return nil
				}
			case "Lte":
				return func() interface{} {
					switch l := lhs().(type) {
					case int64:
						switch r := rhs().(type) {
						case int64:
							return l <= r
						case *big.Int:
							return r.Cmp(big.NewInt(l)) >= 0
						default:
							emitError(fmt.Sprintf("Invalid binary operation: <%s> < <%s>", errorTypeDict[fmt.Sprint(reflect.TypeOf(l))], errorTypeDict[fmt.Sprint(reflect.TypeOf(r))]))
						}
					case *big.Int:
						switch r := rhs().(type) {
						case int64:
							return l.Cmp(big.NewInt(r)) <= 0
						case *big.Int:
							return l.Cmp(r) <= 0
						default:
							emitError(fmt.Sprintf("Invalid binary operation: <%s> < <%s>", errorTypeDict[fmt.Sprint(reflect.TypeOf(l))], errorTypeDict[fmt.Sprint(reflect.TypeOf(r))]))
						}
					default:
						emitError(fmt.Sprintf("Invalid binary operation: <%s> < ...", errorTypeDict[fmt.Sprint(reflect.TypeOf(l))]))
					}
					return nil
				}
			case "Gt":
				return func() interface{} {
					switch l := lhs().(type) {
					case int64:
						switch r := rhs().(type) {
						case int64:
							return l > r
						case *big.Int:
							return r.Cmp(big.NewInt(l)) == -1
						default:
							emitError(fmt.Sprintf("Invalid binary operation: <%s> > <%s>", errorTypeDict[fmt.Sprint(reflect.TypeOf(l))], errorTypeDict[fmt.Sprint(reflect.TypeOf(r))]))
						}
					case *big.Int:
						switch r := rhs().(type) {
						case int64:
							return l.Cmp(big.NewInt(r)) == 1
						case *big.Int:
							return l.Cmp(r) == 1
						default:
							emitError(fmt.Sprintf("Invalid binary operation: <%s> > <%s>", errorTypeDict[fmt.Sprint(reflect.TypeOf(l))], errorTypeDict[fmt.Sprint(reflect.TypeOf(r))]))
						}
					default:
						emitError(fmt.Sprintf("Invalid binary operation: <%s> > ...", errorTypeDict[fmt.Sprint(reflect.TypeOf(l))]))
					}
					return nil
				}
			case "Gte":
				return func() interface{} {
					switch l := lhs().(type) {
					case int64:
						switch r := rhs().(type) {
						case int64:
							return l >= r
						case *big.Int:
							return r.Cmp(big.NewInt(l)) <= 0
						default:
							emitError(fmt.Sprintf("Invalid binary operation: <%s> > <%s>", errorTypeDict[fmt.Sprint(reflect.TypeOf(l))], errorTypeDict[fmt.Sprint(reflect.TypeOf(r))]))
						}
					case *big.Int:
						switch r := rhs().(type) {
						case int64:
							return l.Cmp(big.NewInt(r)) >= 0
						case *big.Int:
							return l.Cmp(r) >= 0
						default:
							emitError(fmt.Sprintf("Invalid binary operation: <%s> > <%s>", errorTypeDict[fmt.Sprint(reflect.TypeOf(l))], errorTypeDict[fmt.Sprint(reflect.TypeOf(r))]))
						}
					default:
						emitError(fmt.Sprintf("Invalid binary operation: <%s> > ...", errorTypeDict[fmt.Sprint(reflect.TypeOf(l))]))
					}
					return nil
				}
			case "Eq", "Neq":
				sign := true
				if term["op"] == "Neq" {
					sign = false
				}
				return func() interface{} {
					switch l := lhs().(type) {
					case int64:
						switch r := rhs().(type) {
						case int64:
							return l == r == sign
						case *big.Int:
							return r.Cmp(big.NewInt(l)) == 0 == sign
						default:
							emitError(fmt.Sprintf("Invalid binary operation: <%s> == <%s>", errorTypeDict[fmt.Sprint(reflect.TypeOf(l))], errorTypeDict[fmt.Sprint(reflect.TypeOf(r))]))
						}
					case *big.Int:
						switch r := rhs().(type) {
						case int64:
							return l.Cmp(big.NewInt(r)) == 0 == sign
						case *big.Int:
							return l.Cmp(r) == 0 == sign
						default:
							emitError(fmt.Sprintf("Invalid binary operation: <%s> == <%s>", errorTypeDict[fmt.Sprint(reflect.TypeOf(l))], errorTypeDict[fmt.Sprint(reflect.TypeOf(r))]))
						}

					case bool:
						switch r := rhs().(type) {
						case bool:
							return l == r == sign
						default:
							emitError(fmt.Sprintf("Invalid binary operation: <%s> == <%s>", errorTypeDict[fmt.Sprint(reflect.TypeOf(l))], errorTypeDict[fmt.Sprint(reflect.TypeOf(r))]))
						}
					case string:
						switch r := rhs().(type) {
						case string:
							return l == r == sign
						default:
							emitError(fmt.Sprintf("Invalid binary operation: <%s> == <%s>", errorTypeDict[fmt.Sprint(reflect.TypeOf(l))], errorTypeDict[fmt.Sprint(reflect.TypeOf(r))]))
						}
					default:
						emitError(fmt.Sprintf("Invalid binary operation: <%s> == ...", errorTypeDict[fmt.Sprint(reflect.TypeOf(l))]))
					}
					return nil
				}
			case "Or":
				return func() interface{} {
					switch l := lhs().(type) {
					case bool:
						switch r := rhs().(type) {
						case bool:
							return l || r
						default:
							emitError(fmt.Sprintf("Invalid binary operation: <%s> || <%s>", errorTypeDict[fmt.Sprint(reflect.TypeOf(l))], errorTypeDict[fmt.Sprint(reflect.TypeOf(r))]))
						}
					default:
						emitError(fmt.Sprintf("Invalid binary operation: <%s> || ...", errorTypeDict[fmt.Sprint(reflect.TypeOf(l))]))
					}
					return nil
				}
			case "And":
				return func() interface{} {
					switch l := lhs().(type) {
					case bool:
						switch r := rhs().(type) {
						case bool:
							return l && r
						default:
							emitError(fmt.Sprintf("Invalid binary operation: <%s> || <%s>", errorTypeDict[fmt.Sprint(reflect.TypeOf(l))], errorTypeDict[fmt.Sprint(reflect.TypeOf(r))]))
						}
					default:
						emitError(fmt.Sprintf("Invalid binary operation: <%s> || ...", errorTypeDict[fmt.Sprint(reflect.TypeOf(l))]))
					}
					return nil
				}
			}

		case "Print":
			val := bake(term["value"].(map[string]interface{}), memo)

			var print func(o interface{}) string
			print = func(o interface{}) string {
				switch v := o.(type) {
				case Closure:
					return "<#closure>"
				case Tuple:
					s := []string{}
					for _, d := range v {
						s = append(s, print(d))
					}
					return "(" + strings.Join(s, ", ") + ")"
				default:
					return fmt.Sprint(v)
				}
			}

			return func() interface{} {
				v := val()
				fmt.Println(print(v))
				return v
			}
		}
		return nil
	}

	return bake(ast, memo)
}
