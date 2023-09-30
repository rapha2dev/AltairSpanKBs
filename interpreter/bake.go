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

type Baked struct {
	c        int
	executor func() interface{}
}

func (self *Baked) call(ch chan interface{}) {
	ch <- self.executor()
}
func (self *Baked) Call() interface{} {
	if self.c == 1000 {
		self.c = 0
		cb := make(chan interface{})
		go self.call(cb)
		return <-cb
	} else {
		self.c++
		return self.executor()
	}
}

func newBaked(executor func() interface{}) *Baked {
	return &Baked{executor: executor}
}

func Bake(file string) *Baked {
	code, ast := LoadAst(file)

	errorHandlers := []func(r interface{}){}
	errorTypeDict := map[string]string{
		"interpreter.Closure": "#closure",
		"interpreter.Tuple":   "tuple",
		"int64":               "int",
		"string":              "string",
		"bool":                "boolean",
	}

	// ----- pré runtime
	var scopeBuilder *ScopeBuilder
	lastBakedLet := ""
	scopedLets := []string{}
	isDirtyClosure := false
	closureDepth := 0
	// ----

	// ----- runtime
	var currScopeInstance *ScopeInstance
	currentErrorHandlerIndex := 0
	// -----

	var bake func(term map[string]interface{}) *Baked
	bake = func(term map[string]interface{}) *Baked {
		if term["expression"] != nil {
			exp := term["expression"].(map[string]interface{})
			return newBaked(func() interface{} {
				defer func() {
					if r := recover(); r != nil {
						errorHandlers[currentErrorHandlerIndex](r)
					}
				}()
				root := newScopeBuilder()
				scopeBuilder = root
				run := bake(exp)
				currScopeInstance = root.New()
				return run.Call()
			})
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
			return newBaked(func() interface{} { return val })

		case "Str", "Bool":
			val := term["value"]
			return newBaked(func() interface{} { return val })

		case "First":
			tuple := bake(term["value"].(map[string]interface{}))
			return newBaked(func() interface{} {
				v := tuple.Call()
				if t, ok := v.(Tuple); ok {
					return t[0]
				} else {
					emitError(fmt.Sprintf("Invalid tuple operation: first(<%s>)", errorTypeDict[fmt.Sprint(reflect.TypeOf(v))]))
					return nil
				}
			})

		case "Second":
			tuple := bake(term["value"].(map[string]interface{}))
			return newBaked(func() interface{} {
				v := tuple.Call()
				if t, ok := v.(Tuple); ok {
					return t[1]
				} else {
					emitError(fmt.Sprintf("Invalid tuple operation: second(<%s>)", errorTypeDict[fmt.Sprint(reflect.TypeOf(v))]))
					return nil
				}
			})

		case "Tuple":
			first := bake(term["first"].(map[string]interface{}))
			second := bake(term["second"].(map[string]interface{}))
			return newBaked(func() interface{} { return Tuple{first.Call(), second.Call()} })

		case "Let":
			letName := term["name"].(map[string]interface{})["text"].(string)
			lastBakedLet = letName
			name := scopeBuilder.Register(letName)
			val := bake(term["value"].(map[string]interface{}))
			lastBakedLet = ""
			scopedLets = append(scopedLets, letName)
			next := bake(term["next"].(map[string]interface{}))
			return newBaked(func() interface{} {
				g := val.Call()
				prev := currScopeInstance.Value(name, currScopeInstance.scope)
				if prev != nil {
					// a função antiga armazenada no let não será mais pura
					if h, ok := prev.(*ScopeInstance); ok && h.scope.memoize != nil {
						h.scope.memoize.enabled = false
						h.scope.memoize.cache = nil
					}
				}
				currScopeInstance.Set(name, g)
				return next.Call()
			})

		case "Var":
			scope := scopeBuilder
			varName := term["text"].(string)
			name := scopeBuilder.Register(varName)
			isDirtyClosure = isDirtyClosure || !slices.Contains(scopedLets, varName)
			return newBaked(func() interface{} {
				v := currScopeInstance.Value(name, scope)
				if v == nil {
					v = currScopeInstance.parent.Find(varName)
				}
				if v == nil {
					emitError("var not found")
				}
				return v
			})

		case "Call":
			callee := bake(term["callee"].(map[string]interface{}))
			args := make([]*Baked, len(term["arguments"].([]interface{})))
			for i, t := range term["arguments"].([]interface{}) {
				args[i] = bake(t.(map[string]interface{}))
			}

			argsLen := len(args)
			var scopeInstance *ScopeInstance
			return newBaked(func() interface{} {
				x := callee.Call()
				if a, ok := x.(*ScopeInstance); ok {
					scopeInstance = a
					if len(a.scope.paramIndexes) != argsLen {
						emitError("Wrong number of arguments")
					}
				} else {
					emitError(fmt.Sprintf("it is not possible to call a <%s>", errorTypeDict[fmt.Sprint(reflect.TypeOf(x))]))
				}

				params := scopeInstance.scope.paramIndexes
				if scopeInstance.scope.memoize.enabled {
					memoize := scopeInstance.scope.memoize
					key := ""
					child := scopeInstance.Child(scopeInstance.scope)
					for i, arg := range args {
						switch a := arg.Call().(type) {
						case int64:
							key += strconv.FormatInt(a, 10) + ","
							child.Set(params[i], arg.Call())
						case *big.Int:
							key += a.String() + ","
							child.Set(params[i], arg.Call())
						default: // se não tiver valor valido desabilita a cache
							memoize.enabled = false
							child.Set(params[i], arg.Call())
						}
					}
					if v, h := memoize.cache[key]; h {
						memoize.cacheMiss = 0
						return v
					} else if memoize.cacheSize == MemoizeCacheLimit {
						if memoize.cacheMiss == 1000000 {
							memoize.enabled = false
						} else {
							memoize.cacheMiss++
						}
					}
					prev := currScopeInstance
					currScopeInstance = child
					v := scopeInstance.scope.body.Call()
					currScopeInstance = prev
					if memoize.enabled {
						if memoize.cacheSize >= MemoizeCacheLimit {
							for k := range memoize.cache {
								delete(memoize.cache, k)
								break
							}
						} else {
							memoize.cacheSize++
						}
						memoize.cache[key] = v
					}
					return v
				} else {
					child := scopeInstance.Child(scopeInstance.scope)
					for i, arg := range args {
						child.Set(params[i], arg.Call())
					}
					prev := currScopeInstance
					currScopeInstance = child
					v := scopeInstance.scope.body.Call()
					currScopeInstance = prev
					return v
				}
			})

		case "Function":
			prevScope := scopeBuilder
			scope := newScopeBuilder()
			scopeBuilder = scope

			ownerLet := lastBakedLet
			prevScopedLets := scopedLets
			scopedLets = []string{ownerLet}
			scope.paramIndexes = make([]int, len(term["parameters"].([]interface{})))
			for i, p := range term["parameters"].([]interface{}) {
				paramName := p.(map[string]interface{})["text"].(string)
				scope.paramIndexes[i] = scope.Register(paramName)
				scopedLets = append(scopedLets, paramName)
			}
			if closureDepth == 0 { // apenas reseta quando a função está no root
				isDirtyClosure = false
			}
			closureDepth++
			scope.body = bake(term["value"].(map[string]interface{}))
			closureDepth--
			scopeBuilder = prevScope
			defer func() { scopedLets = prevScopedLets }()

			// Memoize
			scope.memoize = &Memoize{cache: map[string]interface{}{}}
			scope.memoize.enabled = ownerLet != "" && !isDirtyClosure

			return newBaked(func() interface{} {
				return currScopeInstance.Child(scope)
			})

		case "If":
			condition := bake(term["condition"].(map[string]interface{}))
			then := bake(term["then"].(map[string]interface{}))
			otherwise := bake(term["otherwise"].(map[string]interface{}))
			return newBaked(func() interface{} {
				v := condition.Call()
				if b, ok := v.(bool); ok {
					if b {
						return then.Call()
					}
				} else {
					emitError(fmt.Sprintf("Invalid type: if(<%s>)", errorTypeDict[fmt.Sprint(reflect.TypeOf(v))]))
				}
				return otherwise.Call()
			})

		case "Binary":
			lhs := bake(term["lhs"].(map[string]interface{}))
			rhs := bake(term["rhs"].(map[string]interface{}))

			// otimização quando o rhs é um literal Int ou Bool
			if rightKind := term["rhs"].(map[string]interface{})["kind"]; rightKind == "Int" {
				switch term["op"] {
				case "Sub":
					r := rhs.Call().(int64)
					return newBaked(func() interface{} {
						switch l := lhs.Call().(type) {
						case int64:
							return l - r
						case *big.Int:
							return big.NewInt(0).Sub(l, big.NewInt(r))
						default:
							emitError(fmt.Sprintf("Invalid binary operation: <%s> - <%s>", errorTypeDict[fmt.Sprint(reflect.TypeOf(l))], errorTypeDict[fmt.Sprint(reflect.TypeOf(r))]))
						}
						return nil
					})
				case "Mul":
					r := rhs.Call().(int64)
					return newBaked(func() interface{} {
						switch l := lhs.Call().(type) {
						case int64:
							return l * r
						case *big.Int:
							return big.NewInt(0).Mul(l, big.NewInt(r))
						default:
							emitError(fmt.Sprintf("Invalid binary operation: <%s> * <%s>", errorTypeDict[fmt.Sprint(reflect.TypeOf(l))], errorTypeDict[fmt.Sprint(reflect.TypeOf(r))]))
						}
						return nil
					})
				case "Div":
					r := rhs.Call().(int64)
					return newBaked(func() interface{} {
						switch l := lhs.Call().(type) {
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
					})
				case "Rem":
					r := rhs.Call().(int64)
					return newBaked(func() interface{} {
						switch l := lhs.Call().(type) {
						case int64:
							return l % r
						case *big.Int:
							return big.NewInt(0).Rem(l, big.NewInt(r))
						default:
							emitError(fmt.Sprintf("Invalid binary operation: <%s> %s <%s>", errorTypeDict[fmt.Sprint(reflect.TypeOf(l))], "%", errorTypeDict[fmt.Sprint(reflect.TypeOf(r))]))
						}
						return nil
					})
				case "Lt":
					r := rhs.Call().(int64)
					return newBaked(func() interface{} {
						switch l := lhs.Call().(type) {
						case int64:
							return l < r
						case *big.Int:
							return l.Cmp(big.NewInt(r)) == -1
						default:
							emitError(fmt.Sprintf("Invalid binary operation: <%s> < <%s>", errorTypeDict[fmt.Sprint(reflect.TypeOf(l))], errorTypeDict[fmt.Sprint(reflect.TypeOf(r))]))
						}
						return nil
					})
				case "Lte":
					r := rhs.Call().(int64)
					return newBaked(func() interface{} {
						switch l := lhs.Call().(type) {
						case int64:
							return l <= r
						case *big.Int:
							return l.Cmp(big.NewInt(r)) <= 0
						default:
							emitError(fmt.Sprintf("Invalid binary operation: <%s> < <%s>", errorTypeDict[fmt.Sprint(reflect.TypeOf(l))], errorTypeDict[fmt.Sprint(reflect.TypeOf(r))]))
						}
						return nil
					})
				case "Gt":
					r := rhs.Call().(int64)
					return newBaked(func() interface{} {
						switch l := lhs.Call().(type) {
						case int64:
							return l > r
						case *big.Int:
							return l.Cmp(big.NewInt(r)) == 1
						default:
							emitError(fmt.Sprintf("Invalid binary operation: <%s> > <%s>", errorTypeDict[fmt.Sprint(reflect.TypeOf(l))], errorTypeDict[fmt.Sprint(reflect.TypeOf(r))]))
						}
						return nil
					})
				case "Gte":
					r := rhs.Call().(int64)
					return newBaked(func() interface{} {
						switch l := lhs.Call().(type) {
						case int64:
							return l >= r
						case *big.Int:
							return l.Cmp(big.NewInt(r)) >= 0
						default:
							emitError(fmt.Sprintf("Invalid binary operation: <%s> > <%s>", errorTypeDict[fmt.Sprint(reflect.TypeOf(l))], errorTypeDict[fmt.Sprint(reflect.TypeOf(r))]))
						}
						return nil
					})
				case "Eq":
					r := rhs.Call().(int64)
					return newBaked(func() interface{} {
						switch l := lhs.Call().(type) {
						case int64:
							return l == r
						case *big.Int:
							return l.Cmp(big.NewInt(r)) == 0
						default:
							emitError(fmt.Sprintf("Invalid binary operation: <%s> == <%s>", errorTypeDict[fmt.Sprint(reflect.TypeOf(l))], errorTypeDict[fmt.Sprint(reflect.TypeOf(r))]))
						}
						return nil
					})
				}
			} else if rightKind == "Bool" {
				switch term["op"] {
				case "Eq":
					r := rhs.Call().(bool)
					return newBaked(func() interface{} {
						switch l := lhs.Call().(type) {
						case bool:
							return l == r
						default:
							emitError(fmt.Sprintf("Invalid binary operation: <%s> == <%s>", errorTypeDict[fmt.Sprint(reflect.TypeOf(l))], errorTypeDict[fmt.Sprint(reflect.TypeOf(r))]))
						}
						return nil
					})
				}
			}

			switch term["op"] {
			case "Add":
				return newBaked(func() interface{} {
					switch l := lhs.Call().(type) {
					case int64:
						switch r := rhs.Call().(type) {
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
						switch r := rhs.Call().(type) {
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
						switch r := rhs.Call().(type) {
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
				})
			case "Sub":
				return newBaked(func() interface{} {
					switch l := lhs.Call().(type) {
					case int64:
						switch r := rhs.Call().(type) {
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
						switch r := rhs.Call().(type) {
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
				})

			case "Mul":
				return newBaked(func() interface{} {
					switch l := lhs.Call().(type) {
					case int64:
						switch r := rhs.Call().(type) {
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
				})

			case "Div":
				return newBaked(func() interface{} {
					switch l := lhs.Call().(type) {
					case int64:
						switch r := rhs.Call().(type) {
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
				})

			case "Rem":
				return newBaked(func() interface{} {
					switch l := lhs.Call().(type) {
					case int64:
						switch r := rhs.Call().(type) {
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
				})

			case "Lt":
				return newBaked(func() interface{} {
					switch l := lhs.Call().(type) {
					case int64:
						switch r := rhs.Call().(type) {
						case int64:
							return l < r
						case *big.Int:
							return r.Cmp(big.NewInt(l)) == 1
						default:
							emitError(fmt.Sprintf("Invalid binary operation: <%s> < <%s>", errorTypeDict[fmt.Sprint(reflect.TypeOf(l))], errorTypeDict[fmt.Sprint(reflect.TypeOf(r))]))
						}
					case *big.Int:
						switch r := rhs.Call().(type) {
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
				})
			case "Lte":
				return newBaked(func() interface{} {
					switch l := lhs.Call().(type) {
					case int64:
						switch r := rhs.Call().(type) {
						case int64:
							return l <= r
						case *big.Int:
							return r.Cmp(big.NewInt(l)) >= 0
						default:
							emitError(fmt.Sprintf("Invalid binary operation: <%s> < <%s>", errorTypeDict[fmt.Sprint(reflect.TypeOf(l))], errorTypeDict[fmt.Sprint(reflect.TypeOf(r))]))
						}
					case *big.Int:
						switch r := rhs.Call().(type) {
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
				})
			case "Gt":
				return newBaked(func() interface{} {
					switch l := lhs.Call().(type) {
					case int64:
						switch r := rhs.Call().(type) {
						case int64:
							return l > r
						case *big.Int:
							return r.Cmp(big.NewInt(l)) == -1
						default:
							emitError(fmt.Sprintf("Invalid binary operation: <%s> > <%s>", errorTypeDict[fmt.Sprint(reflect.TypeOf(l))], errorTypeDict[fmt.Sprint(reflect.TypeOf(r))]))
						}
					case *big.Int:
						switch r := rhs.Call().(type) {
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
				})
			case "Gte":
				return newBaked(func() interface{} {
					switch l := lhs.Call().(type) {
					case int64:
						switch r := rhs.Call().(type) {
						case int64:
							return l >= r
						case *big.Int:
							return r.Cmp(big.NewInt(l)) <= 0
						default:
							emitError(fmt.Sprintf("Invalid binary operation: <%s> > <%s>", errorTypeDict[fmt.Sprint(reflect.TypeOf(l))], errorTypeDict[fmt.Sprint(reflect.TypeOf(r))]))
						}
					case *big.Int:
						switch r := rhs.Call().(type) {
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
				})
			case "Eq", "Neq":
				sign := true
				op := "=="
				if term["op"] == "Neq" {
					sign = false
					op = "!="
				}
				return newBaked(func() interface{} {
					switch l := lhs.Call().(type) {
					case int64:
						switch r := rhs.Call().(type) {
						case int64:
							return l == r == sign
						case *big.Int:
							return r.Cmp(big.NewInt(l)) == 0 == sign
						default:
							emitError(fmt.Sprintf("Invalid binary operation: <%s> %s <%s>", errorTypeDict[fmt.Sprint(reflect.TypeOf(l))], op, errorTypeDict[fmt.Sprint(reflect.TypeOf(r))]))
						}
					case *big.Int:
						switch r := rhs.Call().(type) {
						case int64:
							return l.Cmp(big.NewInt(r)) == 0 == sign
						case *big.Int:
							return l.Cmp(r) == 0 == sign
						default:
							emitError(fmt.Sprintf("Invalid binary operation: <%s> %s <%s>", errorTypeDict[fmt.Sprint(reflect.TypeOf(l))], op, errorTypeDict[fmt.Sprint(reflect.TypeOf(r))]))
						}

					case bool:
						switch r := rhs.Call().(type) {
						case bool:
							return l == r == sign
						default:
							emitError(fmt.Sprintf("Invalid binary operation: <%s> %s <%s>", errorTypeDict[fmt.Sprint(reflect.TypeOf(l))], op, errorTypeDict[fmt.Sprint(reflect.TypeOf(r))]))
						}
					case string:
						switch r := rhs.Call().(type) {
						case string:
							return l == r == sign
						default:
							emitError(fmt.Sprintf("Invalid binary operation: <%s> %s <%s>", errorTypeDict[fmt.Sprint(reflect.TypeOf(l))], op, errorTypeDict[fmt.Sprint(reflect.TypeOf(r))]))
						}
					default:
						emitError(fmt.Sprintf("Invalid binary operation: <%s> %s ...", errorTypeDict[fmt.Sprint(reflect.TypeOf(l))], op))
					}
					return nil
				})
			case "Or":
				return newBaked(func() interface{} {
					switch l := lhs.Call().(type) {
					case bool:
						switch r := rhs.Call().(type) {
						case bool:
							return l || r
						default:
							emitError(fmt.Sprintf("Invalid binary operation: <%s> || <%s>", errorTypeDict[fmt.Sprint(reflect.TypeOf(l))], errorTypeDict[fmt.Sprint(reflect.TypeOf(r))]))
						}
					default:
						emitError(fmt.Sprintf("Invalid binary operation: <%s> || ...", errorTypeDict[fmt.Sprint(reflect.TypeOf(l))]))
					}
					return nil
				})
			case "And":
				return newBaked(func() interface{} {
					switch l := lhs.Call().(type) {
					case bool:
						switch r := rhs.Call().(type) {
						case bool:
							return l && r
						default:
							emitError(fmt.Sprintf("Invalid binary operation: <%s> && <%s>", errorTypeDict[fmt.Sprint(reflect.TypeOf(l))], errorTypeDict[fmt.Sprint(reflect.TypeOf(r))]))
						}
					default:
						emitError(fmt.Sprintf("Invalid binary operation: <%s> && ...", errorTypeDict[fmt.Sprint(reflect.TypeOf(l))]))
					}
					return nil
				})
			}

		case "Print":
			isDirtyClosure = true
			val := bake(term["value"].(map[string]interface{}))

			var print func(o interface{}) string
			print = func(o interface{}) string {
				switch v := o.(type) {
				case *ScopeInstance:
					if v.scope.memoize != nil {
						return "<#closure>"
					} else {
						return fmt.Sprint(v)
					}
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

			return newBaked(func() interface{} {
				v := val.Call()
				fmt.Println(print(v))
				return v
			})
		}
		return nil
	}

	return bake(ast)
}
