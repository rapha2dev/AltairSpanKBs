package interpreter

import (
	"fmt"
	"math/big"
	"os"
	"reflect"
	"slices"
	"strconv"
	"strings"
)

type Baked func() interface{}

func Bake(file string, memo *Memory) Baked {
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

	// -----
	lastBakedLet := ""
	scopedLets := []string{}
	isDirtyClosure := false
	closureDepth := 0

	var bake func(term map[string]interface{}, memo *Memory) Baked
	bake = func(term map[string]interface{}, memo *Memory) Baked {
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
			if len(code) > 0 {
				start := int(loc["start"].(float64))
				end := int(loc["end"].(float64))
				lines := strings.Split(code[:start], "\n")
				errorLine := len(lines)
				lineCol := -1
				for i := 0; i < errorLine-1; i++ {
					lineCol += len(lines[i]) + 1
				}
				fmt.Printf("\nError in file: '%s', line: %d, col: %d\n%s\n\n... %s ...\n\n\n", loc["filename"], errorLine, start-lineCol, fmt.Sprint(r), code[start:end])
			} else {
				fmt.Printf("\nError in file: '%s' (source code not found)\n\n... %s ...\n\n\n", loc["filename"], fmt.Sprint(r))
			}
			os.Exit(1)
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
			letName := term["name"].(map[string]interface{})["text"].(string)
			lastBakedLet = letName
			name := memo.MakeStack(letName)
			val := bake(term["value"].(map[string]interface{}), memo)
			lastBakedLet = ""
			scopedLets = append(scopedLets, letName)
			next := bake(term["next"].(map[string]interface{}), memo)
			return func() interface{} {
				g := val()
				switch h := g.(type) {
				case Closure:
					*h[0].(*int)++
					defer h[1].(func())()
				}

				if len(name.data) > 0 {
					last := name.data[len(name.data)-1]
					switch h := last.(type) {
					case Closure: // a função antiga armazenada no let não será mais pura
						h[4].(*Memoize).enabled = false
					}
				}

				name.Push(g)
				v := next()
				name.Pop()
				return v
			}

		case "Var":
			varName := term["text"].(string)
			name := memo.MakeStack(varName)
			isDirtyClosure = isDirtyClosure || !slices.Contains(scopedLets, varName)
			return func() interface{} {
				v := name.Value()
				if v == nil {
					emitError("var not found")
				}
				return v
			}

		case "Call":
			callee := bake(term["callee"].(map[string]interface{}), memo)
			args := make([]Baked, len(term["arguments"].([]interface{})))
			for i, t := range term["arguments"].([]interface{}) {
				args[i] = bake(t.(map[string]interface{}), memo)
			}

			argsLen := len(args)
			var params []*Stack
			var body Baked
			var memoize *Memoize
			return func() interface{} {
				//fmt.Println("call:", term["callee"].(map[string]interface{})["text"])
				if body == nil {
					x := callee()
					if a, ok := x.(Closure); ok {
						body = a[2].(Baked)
						params = a[3].([]*Stack)
						memoize = a[4].(*Memoize)
						if len(params) != argsLen {
							emitError("Wrong number of arguments")
						}
					} else {
						emitError(fmt.Sprintf("it is not possible to call a <%s>", errorTypeDict[fmt.Sprint(reflect.TypeOf(x))]))
					}
				}

				if memoize.enabled {
					key := ""
					for i, arg := range args {
						switch a := arg().(type) {
						case int64:
							key += strconv.FormatInt(a, 10) + ","
							params[i].PushParam(a)
						case *big.Int:
							key += a.String() + ","
							params[i].PushParam(a)
						default: // se não tiver valor valido desabilita a cache
							memoize.enabled = false
							params[i].PushParam(a)							
						}
					}
					for _, param := range params {
						param.PopParam()
					}
					if v, h := memoize.cache[key]; h {
						for _, param := range params {
							param.Pop()
						}
						memoize.cacheMiss = 0
						//fmt.Println("use memoize cache")
						return v
					} else if memoize.cacheSize == MemoizeCacheLimit {
						if memoize.cacheMiss == 1000000 {
							memoize.enabled = false
						} else {
							memoize.cacheMiss++
						}
					}
					v := body()
					for _, param := range params {
						param.Pop()
					}
					if memoize.enabled {
						if memoize.cacheSize >= MemoizeCacheLimit {
							//fmt.Println("cache full")
							for k := range memoize.cache {
								delete(memoize.cache, k)
								break
							}
						} else {
							memoize.cacheSize++
						}
						//fmt.Println("save memoize cache")
						memoize.cache[key] = v
					}
					return v
				} else {
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
			}

		case "Function":
			ownerLet := lastBakedLet
			memo = memo.Fork()

			befScopedLets := scopedLets
			scopedLets = []string{ownerLet}
			params := make([]*Stack, len(term["parameters"].([]interface{})))
			for i, p := range term["parameters"].([]interface{}) {
				paramName := p.(map[string]interface{})["text"].(string)
				params[i] = memo.MakeStack(paramName)
				scopedLets = append(scopedLets, paramName)
			}
			if closureDepth == 0 { // apenas reseta quando a função está no root
				isDirtyClosure = false
			}
			closureDepth++
			body := bake(term["value"].(map[string]interface{}), memo)
			closureDepth--
			defer func() { scopedLets = befScopedLets }()

			// Memoize
			memoize := &Memoize{cache: map[string]interface{}{}}
			memoize.enabled = ownerLet != "" && !isDirtyClosure
			// if memoize.enabled {
			// 	fmt.Println("memoize candidate let:", ownerLet, scopedLets)
			// }

			references := 0
			onUnref := func() {
				references--
				if references == 0 {
					memoize.cache = map[string]interface{}{}
					memo.OnUnreferenced()
				}
			}
			return func() interface{} {
				return Closure{&references, onUnref, body, params, memoize}
			}

		case "If":
			condition := bake(term["condition"].(map[string]interface{}), memo)
			then := bake(term["then"].(map[string]interface{}), memo)
			otherwise := bake(term["otherwise"].(map[string]interface{}), memo)
			return func() interface{} {
				v := condition()
				if b, ok := v.(bool); ok {
					if b {
						return then()
					}
				} else {
					emitError(fmt.Sprintf("Invalid type: if(<%s>)", errorTypeDict[fmt.Sprint(reflect.TypeOf(v))]))
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
							if r == 0 {
								emitError("Integer divide by zero")
							}
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

			case "Mul":
				return func() interface{} {
					switch l := lhs().(type) {
					case int64:
						switch r := rhs().(type) {
						case int64:
							return l * r
						default:
							emitError(fmt.Sprintf("Invalid binary operation: <%s> * <%s>", errorTypeDict[fmt.Sprint(reflect.TypeOf(l))], errorTypeDict[fmt.Sprint(reflect.TypeOf(r))]))
						}
					default:
						emitError(fmt.Sprintf("Invalid binary operation: <%s> * ...", errorTypeDict[fmt.Sprint(reflect.TypeOf(l))]))
					}
					// TODO: suportar bigint
					return nil
				}

			case "Div":
				return func() interface{} {
					switch l := lhs().(type) {
					case int64:
						switch r := rhs().(type) {
						case int64:
							if r == 0 {
								emitError("integer divide by zero")
							}
							return l / r
						default:
							emitError(fmt.Sprintf("Invalid binary operation: <%s> / <%s>", errorTypeDict[fmt.Sprint(reflect.TypeOf(l))], errorTypeDict[fmt.Sprint(reflect.TypeOf(r))]))
						}
					default:
						emitError(fmt.Sprintf("Invalid binary operation: <%s> / ...", errorTypeDict[fmt.Sprint(reflect.TypeOf(l))]))
					}
					// TODO: suportar bigint
					return nil
				}

			case "Rem":
				return func() interface{} {
					switch l := lhs().(type) {
					case int64:
						switch r := rhs().(type) {
						case int64:
							return l % r
						default:
							emitError(fmt.Sprintf("Invalid binary operation: <%s> %s <%s>", errorTypeDict[fmt.Sprint(reflect.TypeOf(l))], "%", errorTypeDict[fmt.Sprint(reflect.TypeOf(r))]))
						}
					default:
						emitError(fmt.Sprintf("Invalid binary operation: <%s> %s ...", "%", errorTypeDict[fmt.Sprint(reflect.TypeOf(l))]))
					}
					// TODO: suportar bigint
					return nil
				}

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
				op := "=="
				if term["op"] == "Neq" {
					sign = false
					op = "!="
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
							emitError(fmt.Sprintf("Invalid binary operation: <%s> %s <%s>", errorTypeDict[fmt.Sprint(reflect.TypeOf(l))], op, errorTypeDict[fmt.Sprint(reflect.TypeOf(r))]))
						}
					case *big.Int:
						switch r := rhs().(type) {
						case int64:
							return l.Cmp(big.NewInt(r)) == 0 == sign
						case *big.Int:
							return l.Cmp(r) == 0 == sign
						default:
							emitError(fmt.Sprintf("Invalid binary operation: <%s> %s <%s>", errorTypeDict[fmt.Sprint(reflect.TypeOf(l))], op, errorTypeDict[fmt.Sprint(reflect.TypeOf(r))]))
						}

					case bool:
						switch r := rhs().(type) {
						case bool:
							return l == r == sign
						default:
							emitError(fmt.Sprintf("Invalid binary operation: <%s> %s <%s>", errorTypeDict[fmt.Sprint(reflect.TypeOf(l))], op, errorTypeDict[fmt.Sprint(reflect.TypeOf(r))]))
						}
					case string:
						switch r := rhs().(type) {
						case string:
							return l == r == sign
						default:
							emitError(fmt.Sprintf("Invalid binary operation: <%s> %s <%s>", errorTypeDict[fmt.Sprint(reflect.TypeOf(l))], op, errorTypeDict[fmt.Sprint(reflect.TypeOf(r))]))
						}
					default:
						emitError(fmt.Sprintf("Invalid binary operation: <%s> %s ...", errorTypeDict[fmt.Sprint(reflect.TypeOf(l))], op))
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
							emitError(fmt.Sprintf("Invalid binary operation: <%s> && <%s>", errorTypeDict[fmt.Sprint(reflect.TypeOf(l))], errorTypeDict[fmt.Sprint(reflect.TypeOf(r))]))
						}
					default:
						emitError(fmt.Sprintf("Invalid binary operation: <%s> && ...", errorTypeDict[fmt.Sprint(reflect.TypeOf(l))]))
					}
					return nil
				}
			}

		case "Print":
			isDirtyClosure = true
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
