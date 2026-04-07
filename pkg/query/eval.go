package query

import (
	"fmt"
	"math"
	"regexp"
	"strconv"
	"strings"
)

// Eval evaluates an expression against a JSON value (any).
func Eval(expr Expr, data any) (any, error) {
	switch e := expr.(type) {
	// --- Literals ---
	case StringLiteral:
		return e.Value, nil
	case NumberLiteral:
		return e.Value, nil
	case BoolLiteral:
		return e.Value, nil
	case NullLiteral:
		return nil, nil
	case RegexLiteral:
		return e, nil

	// --- DotPath ---
	case DotPath:
		return evalDotPath(e.Path, data), nil

	// --- RecursiveDescent ---
	case RecursiveDescent:
		results := recursiveFind(data, e.Field)
		if len(results) == 0 {
			return nil, nil
		}
		if len(results) == 1 {
			return results[0], nil
		}
		return results, nil

	// --- Array operations (pointer receivers in tests) ---
	case *ArrayIndex:
		return evalArrayIndex(e, data)
	case ArrayIndex:
		return evalArrayIndex(&e, data)
	case *ArraySlice:
		return evalArraySlice(e, data)
	case ArraySlice:
		return evalArraySlice(&e, data)
	case *ArrayIterator:
		return evalArrayIterator(e, data)
	case ArrayIterator:
		return evalArrayIterator(&e, data)

	// --- BinaryOp ---
	case *BinaryOp:
		return evalBinaryOp(e, data)
	case BinaryOp:
		return evalBinaryOp(&e, data)

	// --- UnaryOp ---
	case *UnaryOp:
		return evalUnaryOp(e, data)
	case UnaryOp:
		return evalUnaryOp(&e, data)

	// --- FuncCall ---
	case *FuncCall:
		return evalFuncCall(e, data)
	case FuncCall:
		return evalFuncCall(&e, data)

	// --- StringTemplate ---
	case StringTemplate:
		return evalStringTemplate(e, data)
	case *StringTemplate:
		return evalStringTemplate(*e, data)

	// --- Special where expressions (evaluated as values) ---
	case *ContainsExpr:
		b, err := evalContains(e, data)
		if err != nil {
			return nil, err
		}
		return b, nil
	case ContainsExpr:
		b, err := evalContains(&e, data)
		if err != nil {
			return nil, err
		}
		return b, nil

	case *StartsWithExpr:
		b, err := evalStartsWith(e, data)
		return b, err
	case StartsWithExpr:
		b, err := evalStartsWith(&e, data)
		return b, err

	case *EndsWithExpr:
		b, err := evalEndsWith(e, data)
		return b, err
	case EndsWithExpr:
		b, err := evalEndsWith(&e, data)
		return b, err

	case *MatchesExpr:
		b, err := evalMatches(e, data)
		return b, err
	case MatchesExpr:
		b, err := evalMatches(&e, data)
		return b, err

	case *InExpr:
		b, err := evalIn(e, data)
		return b, err
	case InExpr:
		b, err := evalIn(&e, data)
		return b, err

	case *ExistsExpr:
		v, err := Eval(e.Expr, data)
		if err != nil {
			return false, nil
		}
		return v != nil, nil
	case ExistsExpr:
		v, err := Eval(e.Expr, data)
		if err != nil {
			return false, nil
		}
		return v != nil, nil

	case *IsNullExpr:
		v, err := Eval(e.Expr, data)
		if err != nil {
			return nil, err
		}
		return v == nil, nil
	case IsNullExpr:
		v, err := Eval(e.Expr, data)
		if err != nil {
			return nil, err
		}
		return v == nil, nil

	case *IsTypeExpr:
		return evalIsType(e, data)
	case IsTypeExpr:
		return evalIsType(&e, data)

	// --- Pipe ---
	case *PipeExpr:
		lv, err := Eval(e.Left, data)
		if err != nil {
			return nil, err
		}
		return Eval(e.Right, lv)
	case PipeExpr:
		lv, err := Eval(e.Left, data)
		if err != nil {
			return nil, err
		}
		return Eval(e.Right, lv)

	// --- Wildcard ---
	case *WildcardExpr:
		return evalWildcard(e, data)
	case WildcardExpr:
		return evalWildcard(&e, data)

	default:
		return nil, fmt.Errorf("unsupported expression type: %T", expr)
	}
}

// Match evaluates an expression as a boolean condition.
func Match(expr Expr, data any) (bool, error) {
	val, err := Eval(expr, data)
	if err != nil {
		return false, err
	}
	return toBool(val), nil
}

// ── DotPath ─────────────────────────────────────────────────────────

func evalDotPath(path string, data any) any {
	if path == "." {
		return data
	}
	// Strip leading dot
	if strings.HasPrefix(path, ".") {
		path = path[1:]
	}

	parts := splitDotPath(path)
	cur := data
	for _, part := range parts {
		if cur == nil {
			return nil
		}
		obj, ok := cur.(map[string]any)
		if !ok {
			return nil
		}
		v, exists := obj[part]
		if !exists {
			return nil
		}
		cur = v
	}
	return cur
}

// splitDotPath splits "a.b.c" into ["a","b","c"], handling brackets if present.
func splitDotPath(path string) []string {
	var parts []string
	for path != "" {
		// Handle bracket notation at start
		if path[0] == '[' {
			// skip brackets for now
			idx := strings.Index(path, "]")
			if idx < 0 {
				parts = append(parts, path)
				break
			}
			path = path[idx+1:]
			if strings.HasPrefix(path, ".") {
				path = path[1:]
			}
			continue
		}
		dot := strings.IndexAny(path, ".[")
		if dot < 0 {
			parts = append(parts, path)
			break
		}
		if dot > 0 {
			parts = append(parts, path[:dot])
		}
		if path[dot] == '.' {
			path = path[dot+1:]
		} else {
			path = path[dot:]
		}
	}
	return parts
}

// ── RecursiveDescent ────────────────────────────────────────────────

func recursiveFind(data any, field string) []any {
	var results []any
	switch v := data.(type) {
	case map[string]any:
		// Check this level first - sorted keys for deterministic order
		if val, ok := v[field]; ok {
			results = append(results, val)
		}
		// Then recurse into child values (sorted for determinism)
		keys := sortedKeys(v)
		for _, k := range keys {
			if k == field {
				continue
			}
			results = append(results, recursiveFind(v[k], field)...)
		}
	case []any:
		for _, item := range v {
			results = append(results, recursiveFind(item, field)...)
		}
	}
	return results
}

func sortedKeys(m map[string]any) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	// Sort for deterministic ordering
	for i := 1; i < len(keys); i++ {
		for j := i; j > 0 && keys[j] < keys[j-1]; j-- {
			keys[j], keys[j-1] = keys[j-1], keys[j]
		}
	}
	return keys
}

// ── Array operations ────────────────────────────────────────────────

func evalArrayIndex(e *ArrayIndex, data any) (any, error) {
	base, err := Eval(e.Expr, data)
	if err != nil {
		return nil, err
	}
	arr, ok := base.([]any)
	if !ok {
		return nil, nil
	}
	idx := e.Index
	if idx < 0 {
		idx = len(arr) + idx
	}
	if idx < 0 || idx >= len(arr) {
		return nil, nil
	}
	return arr[idx], nil
}

func evalArraySlice(e *ArraySlice, data any) (any, error) {
	base, err := Eval(e.Expr, data)
	if err != nil {
		return nil, err
	}
	arr, ok := base.([]any)
	if !ok {
		return nil, nil
	}
	start := 0
	end := len(arr)
	if e.Start != nil {
		start = *e.Start
	}
	if e.End != nil {
		end = *e.End
	}
	if start < 0 {
		start = len(arr) + start
	}
	if end < 0 {
		end = len(arr) + end
	}
	if start < 0 {
		start = 0
	}
	if end > len(arr) {
		end = len(arr)
	}
	if start >= end {
		return []any{}, nil
	}
	result := make([]any, end-start)
	copy(result, arr[start:end])
	return result, nil
}

func evalArrayIterator(e *ArrayIterator, data any) (any, error) {
	base, err := Eval(e.Expr, data)
	if err != nil {
		return nil, err
	}
	arr, ok := base.([]any)
	if !ok {
		return nil, nil
	}
	result := make([]any, len(arr))
	copy(result, arr)
	return result, nil
}

// ── BinaryOp ────────────────────────────────────────────────────────

func evalBinaryOp(e *BinaryOp, data any) (any, error) {
	switch e.Op {
	case "and":
		lv, err := Match(e.Left, data)
		if err != nil {
			return nil, err
		}
		if !lv {
			return false, nil
		}
		rv, err := Match(e.Right, data)
		if err != nil {
			return nil, err
		}
		return rv, nil

	case "or":
		lv, err := Match(e.Left, data)
		if err != nil {
			return nil, err
		}
		if lv {
			return true, nil
		}
		rv, err := Match(e.Right, data)
		if err != nil {
			return nil, err
		}
		return rv, nil
	}

	left, err := Eval(e.Left, data)
	if err != nil {
		return nil, err
	}
	right, err := Eval(e.Right, data)
	if err != nil {
		return nil, err
	}

	switch e.Op {
	case "==":
		return compareEq(left, right), nil
	case "!=":
		return !compareEq(left, right), nil
	case ">":
		return compareOrd(left, right) > 0, nil
	case "<":
		return compareOrd(left, right) < 0, nil
	case ">=":
		return compareOrd(left, right) >= 0, nil
	case "<=":
		return compareOrd(left, right) <= 0, nil
	case "+":
		return evalAdd(left, right)
	case "-":
		return evalArith(left, right, func(a, b float64) float64 { return a - b })
	case "*":
		return evalArith(left, right, func(a, b float64) float64 { return a * b })
	case "/":
		return evalDiv(left, right)
	case "%":
		return evalArith(left, right, func(a, b float64) float64 { return math.Mod(a, b) })
	default:
		return nil, fmt.Errorf("unknown operator: %s", e.Op)
	}
}

func compareEq(a, b any) bool {
	if a == nil && b == nil {
		return true
	}
	if a == nil || b == nil {
		return false
	}
	// Try numeric comparison
	af, aok := toFloat(a)
	bf, bok := toFloat(b)
	if aok && bok {
		return af == bf
	}
	return fmt.Sprintf("%v", a) == fmt.Sprintf("%v", b)
}

func compareOrd(a, b any) int {
	af, aok := toFloat(a)
	bf, bok := toFloat(b)
	if aok && bok {
		switch {
		case af < bf:
			return -1
		case af > bf:
			return 1
		default:
			return 0
		}
	}
	// String comparison fallback
	as := fmt.Sprintf("%v", a)
	bs := fmt.Sprintf("%v", b)
	switch {
	case as < bs:
		return -1
	case as > bs:
		return 1
	default:
		return 0
	}
}

func evalAdd(left, right any) (any, error) {
	if left == nil || right == nil {
		return nil, nil
	}
	// String concatenation
	ls, lok := left.(string)
	rs, rok := right.(string)
	if lok && rok {
		return ls + rs, nil
	}
	// Numeric addition
	lf, lok := toFloat(left)
	rf, rok := toFloat(right)
	if lok && rok {
		return lf + rf, nil
	}
	return nil, fmt.Errorf("cannot add %T and %T", left, right)
}

func evalArith(left, right any, op func(float64, float64) float64) (any, error) {
	if left == nil || right == nil {
		return nil, nil
	}
	lf, lok := toFloat(left)
	rf, rok := toFloat(right)
	if !lok || !rok {
		return nil, fmt.Errorf("cannot perform arithmetic on %T and %T", left, right)
	}
	return op(lf, rf), nil
}

func evalDiv(left, right any) (any, error) {
	if left == nil || right == nil {
		return nil, nil
	}
	lf, lok := toFloat(left)
	rf, rok := toFloat(right)
	if !lok || !rok {
		return nil, fmt.Errorf("cannot divide %T by %T", left, right)
	}
	if rf == 0 {
		return math.Inf(1), nil
	}
	return lf / rf, nil
}

// ── UnaryOp ─────────────────────────────────────────────────────────

func evalUnaryOp(e *UnaryOp, data any) (any, error) {
	switch e.Op {
	case "not":
		b, err := Match(e.Expr, data)
		if err != nil {
			return nil, err
		}
		return !b, nil
	case "-":
		v, err := Eval(e.Expr, data)
		if err != nil {
			return nil, err
		}
		f, ok := toFloat(v)
		if !ok {
			return nil, fmt.Errorf("cannot negate %T", v)
		}
		return -f, nil
	default:
		return nil, fmt.Errorf("unknown unary operator: %s", e.Op)
	}
}

// ── FuncCall ────────────────────────────────────────────────────────

func evalFuncCall(e *FuncCall, data any) (any, error) {
	switch e.Name {
	case "length":
		if len(e.Args) < 1 {
			return nil, fmt.Errorf("length requires 1 argument")
		}
		v, err := Eval(e.Args[0], data)
		if err != nil {
			return nil, err
		}
		switch val := v.(type) {
		case string:
			return float64(len(val)), nil
		case []any:
			return float64(len(val)), nil
		case map[string]any:
			return float64(len(val)), nil
		case nil:
			return nil, nil
		default:
			return nil, fmt.Errorf("length: unsupported type %T", v)
		}

	case "lower":
		s, err := evalStringArg(e, 0, data)
		if err != nil {
			return nil, err
		}
		return strings.ToLower(s), nil

	case "upper":
		s, err := evalStringArg(e, 0, data)
		if err != nil {
			return nil, err
		}
		return strings.ToUpper(s), nil

	case "trim":
		s, err := evalStringArg(e, 0, data)
		if err != nil {
			return nil, err
		}
		return strings.TrimSpace(s), nil

	case "abs":
		f, err := evalFloatArg(e, 0, data)
		if err != nil {
			return nil, err
		}
		return math.Abs(f), nil

	case "floor":
		f, err := evalFloatArg(e, 0, data)
		if err != nil {
			return nil, err
		}
		return math.Floor(f), nil

	case "ceil":
		f, err := evalFloatArg(e, 0, data)
		if err != nil {
			return nil, err
		}
		return math.Ceil(f), nil

	case "round":
		f, err := evalFloatArg(e, 0, data)
		if err != nil {
			return nil, err
		}
		if len(e.Args) >= 2 {
			prec, err := evalFloatArg(e, 1, data)
			if err != nil {
				return nil, err
			}
			p := math.Pow(10, prec)
			return math.Round(f*p) / p, nil
		}
		return math.Round(f), nil

	case "sqrt":
		f, err := evalFloatArg(e, 0, data)
		if err != nil {
			return nil, err
		}
		return math.Sqrt(f), nil

	case "pow":
		a, err := evalFloatArg(e, 0, data)
		if err != nil {
			return nil, err
		}
		b, err := evalFloatArg(e, 1, data)
		if err != nil {
			return nil, err
		}
		return math.Pow(a, b), nil

	case "coalesce":
		for _, arg := range e.Args {
			v, err := Eval(arg, data)
			if err != nil {
				return nil, err
			}
			if v != nil {
				return v, nil
			}
		}
		return nil, nil

	case "if":
		if len(e.Args) < 3 {
			return nil, fmt.Errorf("if requires 3 arguments")
		}
		cond, err := Match(e.Args[0], data)
		if err != nil {
			return nil, err
		}
		if cond {
			return Eval(e.Args[1], data)
		}
		return Eval(e.Args[2], data)

	case "type":
		if len(e.Args) < 1 {
			return nil, fmt.Errorf("type requires 1 argument")
		}
		v, err := Eval(e.Args[0], data)
		if err != nil {
			return nil, err
		}
		return typeName(v), nil

	case "keys":
		if len(e.Args) < 1 {
			return nil, fmt.Errorf("keys requires 1 argument")
		}
		v, err := Eval(e.Args[0], data)
		if err != nil {
			return nil, err
		}
		obj, ok := v.(map[string]any)
		if !ok {
			return nil, fmt.Errorf("keys: not an object")
		}
		keys := sortedKeys(obj)
		result := make([]any, len(keys))
		for i, k := range keys {
			result[i] = k
		}
		return result, nil

	case "values":
		if len(e.Args) < 1 {
			return nil, fmt.Errorf("values requires 1 argument")
		}
		v, err := Eval(e.Args[0], data)
		if err != nil {
			return nil, err
		}
		obj, ok := v.(map[string]any)
		if !ok {
			return nil, fmt.Errorf("values: not an object")
		}
		keys := sortedKeys(obj)
		result := make([]any, len(keys))
		for i, k := range keys {
			result[i] = obj[k]
		}
		return result, nil

	case "split":
		s, err := evalStringArg(e, 0, data)
		if err != nil {
			return nil, err
		}
		sep, err := evalStringArg(e, 1, data)
		if err != nil {
			return nil, err
		}
		parts := strings.Split(s, sep)
		result := make([]any, len(parts))
		for i, p := range parts {
			result[i] = p
		}
		return result, nil

	case "join":
		if len(e.Args) < 2 {
			return nil, fmt.Errorf("join requires 2 arguments")
		}
		v, err := Eval(e.Args[0], data)
		if err != nil {
			return nil, err
		}
		arr, ok := v.([]any)
		if !ok {
			return nil, fmt.Errorf("join: first argument must be array")
		}
		sep, err := evalStringArg(e, 1, data)
		if err != nil {
			return nil, err
		}
		strs := make([]string, len(arr))
		for i, item := range arr {
			strs[i] = toString(item)
		}
		return strings.Join(strs, sep), nil

	case "replace":
		s, err := evalStringArg(e, 0, data)
		if err != nil {
			return nil, err
		}
		old, err := evalStringArg(e, 1, data)
		if err != nil {
			return nil, err
		}
		newStr, err := evalStringArg(e, 2, data)
		if err != nil {
			return nil, err
		}
		return strings.ReplaceAll(s, old, newStr), nil

	case "substr":
		s, err := evalStringArg(e, 0, data)
		if err != nil {
			return nil, err
		}
		start, err := evalFloatArg(e, 1, data)
		if err != nil {
			return nil, err
		}
		length, err := evalFloatArg(e, 2, data)
		if err != nil {
			return nil, err
		}
		si := int(start)
		li := int(length)
		if si < 0 {
			si = 0
		}
		if si >= len(s) {
			return "", nil
		}
		end := si + li
		if end > len(s) {
			end = len(s)
		}
		return s[si:end], nil

	case "to_number":
		s, err := evalStringArg(e, 0, data)
		if err != nil {
			return nil, err
		}
		f, perr := strconv.ParseFloat(s, 64)
		if perr != nil {
			return nil, fmt.Errorf("to_number: %v", perr)
		}
		return f, nil

	case "to_string":
		if len(e.Args) < 1 {
			return nil, fmt.Errorf("to_string requires 1 argument")
		}
		v, err := Eval(e.Args[0], data)
		if err != nil {
			return nil, err
		}
		return toString(v), nil

	case "regex_extract":
		s, err := evalStringArg(e, 0, data)
		if err != nil {
			return nil, err
		}
		re, err := evalRegexArg(e, 1, data)
		if err != nil {
			return nil, err
		}
		match := re.FindString(s)
		if match == "" {
			return nil, nil
		}
		return match, nil

	case "regex_extract_all":
		s, err := evalStringArg(e, 0, data)
		if err != nil {
			return nil, err
		}
		re, err := evalRegexArg(e, 1, data)
		if err != nil {
			return nil, err
		}
		matches := re.FindAllString(s, -1)
		result := make([]any, len(matches))
		for i, m := range matches {
			result[i] = m
		}
		return result, nil

	default:
		return nil, fmt.Errorf("unknown function: %s", e.Name)
	}
}

// ── Special where expression evaluators ─────────────────────────────

func evalContains(e *ContainsExpr, data any) (bool, error) {
	haystack, err := Eval(e.Haystack, data)
	if err != nil {
		return false, err
	}
	needle, err := Eval(e.Needle, data)
	if err != nil {
		return false, err
	}
	switch h := haystack.(type) {
	case string:
		ns, ok := needle.(string)
		if !ok {
			return false, nil
		}
		return strings.Contains(h, ns), nil
	case []any:
		for _, item := range h {
			if compareEq(item, needle) {
				return true, nil
			}
		}
		return false, nil
	case map[string]any:
		// For maps, check if any value contains the needle (deep string search)
		ns, ok := needle.(string)
		if !ok {
			return false, nil
		}
		return mapContainsString(h, ns), nil
	}
	return false, nil
}

// mapContainsString recursively checks if any string value in a map contains the needle.
func mapContainsString(m map[string]any, needle string) bool {
	for _, v := range m {
		switch val := v.(type) {
		case string:
			if strings.Contains(val, needle) {
				return true
			}
		case map[string]any:
			if mapContainsString(val, needle) {
				return true
			}
		case []any:
			for _, item := range val {
				if s, ok := item.(string); ok && strings.Contains(s, needle) {
					return true
				}
				if sm, ok := item.(map[string]any); ok && mapContainsString(sm, needle) {
					return true
				}
			}
		}
	}
	return false
}

func evalStartsWith(e *StartsWithExpr, data any) (bool, error) {
	v, err := Eval(e.Expr, data)
	if err != nil {
		return false, err
	}
	p, err := Eval(e.Prefix, data)
	if err != nil {
		return false, err
	}
	vs, vok := v.(string)
	ps, pok := p.(string)
	if !vok || !pok {
		return false, nil
	}
	return strings.HasPrefix(vs, ps), nil
}

func evalEndsWith(e *EndsWithExpr, data any) (bool, error) {
	v, err := Eval(e.Expr, data)
	if err != nil {
		return false, err
	}
	s, err := Eval(e.Suffix, data)
	if err != nil {
		return false, err
	}
	vs, vok := v.(string)
	ss, sok := s.(string)
	if !vok || !sok {
		return false, nil
	}
	return strings.HasSuffix(vs, ss), nil
}

func evalMatches(e *MatchesExpr, data any) (bool, error) {
	v, err := Eval(e.Expr, data)
	if err != nil {
		return false, err
	}
	vs, ok := v.(string)
	if !ok {
		return false, nil
	}

	// Get regex from the Regex field
	var re *regexp.Regexp
	switch r := e.Regex.(type) {
	case RegexLiteral:
		re, err = compileRegex(r.Pattern, r.Flags)
	case *RegexLiteral:
		re, err = compileRegex(r.Pattern, r.Flags)
	default:
		// Evaluate as expression, expect string
		rv, rerr := Eval(e.Regex, data)
		if rerr != nil {
			return false, rerr
		}
		rs, ok := rv.(string)
		if !ok {
			return false, fmt.Errorf("matches: regex must be string or RegexLiteral")
		}
		re, err = regexp.Compile(rs)
	}
	if err != nil {
		return false, err
	}
	return re.MatchString(vs), nil
}

func evalIn(e *InExpr, data any) (bool, error) {
	v, err := Eval(e.Expr, data)
	if err != nil {
		return false, err
	}
	for _, valExpr := range e.Values {
		candidate, err := Eval(valExpr, data)
		if err != nil {
			return false, err
		}
		if compareEq(v, candidate) {
			return true, nil
		}
	}
	return false, nil
}

func evalIsType(e *IsTypeExpr, data any) (any, error) {
	v, err := Eval(e.Expr, data)
	if err != nil {
		return nil, err
	}
	tn := typeName(v)
	target := e.TypeName
	// Normalize: "bool" and "boolean" both match
	if target == "bool" {
		return tn == "boolean", nil
	}
	if target == "boolean" {
		return tn == "boolean", nil
	}
	return tn == target, nil
}

// ── StringTemplate ──────────────────────────────────────────────────

func evalStringTemplate(e StringTemplate, data any) (any, error) {
	var sb strings.Builder
	for _, part := range e.Parts {
		v, err := Eval(part, data)
		if err != nil {
			return nil, err
		}
		sb.WriteString(toString(v))
	}
	return sb.String(), nil
}

// ── Wildcard ────────────────────────────────────────────────────────

func evalWildcard(e *WildcardExpr, data any) (any, error) {
	obj, ok := data.(map[string]any)
	if !ok {
		return nil, nil
	}
	result := make([]any, 0, len(obj))
	for _, v := range obj {
		result = append(result, v)
	}
	return result, nil
}

// ── Helpers ─────────────────────────────────────────────────────────

func toBool(v any) bool {
	if v == nil {
		return false
	}
	switch val := v.(type) {
	case bool:
		return val
	case float64:
		return val != 0
	case string:
		return val != ""
	case []any:
		return len(val) > 0
	case map[string]any:
		return len(val) > 0
	}
	return true
}

func toFloat(v any) (float64, bool) {
	switch val := v.(type) {
	case float64:
		return val, true
	case int:
		return float64(val), true
	case int64:
		return float64(val), true
	}
	return 0, false
}

func toString(v any) string {
	if v == nil {
		return "null"
	}
	switch val := v.(type) {
	case string:
		return val
	case float64:
		if val == math.Trunc(val) && !math.IsInf(val, 0) {
			return strconv.FormatInt(int64(val), 10)
		}
		return strconv.FormatFloat(val, 'f', -1, 64)
	case bool:
		if val {
			return "true"
		}
		return "false"
	default:
		return fmt.Sprintf("%v", v)
	}
}

func typeName(v any) string {
	if v == nil {
		return "null"
	}
	switch v.(type) {
	case string:
		return "string"
	case float64:
		return "number"
	case bool:
		return "boolean"
	case []any:
		return "array"
	case map[string]any:
		return "object"
	default:
		return "unknown"
	}
}

func compileRegex(pattern, flags string) (*regexp.Regexp, error) {
	if strings.Contains(flags, "i") {
		pattern = "(?i)" + pattern
	}
	return regexp.Compile(pattern)
}

func evalStringArg(e *FuncCall, idx int, data any) (string, error) {
	if idx >= len(e.Args) {
		return "", fmt.Errorf("%s: missing argument %d", e.Name, idx)
	}
	v, err := Eval(e.Args[idx], data)
	if err != nil {
		return "", err
	}
	s, ok := v.(string)
	if !ok {
		return "", fmt.Errorf("%s: argument %d must be string, got %T", e.Name, idx, v)
	}
	return s, nil
}

func evalFloatArg(e *FuncCall, idx int, data any) (float64, error) {
	if idx >= len(e.Args) {
		return 0, fmt.Errorf("%s: missing argument %d", e.Name, idx)
	}
	v, err := Eval(e.Args[idx], data)
	if err != nil {
		return 0, err
	}
	f, ok := toFloat(v)
	if !ok {
		return 0, fmt.Errorf("%s: argument %d must be number, got %T", e.Name, idx, v)
	}
	return f, nil
}

func evalRegexArg(e *FuncCall, idx int, data any) (*regexp.Regexp, error) {
	if idx >= len(e.Args) {
		return nil, fmt.Errorf("%s: missing argument %d", e.Name, idx)
	}
	arg := e.Args[idx]
	switch r := arg.(type) {
	case RegexLiteral:
		return compileRegex(r.Pattern, r.Flags)
	case *RegexLiteral:
		return compileRegex(r.Pattern, r.Flags)
	default:
		v, err := Eval(arg, data)
		if err != nil {
			return nil, err
		}
		s, ok := v.(string)
		if !ok {
			return nil, fmt.Errorf("%s: argument %d must be regex or string", e.Name, idx)
		}
		return regexp.Compile(s)
	}
}
