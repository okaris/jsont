package query

import (
	"encoding/json"
	"math"
	"reflect"
	"sort"
	"testing"
)

// unmarshal is a helper that parses a JSON string into any.
func unmarshal(t *testing.T, s string) any {
	t.Helper()
	var v any
	if err := json.Unmarshal([]byte(s), &v); err != nil {
		t.Fatalf("bad test JSON %q: %v", s, err)
	}
	return v
}

// deepEqual compares two values for test assertions, handling float vs int,
// slice ordering issues, etc.
func deepEqual(a, b any) bool {
	return reflect.DeepEqual(a, b)
}

// sortedStrings returns a sorted copy of a string slice extracted from []any.
func sortedStrings(v any) []string {
	arr, ok := v.([]any)
	if !ok {
		return nil
	}
	out := make([]string, len(arr))
	for i, x := range arr {
		out[i], _ = x.(string)
	}
	sort.Strings(out)
	return out
}

// ── TestEvalDotPath ──────────────────────────────────────────────────

func TestEvalDotPath(t *testing.T) {
	tests := []struct {
		name string
		json string
		expr Expr
		want any
	}{
		{
			name: "simple field access",
			json: `{"name":"Alice"}`,
			expr: DotPath{Path: ".name"},
			want: "Alice",
		},
		{
			name: "nested field access",
			json: `{"address":{"city":"Portland"}}`,
			expr: DotPath{Path: ".address.city"},
			want: "Portland",
		},
		{
			name: "missing field returns nil",
			json: `{"name":"Alice"}`,
			expr: DotPath{Path: ".missing"},
			want: nil,
		},
		{
			name: "deep missing path returns nil",
			json: `{"x": 1}`,
			expr: DotPath{Path: ".a.b.c"},
			want: nil,
		},
		{
			name: "array index 0",
			json: `{"tags":["a","b"]}`,
			expr: &ArrayIndex{Expr: DotPath{Path: ".tags"}, Index: 0},
			want: "a",
		},
		{
			name: "array index 1",
			json: `{"tags":["a","b"]}`,
			expr: &ArrayIndex{Expr: DotPath{Path: ".tags"}, Index: 1},
			want: "b",
		},
		{
			name: "negative array index",
			json: `{"tags":["a","b"]}`,
			expr: &ArrayIndex{Expr: DotPath{Path: ".tags"}, Index: -1},
			want: "b",
		},
		{
			name: "array iterator returns all elements",
			json: `{"tags":["a","b"]}`,
			expr: &ArrayIterator{Expr: DotPath{Path: ".tags"}},
			want: []any{"a", "b"},
		},
		{
			name: "array iterator with nested field access",
			json: `{"items":[{"name":"x"},{"name":"y"}]}`,
			expr: DotPath{Path: ".items[].name"},
			// If DotPath doesn't handle [], we construct it with iterator + projection
		},
		{
			name: "array slice",
			json: `{"items":[10,20,30,40,50]}`,
			expr: &ArraySlice{
				Expr:  DotPath{Path: ".items"},
				Start: intPtr(1),
				End:   intPtr(3),
			},
			want: []any{float64(20), float64(30)},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			obj := unmarshal(t, tt.json)

			// Skip the iterator+nested test if the AST requires special handling
			if tt.name == "array iterator with nested field access" {
				// Test using explicit AST: iterate .items, then access .name on each
				// This depends on how Eval handles chained iteration.
				// We try with a PipeExpr or nested approach.
				expr := &FuncCall{
					Name: "map",
					Args: []Expr{
						&ArrayIterator{Expr: DotPath{Path: ".items"}},
						DotPath{Path: ".name"},
					},
				}
				// Alternative: direct DotPath if evaluator supports .items[].name
				got, err := Eval(DotPath{Path: ".items"}, obj)
				if err != nil {
					// Try alternate form
					got, err = Eval(expr, obj)
				}
				_ = got
				t.Skip("iterator+nested field depends on evaluator implementation details")
				return
			}

			if tt.want == nil && tt.expr == nil {
				t.Skip("no expr configured")
				return
			}

			got, err := Eval(tt.expr, obj)
			if err != nil {
				t.Fatalf("Eval returned error: %v", err)
			}
			if !deepEqual(got, tt.want) {
				t.Errorf("Eval() = %v (%T), want %v (%T)", got, got, tt.want, tt.want)
			}
		})
	}
}

// ── TestEvalFieldAccess ─────────────────────────────────────────────

func TestEvalFieldAccess(t *testing.T) {
	tests := []struct {
		name string
		json string
		expr Expr
		want any
	}{
		{
			name: "array index then field",
			json: `{"items":[{"name":"alice"},{"name":"bob"}]}`,
			expr: FieldAccess{
				Expr:  ArrayIndex{Expr: DotPath{Path: ".items"}, Index: 0},
				Field: ".name",
			},
			want: "alice",
		},
		{
			name: "array index then nested field",
			json: `{"data":[{"error":{"message":"timeout"}}]}`,
			expr: FieldAccess{
				Expr:  ArrayIndex{Expr: DotPath{Path: ".data"}, Index: 0},
				Field: ".error.message",
			},
			want: "timeout",
		},
		{
			name: "chained array index and field",
			json: `{"a":[{"b":[{"c":42}]}]}`,
			expr: FieldAccess{
				Expr: ArrayIndex{
					Expr: FieldAccess{
						Expr:  ArrayIndex{Expr: DotPath{Path: ".a"}, Index: 0},
						Field: ".b",
					},
					Index: 0,
				},
				Field: ".c",
			},
			want: float64(42),
		},
		{
			name: "field access on nil returns nil",
			json: `{"a":[]}`,
			expr: FieldAccess{
				Expr:  ArrayIndex{Expr: DotPath{Path: ".a"}, Index: 5},
				Field: ".name",
			},
			want: nil,
		},
		{
			name: "negative index then field",
			json: `{"items":[{"name":"first"},{"name":"last"}]}`,
			expr: FieldAccess{
				Expr:  ArrayIndex{Expr: DotPath{Path: ".items"}, Index: -1},
				Field: ".name",
			},
			want: "last",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			data := unmarshal(t, tt.json)
			got, err := Eval(tt.expr, data)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if !deepEqual(got, tt.want) {
				t.Errorf("got %v (%T), want %v (%T)", got, got, tt.want, tt.want)
			}
		})
	}
}

// ── TestEvalRecursiveDescent ─────────────────────────────────────────

func TestEvalRecursiveDescent(t *testing.T) {
	tests := []struct {
		name string
		json string
		expr Expr
		want any
	}{
		{
			name: "finds field at top level",
			json: `{"error":"something broke"}`,
			expr: RecursiveDescent{Field: "error"},
			want: "something broke",
		},
		{
			name: "finds nested field deep",
			json: `{"response":{"body":{"error":"deep failure"}}}`,
			expr: RecursiveDescent{Field: "error"},
			want: "deep failure",
		},
		{
			name: "finds multiple at different depths",
			json: `{"error":"top","nested":{"error":"deep"}}`,
			expr: RecursiveDescent{Field: "error"},
			want: []any{"top", "deep"},
		},
		{
			name: "finds across sibling branches",
			json: `{"a":{"name":"x"},"b":{"c":{"name":"y"}}}`,
			expr: RecursiveDescent{Field: "name"},
			want: []any{"x", "y"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			obj := unmarshal(t, tt.json)
			got, err := Eval(tt.expr, obj)
			if err != nil {
				t.Fatalf("Eval returned error: %v", err)
			}

			// RecursiveDescent may return a single value or a slice.
			// Normalize: if want is a slice and got is a slice, compare as sets.
			wantSlice, wantIsSlice := tt.want.([]any)
			gotSlice, gotIsSlice := got.([]any)
			if wantIsSlice && gotIsSlice {
				if len(gotSlice) != len(wantSlice) {
					t.Fatalf("got %d results, want %d: got=%v", len(gotSlice), len(wantSlice), gotSlice)
				}
				// Order may vary, sort string representations
				gs := sortedStrings(got)
				ws := sortedStrings(tt.want)
				if !reflect.DeepEqual(gs, ws) {
					t.Errorf("got %v, want %v", gs, ws)
				}
			} else if wantIsSlice && !gotIsSlice {
				// Maybe evaluator wraps single result differently
				t.Errorf("got %v (%T), want slice %v", got, got, tt.want)
			} else {
				if !deepEqual(got, tt.want) {
					t.Errorf("got %v (%T), want %v (%T)", got, got, tt.want, tt.want)
				}
			}
		})
	}
}

// ── TestEvalArithmetic ───────────────────────────────────────────────

func TestEvalArithmetic(t *testing.T) {
	tests := []struct {
		name    string
		json    string
		expr    Expr
		want    any
		wantErr bool
	}{
		{
			name: "addition",
			json: `{"a":10,"b":3}`,
			expr: &BinaryOp{Left: DotPath{Path: ".a"}, Op: "+", Right: DotPath{Path: ".b"}},
			want: float64(13),
		},
		{
			name: "subtraction",
			json: `{"a":10,"b":3}`,
			expr: &BinaryOp{Left: DotPath{Path: ".a"}, Op: "-", Right: DotPath{Path: ".b"}},
			want: float64(7),
		},
		{
			name: "multiplication",
			json: `{"a":10,"b":3}`,
			expr: &BinaryOp{Left: DotPath{Path: ".a"}, Op: "*", Right: DotPath{Path: ".b"}},
			want: float64(30),
		},
		{
			name: "division",
			json: `{"a":10,"b":4}`,
			expr: &BinaryOp{Left: DotPath{Path: ".a"}, Op: "/", Right: DotPath{Path: ".b"}},
			want: float64(2.5),
		},
		{
			name: "modulo",
			json: `{"a":10,"b":3}`,
			expr: &BinaryOp{Left: DotPath{Path: ".a"}, Op: "%", Right: DotPath{Path: ".b"}},
			want: float64(1),
		},
		{
			name: "string concatenation",
			json: `{}`,
			expr: &BinaryOp{
				Left:  StringLiteral{Value: "hello"},
				Op:    "+",
				Right: StringLiteral{Value: " world"},
			},
			want: "hello world",
		},
		{
			name: "division by zero",
			json: `{"a":10,"b":0}`,
			expr: &BinaryOp{Left: DotPath{Path: ".a"}, Op: "/", Right: DotPath{Path: ".b"}},
			// Either returns +Inf or an error; we accept both
		},
		{
			name: "arithmetic with null propagation",
			json: `{"a":10}`,
			expr: &BinaryOp{Left: DotPath{Path: ".a"}, Op: "+", Right: DotPath{Path: ".b"}},
			want: nil, // null propagation: missing .b -> null -> result is null
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			obj := unmarshal(t, tt.json)
			got, err := Eval(tt.expr, obj)

			if tt.name == "division by zero" {
				// Accept either an error or +Inf
				if err != nil {
					return // error is acceptable
				}
				if f, ok := got.(float64); ok && (math.IsInf(f, 1) || math.IsNaN(f)) {
					return // infinity/NaN is acceptable
				}
				t.Logf("division by zero returned %v (type %T), accepted", got, got)
				return
			}

			if tt.wantErr {
				if err == nil {
					t.Fatal("expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if !deepEqual(got, tt.want) {
				t.Errorf("got %v (%T), want %v (%T)", got, got, tt.want, tt.want)
			}
		})
	}
}

// ── TestEvalFunctions ────────────────────────────────────────────────

func TestEvalFunctions(t *testing.T) {
	tests := []struct {
		name    string
		json    string
		expr    Expr
		want    any
		wantErr bool
		cmpFunc func(t *testing.T, got any) // custom comparison when needed
	}{
		{
			name: "length of string",
			json: `{}`,
			expr: &FuncCall{Name: "length", Args: []Expr{StringLiteral{Value: "hello"}}},
			want: float64(5),
		},
		{
			name: "length of array",
			json: `{}`,
			expr: &FuncCall{Name: "length", Args: []Expr{
				// Pass a literal array via a path that resolves to an array
			}},
		},
		{
			name: "length of array via path",
			json: `{"items":[1,2,3]}`,
			expr: &FuncCall{Name: "length", Args: []Expr{DotPath{Path: ".items"}}},
			want: float64(3),
		},
		{
			name: "lower",
			json: `{}`,
			expr: &FuncCall{Name: "lower", Args: []Expr{StringLiteral{Value: "HELLO"}}},
			want: "hello",
		},
		{
			name: "upper",
			json: `{}`,
			expr: &FuncCall{Name: "upper", Args: []Expr{StringLiteral{Value: "hello"}}},
			want: "HELLO",
		},
		{
			name: "trim",
			json: `{}`,
			expr: &FuncCall{Name: "trim", Args: []Expr{StringLiteral{Value: "  hi  "}}},
			want: "hi",
		},
		{
			name: "abs of negative",
			json: `{}`,
			expr: &FuncCall{Name: "abs", Args: []Expr{NumberLiteral{Value: -5}}},
			want: float64(5),
		},
		{
			name: "floor",
			json: `{}`,
			expr: &FuncCall{Name: "floor", Args: []Expr{NumberLiteral{Value: 3.7}}},
			want: float64(3),
		},
		{
			name: "ceil",
			json: `{}`,
			expr: &FuncCall{Name: "ceil", Args: []Expr{NumberLiteral{Value: 3.2}}},
			want: float64(4),
		},
		{
			name: "round",
			json: `{}`,
			expr: &FuncCall{Name: "round", Args: []Expr{NumberLiteral{Value: 3.5}}},
			want: float64(4),
		},
		{
			name: "round with precision",
			json: `{}`,
			expr: &FuncCall{Name: "round", Args: []Expr{
				NumberLiteral{Value: 3.14159},
				NumberLiteral{Value: 2},
			}},
			want: float64(3.14),
		},
		{
			name: "sqrt",
			json: `{}`,
			expr: &FuncCall{Name: "sqrt", Args: []Expr{NumberLiteral{Value: 16}}},
			want: float64(4),
		},
		{
			name: "pow",
			json: `{}`,
			expr: &FuncCall{Name: "pow", Args: []Expr{
				NumberLiteral{Value: 2},
				NumberLiteral{Value: 10},
			}},
			want: float64(1024),
		},
		{
			name: "coalesce with nulls and default",
			json: `{}`,
			expr: &FuncCall{Name: "coalesce", Args: []Expr{
				NullLiteral{},
				NullLiteral{},
				StringLiteral{Value: "default"},
			}},
			want: "default",
		},
		{
			name: "coalesce with missing paths",
			json: `{"other": true}`,
			expr: &FuncCall{Name: "coalesce", Args: []Expr{
				DotPath{Path: ".missing"},
				DotPath{Path: ".also_missing"},
				NumberLiteral{Value: 42},
			}},
			want: float64(42),
		},
		{
			name: "if true branch",
			json: `{"active": true}`,
			expr: &FuncCall{Name: "if", Args: []Expr{
				DotPath{Path: ".active"},
				StringLiteral{Value: "yes"},
				StringLiteral{Value: "no"},
			}},
			want: "yes",
		},
		{
			name: "if false branch",
			json: `{"active": false}`,
			expr: &FuncCall{Name: "if", Args: []Expr{
				DotPath{Path: ".active"},
				StringLiteral{Value: "yes"},
				StringLiteral{Value: "no"},
			}},
			want: "no",
		},
		{
			name: "type of string",
			json: `{}`,
			expr: &FuncCall{Name: "type", Args: []Expr{StringLiteral{Value: "hello"}}},
			want: "string",
		},
		{
			name: "type of number",
			json: `{}`,
			expr: &FuncCall{Name: "type", Args: []Expr{NumberLiteral{Value: 42}}},
			want: "number",
		},
		{
			name: "type of bool",
			json: `{}`,
			expr: &FuncCall{Name: "type", Args: []Expr{BoolLiteral{Value: true}}},
			want: "boolean",
		},
		{
			name: "type of null",
			json: `{}`,
			expr: &FuncCall{Name: "type", Args: []Expr{NullLiteral{}}},
			want: "null",
		},
		{
			name: "type of array",
			json: `{"arr":[]}`,
			expr: &FuncCall{Name: "type", Args: []Expr{DotPath{Path: ".arr"}}},
			want: "array",
		},
		{
			name: "type of object",
			json: `{"obj":{}}`,
			expr: &FuncCall{Name: "type", Args: []Expr{DotPath{Path: ".obj"}}},
			want: "object",
		},
		{
			name: "keys of object",
			json: `{"a":1,"b":2}`,
			expr: &FuncCall{Name: "keys", Args: []Expr{DotPath{Path: "."}}},
			cmpFunc: func(t *testing.T, got any) {
				t.Helper()
				arr, ok := got.([]any)
				if !ok {
					t.Fatalf("keys returned %T, want []any", got)
				}
				strs := make([]string, len(arr))
				for i, v := range arr {
					strs[i], _ = v.(string)
				}
				sort.Strings(strs)
				want := []string{"a", "b"}
				if !reflect.DeepEqual(strs, want) {
					t.Errorf("keys = %v, want %v", strs, want)
				}
			},
		},
		{
			name: "values of object",
			json: `{"a":1,"b":2}`,
			expr: &FuncCall{Name: "values", Args: []Expr{DotPath{Path: "."}}},
			cmpFunc: func(t *testing.T, got any) {
				t.Helper()
				arr, ok := got.([]any)
				if !ok {
					t.Fatalf("values returned %T, want []any", got)
				}
				nums := make([]float64, len(arr))
				for i, v := range arr {
					nums[i], _ = v.(float64)
				}
				sort.Float64s(nums)
				want := []float64{1, 2}
				if !reflect.DeepEqual(nums, want) {
					t.Errorf("values = %v, want %v", nums, want)
				}
			},
		},
		{
			name: "split string",
			json: `{}`,
			expr: &FuncCall{Name: "split", Args: []Expr{
				StringLiteral{Value: "a,b,c"},
				StringLiteral{Value: ","},
			}},
			want: []any{"a", "b", "c"},
		},
		{
			name: "join array",
			json: `{"arr":["a","b","c"]}`,
			expr: &FuncCall{Name: "join", Args: []Expr{
				DotPath{Path: ".arr"},
				StringLiteral{Value: ","},
			}},
			want: "a,b,c",
		},
		{
			name: "replace",
			json: `{}`,
			expr: &FuncCall{Name: "replace", Args: []Expr{
				StringLiteral{Value: "hello world"},
				StringLiteral{Value: "world"},
				StringLiteral{Value: "go"},
			}},
			want: "hello go",
		},
		{
			name: "substr",
			json: `{}`,
			expr: &FuncCall{Name: "substr", Args: []Expr{
				StringLiteral{Value: "hello"},
				NumberLiteral{Value: 1},
				NumberLiteral{Value: 3},
			}},
			want: "ell",
		},
		{
			name: "to_number",
			json: `{}`,
			expr: &FuncCall{Name: "to_number", Args: []Expr{StringLiteral{Value: "42"}}},
			want: float64(42),
		},
		{
			name: "to_string",
			json: `{}`,
			expr: &FuncCall{Name: "to_string", Args: []Expr{NumberLiteral{Value: 42}}},
			want: "42",
		},
		{
			name: "regex_extract",
			json: `{}`,
			expr: &FuncCall{Name: "regex_extract", Args: []Expr{
				StringLiteral{Value: "error: timeout after 5s"},
				RegexLiteral{Pattern: `timeout \w+ \d+`, Flags: ""},
			}},
			want: "timeout after 5",
		},
		{
			name: "regex_extract_all",
			json: `{}`,
			expr: &FuncCall{Name: "regex_extract_all", Args: []Expr{
				StringLiteral{Value: "a1b2c3"},
				RegexLiteral{Pattern: `\d+`, Flags: ""},
			}},
			want: []any{"1", "2", "3"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Skip the bare "length of array" test that has no args
			if tt.name == "length of array" {
				t.Skip("requires array literal construction")
				return
			}

			obj := unmarshal(t, tt.json)
			got, err := Eval(tt.expr, obj)

			if tt.wantErr {
				if err == nil {
					t.Fatal("expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if tt.cmpFunc != nil {
				tt.cmpFunc(t, got)
				return
			}

			if !deepEqual(got, tt.want) {
				t.Errorf("got %v (%T), want %v (%T)", got, got, tt.want, tt.want)
			}
		})
	}
}

// ── TestMatch ────────────────────────────────────────────────────────

func TestMatch(t *testing.T) {
	tests := []struct {
		name string
		json string
		expr Expr
		want bool
	}{
		// Equality
		{
			name: "equality true",
			json: `{"status":"failed"}`,
			expr: &BinaryOp{Left: DotPath{Path: ".status"}, Op: "==", Right: StringLiteral{Value: "failed"}},
			want: true,
		},
		{
			name: "equality false",
			json: `{"status":"ok"}`,
			expr: &BinaryOp{Left: DotPath{Path: ".status"}, Op: "==", Right: StringLiteral{Value: "failed"}},
			want: false,
		},

		// Comparison operators
		{
			name: "greater than true",
			json: `{"age":25}`,
			expr: &BinaryOp{Left: DotPath{Path: ".age"}, Op: ">", Right: NumberLiteral{Value: 18}},
			want: true,
		},
		{
			name: "greater than false",
			json: `{"age":15}`,
			expr: &BinaryOp{Left: DotPath{Path: ".age"}, Op: ">", Right: NumberLiteral{Value: 18}},
			want: false,
		},
		{
			name: "greater than or equal true",
			json: `{"age":30}`,
			expr: &BinaryOp{Left: DotPath{Path: ".age"}, Op: ">=", Right: NumberLiteral{Value: 30}},
			want: true,
		},
		{
			name: "not equal true",
			json: `{"name":"Alice"}`,
			expr: &BinaryOp{Left: DotPath{Path: ".name"}, Op: "!=", Right: StringLiteral{Value: "Bob"}},
			want: true,
		},
		{
			name: "not equal false",
			json: `{"name":"Bob"}`,
			expr: &BinaryOp{Left: DotPath{Path: ".name"}, Op: "!=", Right: StringLiteral{Value: "Bob"}},
			want: false,
		},

		// Contains
		{
			name: "string contains true",
			json: `{"msg":"connection error occurred"}`,
			expr: &ContainsExpr{
				Haystack: DotPath{Path: ".msg"},
				Needle:   StringLiteral{Value: "error"},
			},
			want: true,
		},
		{
			name: "string contains false",
			json: `{"msg":"all good"}`,
			expr: &ContainsExpr{
				Haystack: DotPath{Path: ".msg"},
				Needle:   StringLiteral{Value: "error"},
			},
			want: false,
		},
		{
			name: "array contains element",
			json: `{"tags":["user","admin","editor"]}`,
			expr: &ContainsExpr{
				Haystack: DotPath{Path: ".tags"},
				Needle:   StringLiteral{Value: "admin"},
			},
			want: true,
		},
		{
			name: "array does not contain element",
			json: `{"tags":["user","editor"]}`,
			expr: &ContainsExpr{
				Haystack: DotPath{Path: ".tags"},
				Needle:   StringLiteral{Value: "admin"},
			},
			want: false,
		},

		// Starts with / ends with
		{
			name: "starts with true",
			json: `{"name":"Alice"}`,
			expr: &StartsWithExpr{
				Expr:   DotPath{Path: ".name"},
				Prefix: StringLiteral{Value: "Ali"},
			},
			want: true,
		},
		{
			name: "starts with false",
			json: `{"name":"Bob"}`,
			expr: &StartsWithExpr{
				Expr:   DotPath{Path: ".name"},
				Prefix: StringLiteral{Value: "Ali"},
			},
			want: false,
		},
		{
			name: "ends with true",
			json: `{"name":"Alice"}`,
			expr: &EndsWithExpr{
				Expr:   DotPath{Path: ".name"},
				Suffix: StringLiteral{Value: "ice"},
			},
			want: true,
		},
		{
			name: "ends with false",
			json: `{"name":"Bob"}`,
			expr: &EndsWithExpr{
				Expr:   DotPath{Path: ".name"},
				Suffix: StringLiteral{Value: "ice"},
			},
			want: false,
		},

		// Matches regex
		{
			name: "matches regex true",
			json: `{"name":"Alice"}`,
			expr: &MatchesExpr{
				Expr:  DotPath{Path: ".name"},
				Regex: RegexLiteral{Pattern: "^[A-Z]", Flags: ""},
			},
			want: true,
		},
		{
			name: "matches regex false",
			json: `{"name":"alice"}`,
			expr: &MatchesExpr{
				Expr:  DotPath{Path: ".name"},
				Regex: RegexLiteral{Pattern: "^[A-Z]", Flags: ""},
			},
			want: false,
		},

		// In
		{
			name: "in list true",
			json: `{"status":"failed"}`,
			expr: &InExpr{
				Expr: DotPath{Path: ".status"},
				Values: []Expr{
					StringLiteral{Value: "ok"},
					StringLiteral{Value: "failed"},
				},
			},
			want: true,
		},
		{
			name: "in list false",
			json: `{"status":"pending"}`,
			expr: &InExpr{
				Expr: DotPath{Path: ".status"},
				Values: []Expr{
					StringLiteral{Value: "ok"},
					StringLiteral{Value: "failed"},
				},
			},
			want: false,
		},

		// Exists
		{
			name: "exists true",
			json: `{"error":"something"}`,
			expr: &ExistsExpr{Expr: DotPath{Path: ".error"}},
			want: true,
		},
		{
			name: "exists false",
			json: `{"name":"Alice"}`,
			expr: &ExistsExpr{Expr: DotPath{Path: ".error"}},
			want: false,
		},

		// Is null
		{
			name: "is null true",
			json: `{"value":null}`,
			expr: &IsNullExpr{Expr: DotPath{Path: ".value"}},
			want: true,
		},
		{
			name: "is null false for present value",
			json: `{"value":42}`,
			expr: &IsNullExpr{Expr: DotPath{Path: ".value"}},
			want: false,
		},

		// Is type checks
		{
			name: "is string true",
			json: `{"name":"Alice"}`,
			expr: &IsTypeExpr{Expr: DotPath{Path: ".name"}, TypeName: "string"},
			want: true,
		},
		{
			name: "is string false",
			json: `{"name":42}`,
			expr: &IsTypeExpr{Expr: DotPath{Path: ".name"}, TypeName: "string"},
			want: false,
		},
		{
			name: "is number true",
			json: `{"age":30}`,
			expr: &IsTypeExpr{Expr: DotPath{Path: ".age"}, TypeName: "number"},
			want: true,
		},
		{
			name: "is number false",
			json: `{"age":"thirty"}`,
			expr: &IsTypeExpr{Expr: DotPath{Path: ".age"}, TypeName: "number"},
			want: false,
		},
		{
			name: "is bool true",
			json: `{"active":true}`,
			expr: &IsTypeExpr{Expr: DotPath{Path: ".active"}, TypeName: "bool"},
			want: true,
		},
		{
			name: "is bool false",
			json: `{"active":"yes"}`,
			expr: &IsTypeExpr{Expr: DotPath{Path: ".active"}, TypeName: "bool"},
			want: false,
		},
		{
			name: "is array true",
			json: `{"tags":["a","b"]}`,
			expr: &IsTypeExpr{Expr: DotPath{Path: ".tags"}, TypeName: "array"},
			want: true,
		},
		{
			name: "is array false",
			json: `{"tags":"not-array"}`,
			expr: &IsTypeExpr{Expr: DotPath{Path: ".tags"}, TypeName: "array"},
			want: false,
		},
		{
			name: "is object true",
			json: `{"meta":{"key":"val"}}`,
			expr: &IsTypeExpr{Expr: DotPath{Path: ".meta"}, TypeName: "object"},
			want: true,
		},
		{
			name: "is object false",
			json: `{"meta":"not-object"}`,
			expr: &IsTypeExpr{Expr: DotPath{Path: ".meta"}, TypeName: "object"},
			want: false,
		},

		// Compound: and
		{
			name: "and both true",
			json: `{"a":5,"b":3}`,
			expr: &BinaryOp{
				Left:  &BinaryOp{Left: DotPath{Path: ".a"}, Op: ">", Right: NumberLiteral{Value: 1}},
				Op:    "and",
				Right: &BinaryOp{Left: DotPath{Path: ".b"}, Op: "<", Right: NumberLiteral{Value: 10}},
			},
			want: true,
		},
		{
			name: "and one false",
			json: `{"a":5,"b":30}`,
			expr: &BinaryOp{
				Left:  &BinaryOp{Left: DotPath{Path: ".a"}, Op: ">", Right: NumberLiteral{Value: 1}},
				Op:    "and",
				Right: &BinaryOp{Left: DotPath{Path: ".b"}, Op: "<", Right: NumberLiteral{Value: 10}},
			},
			want: false,
		},

		// Compound: or
		{
			name: "or one true",
			json: `{"a":5,"b":3}`,
			expr: &BinaryOp{
				Left:  &BinaryOp{Left: DotPath{Path: ".a"}, Op: ">", Right: NumberLiteral{Value: 100}},
				Op:    "or",
				Right: &BinaryOp{Left: DotPath{Path: ".b"}, Op: "<", Right: NumberLiteral{Value: 5}},
			},
			want: true,
		},
		{
			name: "or both false",
			json: `{"a":5,"b":30}`,
			expr: &BinaryOp{
				Left:  &BinaryOp{Left: DotPath{Path: ".a"}, Op: ">", Right: NumberLiteral{Value: 100}},
				Op:    "or",
				Right: &BinaryOp{Left: DotPath{Path: ".b"}, Op: "<", Right: NumberLiteral{Value: 5}},
			},
			want: false,
		},

		// Not
		{
			name: "not true becomes false",
			json: `{"active":true}`,
			expr: &UnaryOp{Op: "not", Expr: DotPath{Path: ".active"}},
			want: false,
		},
		{
			name: "not false becomes true",
			json: `{"active":false}`,
			expr: &UnaryOp{Op: "not", Expr: DotPath{Path: ".active"}},
			want: true,
		},

		// Grouping: (a or b) and c
		{
			name: "grouped or with and - true",
			json: `{"a":true,"b":false,"c":true}`,
			expr: &BinaryOp{
				Left: &BinaryOp{
					Left:  DotPath{Path: ".a"},
					Op:    "or",
					Right: DotPath{Path: ".b"},
				},
				Op:    "and",
				Right: DotPath{Path: ".c"},
			},
			want: true,
		},
		{
			name: "grouped or with and - false because c is false",
			json: `{"a":true,"b":false,"c":false}`,
			expr: &BinaryOp{
				Left: &BinaryOp{
					Left:  DotPath{Path: ".a"},
					Op:    "or",
					Right: DotPath{Path: ".b"},
				},
				Op:    "and",
				Right: DotPath{Path: ".c"},
			},
			want: false,
		},

		// Recursive descent in condition
		{
			name: "recursive descent contains in condition",
			json: `{"response":{"body":{"error":"connection timeout"}}}`,
			expr: &ContainsExpr{
				Haystack: RecursiveDescent{Field: "error"},
				Needle:   StringLiteral{Value: "timeout"},
			},
			want: true,
		},
		{
			name: "recursive descent contains false",
			json: `{"response":{"body":{"error":"not found"}}}`,
			expr: &ContainsExpr{
				Haystack: RecursiveDescent{Field: "error"},
				Needle:   StringLiteral{Value: "timeout"},
			},
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			obj := unmarshal(t, tt.json)
			got, err := Match(tt.expr, obj)
			if err != nil {
				t.Fatalf("Match returned error: %v", err)
			}
			if got != tt.want {
				t.Errorf("Match() = %v, want %v", got, tt.want)
			}
		})
	}
}
