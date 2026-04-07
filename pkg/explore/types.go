package explore

// SchemaField represents one field in the inferred schema.
type SchemaField struct {
	Path      string   // e.g. ".error.message"
	Types     []string // e.g. ["string", "null"]
	Frequency float64  // 0.0 to 1.0
	Examples  []any    // sample values
	Unique    int      // count of unique values (approx for large sets)
}

// TreeNode represents one node in the structure tree.
type TreeNode struct {
	Path     string
	Type     string // "string", "number", "object", "array[string]", etc.
	Optional bool
	Children []*TreeNode
	Unique   int // unique value count, 0 if not tracked
}

// Stats represents statistical summary.
type Stats struct {
	Count        int
	SchemaCount  int // distinct shapes
	Fields       int // unique field paths
	NumericStats map[string]NumericStat
	StringStats  map[string]StringStat
	NullCounts   map[string]int
}

// NumericStat holds numeric field statistics.
type NumericStat struct {
	Min, Max, Mean, Median float64
	P95, P99               float64
}

// StringStat holds string field statistics.
type StringStat struct {
	Unique    int
	TopValues map[string]int // value -> count
}

// FindResult represents a full-text search match.
type FindResult struct {
	Index int    // object index
	Path  string // dot-path where found
	Value string // the matching value
	File  string // source filename (if available)
}

// SchemaOpts configures schema inference.
type SchemaOpts struct {
	MaxExamples int
}

// TreeOpts configures tree building.
type TreeOpts struct {
	TrackUnique bool
}

// FindOpts configures find behaviour.
type FindOpts struct {
	CaseInsensitive bool
	Regex           bool
	Keys            bool
	First           int // 0 means no limit
}
