package query

// ── Query top-level structure ────────────────────────────────────────

type Query struct {
	Select   *SelectClause
	Where    *WhereClause
	SortBy   *SortByClause
	GroupBy  *GroupByClause
	Count    *CountClause
	Distinct *DistinctClause
	Limit    *LimitClause
}

type SelectClause struct {
	Fields []SelectField
}

type SelectField struct {
	Expr  Expr
	Alias string
}

type WhereClause struct {
	Condition Expr
}

type SortByClause struct {
	Fields []SortField
}

type SortField struct {
	Expr Expr
	Desc bool
}

type GroupByClause struct {
	Expr Expr
}

type CountClause struct {
	By Expr // nil for plain count
}

type DistinctClause struct {
	Expr Expr
}

type LimitClause struct {
	N      int
	Offset int
	IsLast bool
}

// ── Expression interface ─────────────────────────────────────────────

type Expr interface{ exprNode() }

// ── Path expressions ─────────────────────────────────────────────────

type DotPath struct{ Path string }
type RecursiveDescent struct{ Field string }

// ── Array expressions ────────────────────────────────────────────────

type ArrayIndex struct {
	Expr  Expr
	Index int
}

type ArraySlice struct {
	Expr       Expr
	Start, End *int
	Step       *int
}

type ArrayIterator struct{ Expr Expr }

// ── Literal expressions ──────────────────────────────────────────────

type StringLiteral struct{ Value string }
type NumberLiteral struct{ Value float64 }
type BoolLiteral struct{ Value bool }
type NullLiteral struct{}

type RegexLiteral struct {
	Pattern string
	Flags   string
}

type StringTemplate struct{ Parts []Expr }

// ── Operator expressions ─────────────────────────────────────────────

type BinaryOp struct {
	Left  Expr
	Op    string
	Right Expr
}

type UnaryOp struct {
	Op   string
	Expr Expr
}

// ── Function / wildcard / pipe ───────────────────────────────────────

type FuncCall struct {
	Name string
	Args []Expr
}

type WildcardExpr struct{ Prefix string }

type PipeExpr struct {
	Left  Expr
	Right Expr
}

// ── Special where expressions ────────────────────────────────────────

type ExistsExpr struct{ Expr Expr }
type IsNullExpr struct{ Expr Expr }

type IsTypeExpr struct {
	Expr     Expr
	TypeName string
}

type InExpr struct {
	Expr   Expr
	Values []Expr
}

type ContainsExpr struct {
	Haystack Expr
	Needle   Expr
}

type StartsWithExpr struct {
	Expr   Expr
	Prefix Expr
}

type EndsWithExpr struct {
	Expr   Expr
	Suffix Expr
}

type MatchesExpr struct {
	Expr  Expr
	Regex Expr
}

// ── exprNode marker methods ──────────────────────────────────────────

func (DotPath) exprNode()          {}
func (RecursiveDescent) exprNode() {}
func (ArrayIndex) exprNode()       {}
func (ArraySlice) exprNode()       {}
func (ArrayIterator) exprNode()    {}
func (StringLiteral) exprNode()    {}
func (NumberLiteral) exprNode()    {}
func (BoolLiteral) exprNode()      {}
func (NullLiteral) exprNode()      {}
func (RegexLiteral) exprNode()     {}
func (StringTemplate) exprNode()   {}
func (BinaryOp) exprNode()         {}
func (UnaryOp) exprNode()          {}
func (FuncCall) exprNode()         {}
func (WildcardExpr) exprNode()     {}
func (PipeExpr) exprNode()         {}
func (ExistsExpr) exprNode()       {}
func (IsNullExpr) exprNode()       {}
func (IsTypeExpr) exprNode()       {}
func (InExpr) exprNode()           {}
func (ContainsExpr) exprNode()     {}
func (StartsWithExpr) exprNode()   {}
func (EndsWithExpr) exprNode()     {}
func (MatchesExpr) exprNode()      {}
