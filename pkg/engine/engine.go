package engine

import (
	"fmt"
	"sort"
	"strings"

	"github.com/okaris/jsont/pkg/query"
)

// Result holds the output of a query execution.
type Result struct {
	Objects []any
}

// EngineOpts controls optional engine behaviors.
type EngineOpts struct {
	ShowFile  bool
	ShowIndex bool
}

// Execute parses the query string and applies the query pipeline to the objects.
func Execute(objects []any, queryStr string, opts EngineOpts) (*Result, error) {
	q, err := query.Parse(queryStr)
	if err != nil {
		return nil, err
	}

	result := make([]any, len(objects))
	copy(result, objects)

	// Determine if "select" keyword is explicitly in the query.
	hasExplicitSelect := strings.Contains(strings.ToLower(queryStr), "select")

	// 1. Where filter
	if q.Where != nil {
		filtered := make([]any, 0, len(result))
		for _, obj := range result {
			matched, err := query.Match(q.Where.Condition, obj)
			if err != nil {
				continue
			}
			if matched {
				filtered = append(filtered, obj)
			}
		}
		result = filtered
	}

	// 2. Count (plain or count by)
	if q.Count != nil {
		if q.Count.By == nil {
			// Plain count
			out := &Result{Objects: []any{float64(len(result))}}
			return out, nil
		}
		// Count by field
		return executeCountBy(result, q, q.Count.By)
	}

	// 3. Group by
	if q.GroupBy != nil {
		return executeGroupBy(result, q)
	}

	// 4. Check for aggregate functions in select (without group by)
	if q.Select != nil && hasAggregates(q.Select) {
		return executeAggregateSelect(result, q.Select)
	}

	// 5. Sort by (before select, so sort expressions can reference original fields)
	if q.SortBy != nil {
		applySortBy(result, q.SortBy)
	}

	// 6. Select
	if q.Select != nil {
		result, err = applySelect(result, q.Select, hasExplicitSelect)
		if err != nil {
			return nil, err
		}
	}

	// 7. Distinct
	if q.Distinct != nil {
		result = applyDistinct(result, q.Distinct)
	}

	// 8. Limit / First / Last
	if q.Limit != nil {
		result = applyLimit(result, q.Limit)
	}

	return &Result{Objects: result}, nil
}

// selectKeyName returns the key to use for a select field in the output map.
func selectKeyName(f query.SelectField) string {
	if f.Alias != "" {
		return f.Alias
	}
	switch e := f.Expr.(type) {
	case query.DotPath:
		// Use last segment: ".error.message" -> "message", ".name" -> "name"
		path := e.Path
		if strings.HasPrefix(path, ".") {
			path = path[1:]
		}
		// For nested paths like "address.city", use the full path as key
		return path
	case query.FieldAccess:
		// Use the trailing field path: .items[0].name -> "name"
		path := e.Field
		if strings.HasPrefix(path, ".") {
			path = path[1:]
		}
		return path
	case query.FuncCall:
		return e.Name
	default:
		return fmt.Sprintf("%v", f.Expr)
	}
}

func applySelect(objects []any, sel *query.SelectClause, explicitSelect bool) ([]any, error) {
	// Check for wildcard
	if len(sel.Fields) == 1 {
		if _, ok := sel.Fields[0].Expr.(query.WildcardExpr); ok {
			return objects, nil
		}
	}

	// Single field with no alias and implicit select -> return bare values
	singleBare := len(sel.Fields) == 1 && sel.Fields[0].Alias == "" && !explicitSelect

	result := make([]any, 0, len(objects))
	for _, obj := range objects {
		if singleBare {
			val, err := query.Eval(sel.Fields[0].Expr, obj)
			if err != nil {
				result = append(result, nil)
				continue
			}
			result = append(result, val)
		} else {
			m := make(map[string]any)
			for _, f := range sel.Fields {
				if _, ok := f.Expr.(query.WildcardExpr); ok {
					// Wildcard in multi-field select: merge all fields
					if om, ok := obj.(map[string]any); ok {
						for k, v := range om {
							m[k] = v
						}
					}
					continue
				}
				key := selectKeyName(f)
				val, err := query.Eval(f.Expr, obj)
				if err != nil {
					m[key] = nil
					continue
				}
				m[key] = val
			}
			result = append(result, m)
		}
	}
	return result, nil
}

func applySortBy(objects []any, sb *query.SortByClause) {
	sort.SliceStable(objects, func(i, j int) bool {
		for _, sf := range sb.Fields {
			vi, _ := query.Eval(sf.Expr, objects[i])
			vj, _ := query.Eval(sf.Expr, objects[j])
			cmp := compareValues(vi, vj)
			if cmp == 0 {
				continue
			}
			if sf.Desc {
				return cmp > 0
			}
			return cmp < 0
		}
		return false
	})
}

func compareValues(a, b any) int {
	// Nulls last
	if a == nil && b == nil {
		return 0
	}
	if a == nil {
		return 1
	}
	if b == nil {
		return -1
	}

	// Numbers
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

	// Strings
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

func applyDistinct(objects []any, d *query.DistinctClause) []any {
	seen := make(map[string]bool)
	result := make([]any, 0)
	for _, obj := range objects {
		val, err := query.Eval(d.Expr, obj)
		if err != nil {
			continue
		}
		key := fmt.Sprintf("%v", val)
		if seen[key] {
			continue
		}
		seen[key] = true
		result = append(result, val)
	}
	return result
}

func applyLimit(objects []any, lim *query.LimitClause) []any {
	if lim.IsLast {
		if lim.N >= len(objects) {
			return objects
		}
		return objects[len(objects)-lim.N:]
	}
	start := lim.Offset
	if start >= len(objects) {
		return []any{}
	}
	end := start + lim.N
	if end > len(objects) {
		end = len(objects)
	}
	return objects[start:end]
}

func executeCountBy(objects []any, q *query.Query, byExpr query.Expr) (*Result, error) {
	// Get the field name for the key
	keyName := "key"
	if dp, ok := byExpr.(query.DotPath); ok {
		path := dp.Path
		if strings.HasPrefix(path, ".") {
			path = path[1:]
		}
		// Use last segment
		parts := strings.Split(path, ".")
		keyName = parts[len(parts)-1]
	}

	groups := make(map[string]float64)
	groupKeys := make(map[string]any) // preserve original values
	order := make([]string, 0)
	for _, obj := range objects {
		val, _ := query.Eval(byExpr, obj)
		key := fmt.Sprintf("%v", val)
		if _, exists := groups[key]; !exists {
			order = append(order, key)
			groupKeys[key] = val
		}
		groups[key]++
	}

	result := make([]any, 0, len(groups))
	for _, key := range order {
		m := map[string]any{
			keyName: groupKeys[key],
			"count": groups[key],
		}
		result = append(result, m)
	}

	// Apply sort if present
	if q.SortBy != nil {
		applySortBy(result, q.SortBy)
	}

	// Apply limit
	if q.Limit != nil {
		result = applyLimit(result, q.Limit)
	}

	return &Result{Objects: result}, nil
}

func executeGroupBy(objects []any, q *query.Query) (*Result, error) {
	byExpr := q.GroupBy.Expr

	// Get the field name for the key
	keyName := "key"
	if dp, ok := byExpr.(query.DotPath); ok {
		path := dp.Path
		if strings.HasPrefix(path, ".") {
			path = path[1:]
		}
		parts := strings.Split(path, ".")
		keyName = parts[len(parts)-1]
	}

	type group struct {
		key   any
		items []any
	}
	groupMap := make(map[string]*group)
	order := make([]string, 0)

	for _, obj := range objects {
		val, _ := query.Eval(byExpr, obj)
		skey := fmt.Sprintf("%v", val)
		if _, exists := groupMap[skey]; !exists {
			groupMap[skey] = &group{key: val, items: make([]any, 0)}
			order = append(order, skey)
		}
		groupMap[skey].items = append(groupMap[skey].items, obj)
	}

	// If there are aggregate functions in select, compute per group
	if q.Select != nil && hasAggregates(q.Select) {
		result := make([]any, 0, len(order))
		for _, skey := range order {
			g := groupMap[skey]
			m := make(map[string]any)
			for _, f := range q.Select.Fields {
				key := selectKeyName(f)
				switch e := f.Expr.(type) {
				case query.FuncCall:
					val, err := computeAggregate(e.Name, e.Args, g.items)
					if err != nil {
						return nil, err
					}
					m[key] = val
				case query.DotPath:
					// Use the group key value
					m[key] = g.key
				default:
					// Evaluate against first item
					if len(g.items) > 0 {
						val, _ := query.Eval(f.Expr, g.items[0])
						m[key] = val
					}
				}
			}
			result = append(result, m)
		}
		return &Result{Objects: result}, nil
	}

	// Default group by: return group key, count, and items
	result := make([]any, 0, len(order))
	for _, skey := range order {
		g := groupMap[skey]
		m := map[string]any{
			keyName: g.key,
			"count": float64(len(g.items)),
			"items": g.items,
		}
		result = append(result, m)
	}

	return &Result{Objects: result}, nil
}

// hasAggregates checks if the select clause contains aggregate function calls.
func hasAggregates(sel *query.SelectClause) bool {
	for _, f := range sel.Fields {
		if fc, ok := f.Expr.(query.FuncCall); ok {
			switch fc.Name {
			case "count", "avg", "min", "max", "sum":
				return true
			}
		}
	}
	return false
}

// isAggregate checks if a function name is an aggregate.
func isAggregate(name string) bool {
	switch name {
	case "count", "avg", "min", "max", "sum":
		return true
	}
	return false
}

func computeAggregate(name string, args []query.Expr, objects []any) (any, error) {
	switch name {
	case "count":
		if len(args) > 0 {
			return nil, fmt.Errorf("count() takes no arguments")
		}
		return float64(len(objects)), nil
	case "avg":
		if len(args) != 1 {
			return nil, fmt.Errorf("avg() requires exactly 1 argument")
		}
		var sum float64
		var count int
		for _, obj := range objects {
			val, err := query.Eval(args[0], obj)
			if err != nil || val == nil {
				continue
			}
			f, ok := toFloat(val)
			if !ok {
				continue
			}
			sum += f
			count++
		}
		if count == 0 {
			return nil, nil
		}
		return sum / float64(count), nil
	case "sum":
		if len(args) != 1 {
			return nil, fmt.Errorf("sum() requires exactly 1 argument")
		}
		var sum float64
		for _, obj := range objects {
			val, err := query.Eval(args[0], obj)
			if err != nil || val == nil {
				continue
			}
			f, ok := toFloat(val)
			if !ok {
				continue
			}
			sum += f
		}
		return sum, nil
	case "min":
		if len(args) != 1 {
			return nil, fmt.Errorf("min() requires exactly 1 argument")
		}
		var minVal float64
		first := true
		for _, obj := range objects {
			val, err := query.Eval(args[0], obj)
			if err != nil || val == nil {
				continue
			}
			f, ok := toFloat(val)
			if !ok {
				continue
			}
			if first || f < minVal {
				minVal = f
				first = false
			}
		}
		if first {
			return nil, nil
		}
		return minVal, nil
	case "max":
		if len(args) != 1 {
			return nil, fmt.Errorf("max() requires exactly 1 argument")
		}
		var maxVal float64
		first := true
		for _, obj := range objects {
			val, err := query.Eval(args[0], obj)
			if err != nil || val == nil {
				continue
			}
			f, ok := toFloat(val)
			if !ok {
				continue
			}
			if first || f > maxVal {
				maxVal = f
				first = false
			}
		}
		if first {
			return nil, nil
		}
		return maxVal, nil
	default:
		return nil, fmt.Errorf("unknown aggregate function: %s", name)
	}
}

func executeAggregateSelect(objects []any, sel *query.SelectClause) (*Result, error) {
	m := make(map[string]any)
	for _, f := range sel.Fields {
		key := selectKeyName(f)
		if fc, ok := f.Expr.(query.FuncCall); ok && isAggregate(fc.Name) {
			val, err := computeAggregate(fc.Name, fc.Args, objects)
			if err != nil {
				return nil, err
			}
			m[key] = val
		} else {
			// Non-aggregate in aggregate select: evaluate against first object
			if len(objects) > 0 {
				val, _ := query.Eval(f.Expr, objects[0])
				m[key] = val
			}
		}
	}
	return &Result{Objects: []any{m}}, nil
}

