package query

import (
	"fmt"
	"reflect"
	"strings"
	"testing"
)

// ── Helpers ───────────────────────────────────────────────────────────

func intPtr(v int) *int { return &v }

// astEqual performs a deep comparison of two AST nodes, returning a human-readable
// diff string if they differ or "" if equal.
func astEqual(got, want interface{}) string {
	return astEqualPath(got, want, "root")
}

func astEqualPath(got, want interface{}, path string) string {
	if got == nil && want == nil {
		return ""
	}
	if (got == nil) != (want == nil) {
		return fmt.Sprintf("%s: got %v, want %v", path, got, want)
	}

	gv := reflect.ValueOf(got)
	wv := reflect.ValueOf(want)

	// Dereference pointers.
	for gv.Kind() == reflect.Ptr {
		if gv.IsNil() && wv.IsNil() {
			return ""
		}
		if gv.IsNil() != wv.IsNil() {
			return fmt.Sprintf("%s: got nil=%v, want nil=%v", path, gv.IsNil(), wv.IsNil())
		}
		gv = gv.Elem()
		wv = wv.Elem()
	}

	if gv.Type() != wv.Type() {
		return fmt.Sprintf("%s: type mismatch: got %T, want %T", path, got, want)
	}

	switch gv.Kind() {
	case reflect.Struct:
		for i := 0; i < gv.NumField(); i++ {
			fieldName := gv.Type().Field(i).Name
			sub := astEqualPath(gv.Field(i).Interface(), wv.Field(i).Interface(), path+"."+fieldName)
			if sub != "" {
				return sub
			}
		}
	case reflect.Slice:
		if gv.Len() != wv.Len() {
			return fmt.Sprintf("%s: slice len got %d, want %d", path, gv.Len(), wv.Len())
		}
		for i := 0; i < gv.Len(); i++ {
			sub := astEqualPath(gv.Index(i).Interface(), wv.Index(i).Interface(), fmt.Sprintf("%s[%d]", path, i))
			if sub != "" {
				return sub
			}
		}
	case reflect.Interface:
		if gv.IsNil() && wv.IsNil() {
			return ""
		}
		if gv.IsNil() != wv.IsNil() {
			return fmt.Sprintf("%s: interface nil mismatch", path)
		}
		return astEqualPath(gv.Elem().Interface(), wv.Elem().Interface(), path)
	default:
		if !reflect.DeepEqual(gv.Interface(), wv.Interface()) {
			return fmt.Sprintf("%s: got %v, want %v", path, gv.Interface(), wv.Interface())
		}
	}
	return ""
}

func requireNoError(t *testing.T, err error) {
	t.Helper()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func requireError(t *testing.T, err error) {
	t.Helper()
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

func requireErrorContains(t *testing.T, err error, substr string) {
	t.Helper()
	if err == nil {
		t.Fatalf("expected error containing %q, got nil", substr)
	}
	if !strings.Contains(err.Error(), substr) {
		t.Fatalf("expected error containing %q, got: %v", substr, err)
	}
}

func assertAST(t *testing.T, got, want *Query) {
	t.Helper()
	if diff := astEqual(got, want); diff != "" {
		t.Fatalf("AST mismatch:\n%s", diff)
	}
}

// ── Tests ─────────────────────────────────────────────────────────────

func TestParse(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  *Query
	}{
		// 1. Bare dot-path
		{
			name:  "bare dot-path selects one field",
			input: ".name",
			want: &Query{
				Select: &SelectClause{
					Fields: []SelectField{
						{Expr: DotPath{Path: ".name"}},
					},
				},
			},
		},

		// 2. Multiple fields
		{
			name:  "multiple bare fields",
			input: ".name, .age",
			want: &Query{
				Select: &SelectClause{
					Fields: []SelectField{
						{Expr: DotPath{Path: ".name"}},
						{Expr: DotPath{Path: ".age"}},
					},
				},
			},
		},

		// 3. Select with aliases
		{
			name:  "select with alias",
			input: "select .error.message as err",
			want: &Query{
				Select: &SelectClause{
					Fields: []SelectField{
						{Expr: DotPath{Path: ".error.message"}, Alias: "err"},
					},
				},
			},
		},

		// 4. Computed fields
		{
			name:  "select computed field with alias",
			input: "select .end - .start as duration",
			want: &Query{
				Select: &SelectClause{
					Fields: []SelectField{
						{
							Expr: BinaryOp{
								Left:  DotPath{Path: ".end"},
								Op:    "-",
								Right: DotPath{Path: ".start"},
							},
							Alias: "duration",
						},
					},
				},
			},
		},

		// 5. String templates
		{
			name:  "select string template with alias",
			input: `select "\(.first) \(.last)" as full_name`,
			want: &Query{
				Select: &SelectClause{
					Fields: []SelectField{
						{
							Expr: StringTemplate{
								Parts: []Expr{
									DotPath{Path: ".first"},
									StringLiteral{Value: " "},
									DotPath{Path: ".last"},
								},
							},
							Alias: "full_name",
						},
					},
				},
			},
		},

		// 6. Wildcards
		{
			name:  "select wildcard on field",
			input: "select .metadata.*",
			want: &Query{
				Select: &SelectClause{
					Fields: []SelectField{
						{Expr: WildcardExpr{Prefix: ".metadata"}},
					},
				},
			},
		},
		{
			name:  "select bare wildcard",
			input: "select *",
			want: &Query{
				Select: &SelectClause{
					Fields: []SelectField{
						{Expr: WildcardExpr{Prefix: ""}},
					},
				},
			},
		},

		// 7. Where simple
		{
			name:  "where equality string",
			input: `where .status == "failed"`,
			want: &Query{
				Where: &WhereClause{
					Condition: BinaryOp{
						Left:  DotPath{Path: ".status"},
						Op:    "==",
						Right: StringLiteral{Value: "failed"},
					},
				},
			},
		},

		// 8. Where with all operators
		{
			name:  "where ==",
			input: `where .a == 1`,
			want: &Query{
				Where: &WhereClause{
					Condition: BinaryOp{Left: DotPath{Path: ".a"}, Op: "==", Right: NumberLiteral{Value: 1}},
				},
			},
		},
		{
			name:  "where !=",
			input: `where .a != 1`,
			want: &Query{
				Where: &WhereClause{
					Condition: BinaryOp{Left: DotPath{Path: ".a"}, Op: "!=", Right: NumberLiteral{Value: 1}},
				},
			},
		},
		{
			name:  "where >",
			input: `where .a > 1`,
			want: &Query{
				Where: &WhereClause{
					Condition: BinaryOp{Left: DotPath{Path: ".a"}, Op: ">", Right: NumberLiteral{Value: 1}},
				},
			},
		},
		{
			name:  "where <",
			input: `where .a < 1`,
			want: &Query{
				Where: &WhereClause{
					Condition: BinaryOp{Left: DotPath{Path: ".a"}, Op: "<", Right: NumberLiteral{Value: 1}},
				},
			},
		},
		{
			name:  "where >=",
			input: `where .a >= 1`,
			want: &Query{
				Where: &WhereClause{
					Condition: BinaryOp{Left: DotPath{Path: ".a"}, Op: ">=", Right: NumberLiteral{Value: 1}},
				},
			},
		},
		{
			name:  "where <=",
			input: `where .a <= 1`,
			want: &Query{
				Where: &WhereClause{
					Condition: BinaryOp{Left: DotPath{Path: ".a"}, Op: "<=", Right: NumberLiteral{Value: 1}},
				},
			},
		},
		{
			name:  "where contains",
			input: `where .msg contains "error"`,
			want: &Query{
				Where: &WhereClause{
					Condition: ContainsExpr{
						Haystack: DotPath{Path: ".msg"},
						Needle:   StringLiteral{Value: "error"},
					},
				},
			},
		},
		{
			name:  "where starts with",
			input: `where .path starts with "/api"`,
			want: &Query{
				Where: &WhereClause{
					Condition: StartsWithExpr{
						Expr:   DotPath{Path: ".path"},
						Prefix: StringLiteral{Value: "/api"},
					},
				},
			},
		},
		{
			name:  "where ends with",
			input: `where .name ends with ".json"`,
			want: &Query{
				Where: &WhereClause{
					Condition: EndsWithExpr{
						Expr:   DotPath{Path: ".name"},
						Suffix: StringLiteral{Value: ".json"},
					},
				},
			},
		},
		{
			name:  "where matches regex",
			input: `where .msg matches /time.*out/i`,
			want: &Query{
				Where: &WhereClause{
					Condition: MatchesExpr{
						Expr:  DotPath{Path: ".msg"},
						Regex: RegexLiteral{Pattern: "time.*out", Flags: "i"},
					},
				},
			},
		},
		{
			name:  "where in list",
			input: `where .model in ("gpt-4", "claude-3")`,
			want: &Query{
				Where: &WhereClause{
					Condition: InExpr{
						Expr: DotPath{Path: ".model"},
						Values: []Expr{
							StringLiteral{Value: "gpt-4"},
							StringLiteral{Value: "claude-3"},
						},
					},
				},
			},
		},
		{
			name:  "where exists",
			input: `where .error exists`,
			want: &Query{
				Where: &WhereClause{
					Condition: ExistsExpr{Expr: DotPath{Path: ".error"}},
				},
			},
		},
		{
			name:  "where is null",
			input: `where .error is null`,
			want: &Query{
				Where: &WhereClause{
					Condition: IsNullExpr{Expr: DotPath{Path: ".error"}},
				},
			},
		},
		{
			name:  "where is type",
			input: `where .value is string`,
			want: &Query{
				Where: &WhereClause{
					Condition: IsTypeExpr{Expr: DotPath{Path: ".value"}, TypeName: "string"},
				},
			},
		},

		// 9. Where compound
		{
			name:  "where and",
			input: `where .a > 1 and .b < 2`,
			want: &Query{
				Where: &WhereClause{
					Condition: BinaryOp{
						Left:  BinaryOp{Left: DotPath{Path: ".a"}, Op: ">", Right: NumberLiteral{Value: 1}},
						Op:    "and",
						Right: BinaryOp{Left: DotPath{Path: ".b"}, Op: "<", Right: NumberLiteral{Value: 2}},
					},
				},
			},
		},
		{
			name:  "where or with parentheses",
			input: `where (.a or .b) and .c`,
			want: &Query{
				Where: &WhereClause{
					Condition: BinaryOp{
						Left: BinaryOp{
							Left:  DotPath{Path: ".a"},
							Op:    "or",
							Right: DotPath{Path: ".b"},
						},
						Op:    "and",
						Right: DotPath{Path: ".c"},
					},
				},
			},
		},

		// 10. Where not
		{
			name:  "where not",
			input: `where not .active`,
			want: &Query{
				Where: &WhereClause{
					Condition: UnaryOp{Op: "not", Expr: DotPath{Path: ".active"}},
				},
			},
		},

		// 11. Where recursive descent
		{
			name:  "where recursive descent contains",
			input: `where ..error contains "timeout"`,
			want: &Query{
				Where: &WhereClause{
					Condition: ContainsExpr{
						Haystack: RecursiveDescent{Field: "error"},
						Needle:   StringLiteral{Value: "timeout"},
					},
				},
			},
		},

		// 12. Sort by
		{
			name:  "sort by single field ascending",
			input: `sort by .latency_ms`,
			want: &Query{
				SortBy: &SortByClause{
					Fields: []SortField{
						{Expr: DotPath{Path: ".latency_ms"}, Desc: false},
					},
				},
			},
		},
		{
			name:  "sort by single field descending",
			input: `sort by .name desc`,
			want: &Query{
				SortBy: &SortByClause{
					Fields: []SortField{
						{Expr: DotPath{Path: ".name"}, Desc: true},
					},
				},
			},
		},
		{
			name:  "sort by multiple fields mixed order",
			input: `sort by .status, .latency_ms desc`,
			want: &Query{
				SortBy: &SortByClause{
					Fields: []SortField{
						{Expr: DotPath{Path: ".status"}, Desc: false},
						{Expr: DotPath{Path: ".latency_ms"}, Desc: true},
					},
				},
			},
		},

		// 13. Group by
		{
			name:  "group by field",
			input: `group by .model`,
			want: &Query{
				GroupBy: &GroupByClause{
					Expr: DotPath{Path: ".model"},
				},
			},
		},

		// 14. Count
		{
			name:  "plain count",
			input: `count`,
			want: &Query{
				Count: &CountClause{By: nil},
			},
		},
		{
			name:  "count by field",
			input: `count by .model`,
			want: &Query{
				Count: &CountClause{By: DotPath{Path: ".model"}},
			},
		},

		// 15. First / last
		{
			name:  "first N",
			input: `first 10`,
			want: &Query{
				Limit: &LimitClause{N: 10, Offset: 0, IsLast: false},
			},
		},
		{
			name:  "last N",
			input: `last 5`,
			want: &Query{
				Limit: &LimitClause{N: 5, Offset: 0, IsLast: true},
			},
		},

		// 16. Limit / offset
		{
			name:  "limit with offset",
			input: `limit 20 offset 100`,
			want: &Query{
				Limit: &LimitClause{N: 20, Offset: 100, IsLast: false},
			},
		},
		{
			name:  "limit without offset",
			input: `limit 20`,
			want: &Query{
				Limit: &LimitClause{N: 20, Offset: 0, IsLast: false},
			},
		},

		// 17. Distinct
		{
			name:  "distinct field",
			input: `distinct .model`,
			want: &Query{
				Distinct: &DistinctClause{
					Expr: DotPath{Path: ".model"},
				},
			},
		},

		// 18. Combined query
		{
			name:  "select where sort first combined",
			input: `select .id, .model where .status == "failed" sort by .latency_ms desc first 10`,
			want: &Query{
				Select: &SelectClause{
					Fields: []SelectField{
						{Expr: DotPath{Path: ".id"}},
						{Expr: DotPath{Path: ".model"}},
					},
				},
				Where: &WhereClause{
					Condition: BinaryOp{
						Left:  DotPath{Path: ".status"},
						Op:    "==",
						Right: StringLiteral{Value: "failed"},
					},
				},
				SortBy: &SortByClause{
					Fields: []SortField{
						{Expr: DotPath{Path: ".latency_ms"}, Desc: true},
					},
				},
				Limit: &LimitClause{N: 10, Offset: 0, IsLast: false},
			},
		},

		// 19. Aggregate functions
		{
			name:  "select with aggregate functions and group by",
			input: `select .model, count(), avg(.latency_ms) group by .model`,
			want: &Query{
				Select: &SelectClause{
					Fields: []SelectField{
						{Expr: DotPath{Path: ".model"}},
						{Expr: FuncCall{Name: "count", Args: nil}},
						{Expr: FuncCall{Name: "avg", Args: []Expr{DotPath{Path: ".latency_ms"}}}},
					},
				},
				GroupBy: &GroupByClause{
					Expr: DotPath{Path: ".model"},
				},
			},
		},

		// 20. Scalar functions
		{
			name:  "select scalar functions",
			input: `select length(.name), lower(.email)`,
			want: &Query{
				Select: &SelectClause{
					Fields: []SelectField{
						{Expr: FuncCall{Name: "length", Args: []Expr{DotPath{Path: ".name"}}}},
						{Expr: FuncCall{Name: "lower", Args: []Expr{DotPath{Path: ".email"}}}},
					},
				},
			},
		},

		// 21. Arithmetic in where
		{
			name:  "where arithmetic expression",
			input: `where .price * .qty > 100`,
			want: &Query{
				Where: &WhereClause{
					Condition: BinaryOp{
						Left: BinaryOp{
							Left:  DotPath{Path: ".price"},
							Op:    "*",
							Right: DotPath{Path: ".qty"},
						},
						Op:    ">",
						Right: NumberLiteral{Value: 100},
					},
				},
			},
		},

		// 22. Array index and slice
		{
			name:  "array index access",
			input: `.items[0]`,
			want: &Query{
				Select: &SelectClause{
					Fields: []SelectField{
						{Expr: ArrayIndex{Expr: DotPath{Path: ".items"}, Index: 0}},
					},
				},
			},
		},
		{
			name:  "array slice with start and end",
			input: `.items[2:5]`,
			want: &Query{
				Select: &SelectClause{
					Fields: []SelectField{
						{Expr: ArraySlice{
							Expr:  DotPath{Path: ".items"},
							Start: intPtr(2),
							End:   intPtr(5),
						}},
					},
				},
			},
		},
		{
			name:  "array slice from negative index",
			input: `.items[-3:]`,
			want: &Query{
				Select: &SelectClause{
					Fields: []SelectField{
						{Expr: ArraySlice{
							Expr:  DotPath{Path: ".items"},
							Start: intPtr(-3),
							End:   nil,
						}},
					},
				},
			},
		},
		{
			name:  "array iterator",
			input: `.items[]`,
			want: &Query{
				Select: &SelectClause{
					Fields: []SelectField{
						{Expr: ArrayIterator{Expr: DotPath{Path: ".items"}}},
					},
				},
			},
		},

		// 23. Pipe to function
		{
			name:  "pipe dot-path to function",
			input: `.name | length`,
			want: &Query{
				Select: &SelectClause{
					Fields: []SelectField{
						{Expr: PipeExpr{
							Left:  DotPath{Path: ".name"},
							Right: FuncCall{Name: "length", Args: nil},
						}},
					},
				},
			},
		},

		// ── Additional structural tests ───────────────────────────────

		// Nested paths
		{
			name:  "deeply nested dot-path",
			input: `.response.body.data.items`,
			want: &Query{
				Select: &SelectClause{
					Fields: []SelectField{
						{Expr: DotPath{Path: ".response.body.data.items"}},
					},
				},
			},
		},

		// Boolean literals in where
		{
			name:  "where with boolean literal",
			input: `where .active == true`,
			want: &Query{
				Where: &WhereClause{
					Condition: BinaryOp{
						Left:  DotPath{Path: ".active"},
						Op:    "==",
						Right: BoolLiteral{Value: true},
					},
				},
			},
		},
		{
			name:  "where with false literal",
			input: `where .deleted == false`,
			want: &Query{
				Where: &WhereClause{
					Condition: BinaryOp{
						Left:  DotPath{Path: ".deleted"},
						Op:    "==",
						Right: BoolLiteral{Value: false},
					},
				},
			},
		},

		// Null literal in comparison
		{
			name:  "where equality with null",
			input: `where .error == null`,
			want: &Query{
				Where: &WhereClause{
					Condition: BinaryOp{
						Left:  DotPath{Path: ".error"},
						Op:    "==",
						Right: NullLiteral{},
					},
				},
			},
		},

		// Multiple select fields with aliases
		{
			name:  "select multiple fields with aliases",
			input: `select .first_name as first, .last_name as last, .age`,
			want: &Query{
				Select: &SelectClause{
					Fields: []SelectField{
						{Expr: DotPath{Path: ".first_name"}, Alias: "first"},
						{Expr: DotPath{Path: ".last_name"}, Alias: "last"},
						{Expr: DotPath{Path: ".age"}},
					},
				},
			},
		},

		// Where with multiple and/or
		{
			name:  "where chained and",
			input: `where .a > 1 and .b < 2 and .c == 3`,
			want: &Query{
				Where: &WhereClause{
					Condition: BinaryOp{
						Left: BinaryOp{
							Left:  BinaryOp{Left: DotPath{Path: ".a"}, Op: ">", Right: NumberLiteral{Value: 1}},
							Op:    "and",
							Right: BinaryOp{Left: DotPath{Path: ".b"}, Op: "<", Right: NumberLiteral{Value: 2}},
						},
						Op:    "and",
						Right: BinaryOp{Left: DotPath{Path: ".c"}, Op: "==", Right: NumberLiteral{Value: 3}},
					},
				},
			},
		},

		// In with numbers
		{
			name:  "where in with numbers",
			input: `where .status in (200, 201, 204)`,
			want: &Query{
				Where: &WhereClause{
					Condition: InExpr{
						Expr: DotPath{Path: ".status"},
						Values: []Expr{
							NumberLiteral{Value: 200},
							NumberLiteral{Value: 201},
							NumberLiteral{Value: 204},
						},
					},
				},
			},
		},

		// Regex without flags
		{
			name:  "where matches regex without flags",
			input: `where .path matches /^\/api/`,
			want: &Query{
				Where: &WhereClause{
					Condition: MatchesExpr{
						Expr:  DotPath{Path: ".path"},
						Regex: RegexLiteral{Pattern: `^\/api`, Flags: ""},
					},
				},
			},
		},

		// Function with multiple args
		{
			name:  "select function with multiple args",
			input: `select substr(.name, 0, 5)`,
			want: &Query{
				Select: &SelectClause{
					Fields: []SelectField{
						{Expr: FuncCall{
							Name: "substr",
							Args: []Expr{
								DotPath{Path: ".name"},
								NumberLiteral{Value: 0},
								NumberLiteral{Value: 5},
							},
						}},
					},
				},
			},
		},

		// Sort by with explicit asc
		{
			name:  "sort by explicit asc",
			input: `sort by .name asc`,
			want: &Query{
				SortBy: &SortByClause{
					Fields: []SortField{
						{Expr: DotPath{Path: ".name"}, Desc: false},
					},
				},
			},
		},

		// Where not with compound
		{
			name:  "where not with comparison",
			input: `where not .status == "active"`,
			want: &Query{
				Where: &WhereClause{
					Condition: UnaryOp{
						Op: "not",
						Expr: BinaryOp{
							Left:  DotPath{Path: ".status"},
							Op:    "==",
							Right: StringLiteral{Value: "active"},
						},
					},
				},
			},
		},

		// Combined with where, group by, and sort
		{
			name:  "select with group by and sort",
			input: `select .model, count() group by .model sort by .model`,
			want: &Query{
				Select: &SelectClause{
					Fields: []SelectField{
						{Expr: DotPath{Path: ".model"}},
						{Expr: FuncCall{Name: "count", Args: nil}},
					},
				},
				GroupBy: &GroupByClause{
					Expr: DotPath{Path: ".model"},
				},
				SortBy: &SortByClause{
					Fields: []SortField{
						{Expr: DotPath{Path: ".model"}, Desc: false},
					},
				},
			},
		},

		// Combined with where, distinct
		{
			name:  "distinct with where",
			input: `distinct .model where .status == "ok"`,
			want: &Query{
				Distinct: &DistinctClause{
					Expr: DotPath{Path: ".model"},
				},
				Where: &WhereClause{
					Condition: BinaryOp{
						Left:  DotPath{Path: ".status"},
						Op:    "==",
						Right: StringLiteral{Value: "ok"},
					},
				},
			},
		},

		// 24. Array index with field access (FieldAccess)
		{
			name:  "array index then field access",
			input: `.items[0].name`,
			want: &Query{
				Select: &SelectClause{
					Fields: []SelectField{
						{Expr: FieldAccess{
							Expr:  ArrayIndex{Expr: DotPath{Path: ".items"}, Index: 0},
							Field: ".name",
						}},
					},
				},
			},
		},
		{
			name:  "array index then nested field access",
			input: `.data[0].error.message`,
			want: &Query{
				Select: &SelectClause{
					Fields: []SelectField{
						{Expr: FieldAccess{
							Expr:  ArrayIndex{Expr: DotPath{Path: ".data"}, Index: 0},
							Field: ".error.message",
						}},
					},
				},
			},
		},
		{
			name:  "chained array index and field access",
			input: `.a[0].b[1].c`,
			want: &Query{
				Select: &SelectClause{
					Fields: []SelectField{
						{Expr: FieldAccess{
							Expr: ArrayIndex{
								Expr: FieldAccess{
									Expr:  ArrayIndex{Expr: DotPath{Path: ".a"}, Index: 0},
									Field: ".b",
								},
								Index: 1,
							},
							Field: ".c",
						}},
					},
				},
			},
		},
		{
			name:  "where with array index field access",
			input: `where .message.content[0].type == "tool_use"`,
			want: &Query{
				Where: &WhereClause{
					Condition: BinaryOp{
						Left: FieldAccess{
							Expr:  ArrayIndex{Expr: DotPath{Path: ".message.content"}, Index: 0},
							Field: ".type",
						},
						Op:    "==",
						Right: StringLiteral{Value: "tool_use"},
					},
				},
			},
		},
		{
			name:  "select and where with array index field access",
			input: `where .message.content[0].type == "tool_use" select .message.content[0].name`,
			want: &Query{
				Where: &WhereClause{
					Condition: BinaryOp{
						Left: FieldAccess{
							Expr:  ArrayIndex{Expr: DotPath{Path: ".message.content"}, Index: 0},
							Field: ".type",
						},
						Op:    "==",
						Right: StringLiteral{Value: "tool_use"},
					},
				},
				Select: &SelectClause{
					Fields: []SelectField{
						{Expr: FieldAccess{
							Expr:  ArrayIndex{Expr: DotPath{Path: ".message.content"}, Index: 0},
							Field: ".name",
						}},
					},
				},
			},
		},
		{
			name:  "array iterator then field access",
			input: `.items[].name`,
			want: &Query{
				Select: &SelectClause{
					Fields: []SelectField{
						{Expr: FieldAccess{
							Expr:  ArrayIterator{Expr: DotPath{Path: ".items"}},
							Field: ".name",
						}},
					},
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := Parse(tt.input)
			requireNoError(t, err)
			assertAST(t, got, tt.want)
		})
	}
}

// ── Error cases ───────────────────────────────────────────────────────

func TestParseErrors(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		wantErr string // substring to match in error message
	}{
		{
			name:    "empty query",
			input:   "",
			wantErr: "empty",
		},
		{
			name:    "invalid syntax bare operator",
			input:   "== > <",
			wantErr: "unexpected",
		},
		{
			name:    "unterminated string",
			input:   `where .name == "hello`,
			wantErr: "unterminated",
		},
		{
			name:    "missing operand after operator",
			input:   `where .a >`,
			wantErr: "expected",
		},
		{
			name:    "missing field after sort by",
			input:   `sort by`,
			wantErr: "expected",
		},
		{
			name:    "missing field after group by",
			input:   `group by`,
			wantErr: "expected",
		},
		{
			name:    "missing number after first",
			input:   `first`,
			wantErr: "expected",
		},
		{
			name:    "missing number after last",
			input:   `last`,
			wantErr: "expected",
		},
		{
			name:    "missing number after limit",
			input:   `limit`,
			wantErr: "expected",
		},
		{
			name:    "missing field after distinct",
			input:   `distinct`,
			wantErr: "expected",
		},
		{
			name:    "unmatched open paren",
			input:   `where (.a > 1`,
			wantErr: ")",
		},
		{
			name:    "unmatched close paren",
			input:   `where .a > 1)`,
			wantErr: "unexpected",
		},
		{
			name:    "unmatched open bracket",
			input:   `.items[0`,
			wantErr: "]",
		},
		{
			name:    "missing with after starts",
			input:   `where .name starts "foo"`,
			wantErr: "with",
		},
		{
			name:    "missing with after ends",
			input:   `where .name ends "foo"`,
			wantErr: "with",
		},
		{
			name:    "in without opening paren",
			input:   `where .model in "gpt-4"`,
			wantErr: "(",
		},
		{
			name:    "negative number after first",
			input:   `first -5`,
			wantErr: "positive",
		},
		{
			name:    "non-integer after limit",
			input:   `limit 3.5`,
			wantErr: "integer",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := Parse(tt.input)
			requireError(t, err)
			if !strings.Contains(strings.ToLower(err.Error()), strings.ToLower(tt.wantErr)) {
				t.Errorf("Parse(%q): error = %v, want error containing %q", tt.input, err, tt.wantErr)
			}
		})
	}
}

// ── Operator precedence tests ─────────────────────────────────────────

func TestParsePrecedence(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  *Query
	}{
		{
			name:  "multiplication before addition",
			input: `where .a + .b * .c > 10`,
			want: &Query{
				Where: &WhereClause{
					Condition: BinaryOp{
						Left: BinaryOp{
							Left: DotPath{Path: ".a"},
							Op:   "+",
							Right: BinaryOp{
								Left:  DotPath{Path: ".b"},
								Op:    "*",
								Right: DotPath{Path: ".c"},
							},
						},
						Op:    ">",
						Right: NumberLiteral{Value: 10},
					},
				},
			},
		},
		{
			name:  "comparison before and",
			input: `where .a > 1 and .b < 2`,
			want: &Query{
				Where: &WhereClause{
					Condition: BinaryOp{
						Left:  BinaryOp{Left: DotPath{Path: ".a"}, Op: ">", Right: NumberLiteral{Value: 1}},
						Op:    "and",
						Right: BinaryOp{Left: DotPath{Path: ".b"}, Op: "<", Right: NumberLiteral{Value: 2}},
					},
				},
			},
		},
		{
			name:  "and before or",
			input: `where .a or .b and .c`,
			want: &Query{
				Where: &WhereClause{
					Condition: BinaryOp{
						Left: DotPath{Path: ".a"},
						Op:   "or",
						Right: BinaryOp{
							Left:  DotPath{Path: ".b"},
							Op:    "and",
							Right: DotPath{Path: ".c"},
						},
					},
				},
			},
		},
		{
			name:  "parentheses override precedence",
			input: `where (.a or .b) and .c`,
			want: &Query{
				Where: &WhereClause{
					Condition: BinaryOp{
						Left: BinaryOp{
							Left:  DotPath{Path: ".a"},
							Op:    "or",
							Right: DotPath{Path: ".b"},
						},
						Op:    "and",
						Right: DotPath{Path: ".c"},
					},
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := Parse(tt.input)
			requireNoError(t, err)
			assertAST(t, got, tt.want)
		})
	}
}
