package interpreter

import (
	"fmt"
	"math/big"
	"strconv"
	"strings"
)

func Bake(term map[string]interface{}, memo *Memory) Scope {
	if term["expression"] != nil {
		exp := term["expression"].(map[string]interface{})
		return func() interface{} {
			res := Bake(exp, memo)()
			fmt.Print(res)
			return nil
		}
	}

	switch term["kind"] {

	case "Int":
		//val := int(term["value"].(float64))
		val := int64(term["value"].(float64))
		return func() interface{} { return val }

	case "Str", "Bool":
		val := term["value"]
		return func() interface{} { return val }

	case "Tuple":
		first := Bake(term["first"].(map[string]interface{}), memo)
		second := Bake(term["second"].(map[string]interface{}), memo)
		return func() interface{} { return []interface{}{first(), second()} }

	case "Let":
		name := memo.MakeStack(term["name"].(map[string]interface{})["text"].(string))
		val := Bake(term["value"].(map[string]interface{}), memo)
		next := Bake(term["next"].(map[string]interface{}), memo)
		return func() interface{} {
			g := val()
			switch h := g.(type) {
			case []interface{}:
				if refs, ok := h[0].(*int); ok {
					//fmt.Println("função em let:", term["name"].(map[string]interface{})["text"].(string))
					*refs++
					defer h[1].(func())()
					//defer fmt.Println("---- unref função em let:", term["name"].(map[string]interface{})["text"].(string))
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
			return name.Value()
		}

	case "Call":
		callee := Bake(term["callee"].(map[string]interface{}), memo)

		memo = memo.Fork()
		args := make([]Scope, len(term["arguments"].([]interface{})))
		for i, t := range term["arguments"].([]interface{}) {
			args[i] = Bake(t.(map[string]interface{}), memo)
		}

		var params []*Stack
		var body Scope
		argsRes := make([]interface{}, len(args))
		return func() interface{} {
			if body == nil {
				a := callee().([]interface{})
				body = a[2].(Scope)
				params = a[3].([]*Stack)
			}

			for i, arg := range args {
				argsRes[i] = arg()
				//params[i].Push(argsRes[i])
				// switch h := argsRes[i].(type) {
				// case []interface{}:
				// 	refs := h[0].(*int)
				// 	*refs++
				// 	defer h[1].(func())()
				// }
			}
			for i, param := range params {
				param.Push(argsRes[i])
			}
			v := body()
			for _, param := range params {
				param.Pop()
			}
			return v
		}

	case "Function":
		memo = memo.Fork()
		body := Bake(term["value"].(map[string]interface{}), memo)
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
			return []interface{}{&references, onUnref, body, params}
		}

	case "If":
		condition := Bake(term["condition"].(map[string]interface{}), memo)
		then := Bake(term["then"].(map[string]interface{}), memo)
		otherwise := Bake(term["otherwise"].(map[string]interface{}), memo)
		return func() interface{} {
			if condition().(bool) {
				return then()
			}
			return otherwise()
		}

	case "Binary":
		lhs := Bake(term["lhs"].(map[string]interface{}), memo)
		rhs := Bake(term["rhs"].(map[string]interface{}), memo)

		// cache rhs
		if rightKind := term["rhs"].(map[string]interface{})["kind"]; rightKind == "Int" {
			switch term["op"] {
			case "Sub":
				r := rhs().(int64)
				return func() interface{} {
					return lhs().(int64) - r
				}
			case "Lt":
				r := rhs().(int64)
				return func() interface{} {
					return lhs().(int64) < r
				}
			case "Gt":
				r := rhs().(int64)
				return func() interface{} {
					return lhs().(int64) > r
				}
			case "Eq":
				r := rhs().(int64)
				return func() interface{} {
					return lhs().(int64) == r
				}
				// TODO: ops
			}
		} else if rightKind == "Bool" {
			switch term["op"] {
			case "Eq":
				v := rhs().(bool)
				return func() interface{} {
					return lhs().(bool) == v
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
					}
				case *big.Int:
					switch r := rhs().(type) {
					case int64:
						return big.NewInt(0).Add(l, big.NewInt(r))
					case *big.Int:
						return big.NewInt(0).Add(l, r)
					case string:
						return l.String() + r
					}
				case string:
					switch r := rhs().(type) {
					case int64:
						return l + strconv.FormatInt(r, 10)
					case *big.Int:
						return l + r.String()
					case string:
						return l + r
					}
				}
				// TODO: tratar quando for adição por valor errado
				return "<undefined>"
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
					case string:
						return strconv.FormatInt(l, 10) + r
					}
				case *big.Int:
					switch r := rhs().(type) {
					case int64:
						return big.NewInt(0).Sub(l, big.NewInt(r))
					case *big.Int:
						return big.NewInt(0).Sub(l, r)
					case string:
						return l.String() + r
					}
				}
				return "<undefined>"
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
					}
				case *big.Int:
					switch r := rhs().(type) {
					case int64:
						return l.Cmp(big.NewInt(r)) == -1
					case *big.Int:
						return l.Cmp(r) == -1
					}
				}
				return "<undefined>"
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
					}
				case *big.Int:
					switch r := rhs().(type) {
					case int64:
						return l.Cmp(big.NewInt(r)) == 1
					case *big.Int:
						return l.Cmp(r) == 1
					}
				}
				return "<undefined>"
			}
		case "Eq":
			return func() interface{} {
				switch l := lhs().(type) {
				case int64:
					switch r := rhs().(type) {
					case int64:
						return l == r
					case *big.Int:
						return r.Cmp(big.NewInt(l)) == 0
					}
				case *big.Int:
					switch r := rhs().(type) {
					case int64:
						return l.Cmp(big.NewInt(r)) == 0
					case *big.Int:
						return l.Cmp(r) == 0
					}

				case bool:
					return l == rhs().(bool)
				case string:
					return l == rhs().(string)
				}
				// TODO: tratar quando for adição por valor errado
				return "<undefined>"
			}
		case "Or":
			return func() interface{} {
				return lhs().(bool) || rhs().(bool)
			}
			// TODO: continuar outros operadores
		}

	case "Print":
		val := Bake(term["value"].(map[string]interface{}), memo)

		var print func(o interface{}) string
		print = func(o interface{}) string {
			switch v := o.(type) {
			case []interface{}:
				if _, ok := v[0].(*int); ok {
					return "<#closure>"
				} else {
					s := []string{}
					for _, d := range v {
						s = append(s, print(d))
					}
					return "(" + strings.Join(s, ", ") + ")"
				}
			default:
				return fmt.Sprint(v)
			}
		}

		return func() interface{} {
			s := print(val()) + "\n"
			return s
		}
	}
	return nil
}
