package explore

import (
	"encoding/json"
	"fmt"
	"math"
	"math/rand"
	"regexp"
	"sort"
	"strings"
)

// InferSchema walks all objects and collects field paths with types and frequencies.
func InferSchema(objects []any, opts SchemaOpts) []SchemaField {
	if len(objects) == 0 {
		return nil
	}

	type fieldInfo struct {
		types    map[string]bool
		count    int
		examples []any
		unique   map[string]bool
	}

	fields := map[string]*fieldInfo{}
	total := len(objects)

	var walk func(prefix string, v any)
	walk = func(prefix string, v any) {
		fi := fields[prefix]
		if fi == nil {
			fi = &fieldInfo{
				types:  map[string]bool{},
				unique: map[string]bool{},
			}
			fields[prefix] = fi
		}
		fi.count++

		switch val := v.(type) {
		case nil:
			fi.types["null"] = true
			fi.unique["null"] = true
			if opts.MaxExamples == 0 || len(fi.examples) < opts.MaxExamples {
				fi.examples = append(fi.examples, nil)
			}
		case bool:
			fi.types["boolean"] = true
			s := fmt.Sprintf("%v", val)
			fi.unique[s] = true
			if opts.MaxExamples == 0 || len(fi.examples) < opts.MaxExamples {
				fi.examples = append(fi.examples, val)
			}
		case float64:
			fi.types["number"] = true
			s := fmt.Sprintf("%v", val)
			fi.unique[s] = true
			if opts.MaxExamples == 0 || len(fi.examples) < opts.MaxExamples {
				fi.examples = append(fi.examples, val)
			}
		case string:
			fi.types["string"] = true
			fi.unique[val] = true
			if opts.MaxExamples == 0 || len(fi.examples) < opts.MaxExamples {
				fi.examples = append(fi.examples, val)
			}
		case map[string]any:
			fi.types["object"] = true
			for k, child := range val {
				walk(prefix+"."+k, child)
			}
		case []any:
			elemType := arrayElementType(val)
			fi.types["array"] = true
			if elemType != "" {
				fi.types["array["+elemType+"]"] = true
			}
			s, _ := json.Marshal(val)
			fi.unique[string(s)] = true
			if opts.MaxExamples == 0 || len(fi.examples) < opts.MaxExamples {
				fi.examples = append(fi.examples, val)
			}
		}
	}

	for _, obj := range objects {
		m, ok := obj.(map[string]any)
		if !ok {
			continue
		}
		for k, v := range m {
			walk("."+k, v)
		}
	}

	// Remove "object" type entries that are intermediate nodes (have children)
	// Actually, keep them but filter: we want leaf fields + container fields that have types
	// For schema, skip pure object intermediates — only report leaf paths and arrays
	result := make([]SchemaField, 0, len(fields))
	for path, fi := range fields {
		// Skip pure intermediate objects — those with only "object" type
		types := make([]string, 0, len(fi.types))
		for t := range fi.types {
			if t == "object" {
				continue
			}
			types = append(types, t)
		}
		// If the only type was "object" with no other types, and it has children, skip
		if len(types) == 0 && fi.types["object"] {
			// Check if it has children
			hasChildren := false
			for other := range fields {
				if other != path && strings.HasPrefix(other, path+".") {
					hasChildren = true
					break
				}
			}
			if hasChildren {
				continue
			}
			// No children — report as object
			types = []string{"object"}
		}
		if len(types) == 0 {
			continue
		}

		// Ensure plain "array" is present if any array type exists
		hasArray := false
		hasTypedArray := false
		for _, t := range types {
			if t == "array" {
				hasArray = true
			}
			if strings.HasPrefix(t, "array[") {
				hasTypedArray = true
			}
		}
		if hasTypedArray && !hasArray {
			types = append(types, "array")
		}

		sort.Strings(types)
		result = append(result, SchemaField{
			Path:      path,
			Types:     types,
			Frequency: float64(fi.count) / float64(total),
			Examples:  fi.examples,
			Unique:    len(fi.unique),
		})
	}

	sort.Slice(result, func(i, j int) bool {
		return result[i].Path < result[j].Path
	})
	return result
}

func arrayElementType(arr []any) string {
	if len(arr) == 0 {
		return ""
	}
	types := map[string]bool{}
	for _, elem := range arr {
		types[jsonType(elem)] = true
	}
	if len(types) == 1 {
		for t := range types {
			return t
		}
	}
	// mixed
	ts := make([]string, 0, len(types))
	for t := range types {
		ts = append(ts, t)
	}
	sort.Strings(ts)
	return strings.Join(ts, "|")
}

func jsonType(v any) string {
	switch v.(type) {
	case nil:
		return "null"
	case bool:
		return "boolean"
	case float64:
		return "number"
	case string:
		return "string"
	case map[string]any:
		return "object"
	case []any:
		return "array"
	default:
		return "unknown"
	}
}

// BuildTree builds a hierarchical tree of field paths.
func BuildTree(objects []any, opts TreeOpts) *TreeNode {
	if len(objects) == 0 {
		return &TreeNode{Path: "(root)", Type: "object"}
	}

	total := len(objects)

	// Collect field info: path -> types seen, count
	type fieldData struct {
		types map[string]bool
		count int
	}
	fields := map[string]*fieldData{}

	var walk func(prefix string, v any)
	walk = func(prefix string, v any) {
		fd := fields[prefix]
		if fd == nil {
			fd = &fieldData{types: map[string]bool{}}
			fields[prefix] = fd
		}
		fd.count++

		switch val := v.(type) {
		case nil:
			fd.types["null"] = true
		case bool:
			fd.types["boolean"] = true
		case float64:
			fd.types["number"] = true
		case string:
			fd.types["string"] = true
		case map[string]any:
			fd.types["object"] = true
			for k, child := range val {
				walk(prefix+"."+k, child)
			}
		case []any:
			elemType := arrayElementType(val)
			if elemType != "" {
				fd.types["array["+elemType+"]"] = true
			} else {
				fd.types["array"] = true
			}
		}
	}

	for _, obj := range objects {
		m, ok := obj.(map[string]any)
		if !ok {
			continue
		}
		for k, v := range m {
			walk(k, v)
		}
	}

	// Build tree from fields
	root := &TreeNode{Path: "(root)", Type: "object"}

	// Get all top-level keys
	type nodeKey struct {
		parts []string
		path  string
	}

	// Group by hierarchy
	for path, fd := range fields {
		parts := strings.Split(path, ".")
		current := root
		for i, part := range parts {
			fullPath := strings.Join(parts[:i+1], ".")
			// Find or create child
			var child *TreeNode
			for _, c := range current.Children {
				if c.Path == part {
					child = c
					break
				}
			}
			if child == nil {
				child = &TreeNode{Path: part}
				current.Children = append(current.Children, child)
			}

			// If this is the leaf (the actual field), set type info
			if fullPath == path {
				// Build type string
				types := make([]string, 0, len(fd.types))
				for t := range fd.types {
					if t == "object" {
						continue // don't show object for intermediates
					}
					types = append(types, t)
				}
				if len(types) == 0 {
					types = []string{"object"}
				}
				sort.Strings(types)
				child.Type = strings.Join(types, "|")
				child.Optional = fd.count < total
			} else {
				// intermediate node
				if child.Type == "" {
					child.Type = "object"
				}
				// Check optionality from the field data of this intermediate
				if ifd, ok := fields[fullPath]; ok {
					child.Optional = ifd.count < total
				}
			}
			current = child
		}
	}

	// Sort children recursively
	var sortTree func(n *TreeNode)
	sortTree = func(n *TreeNode) {
		sort.Slice(n.Children, func(i, j int) bool {
			return n.Children[i].Path < n.Children[j].Path
		})
		for _, c := range n.Children {
			sortTree(c)
		}
	}
	sortTree(root)

	return root
}

// ListFields returns a sorted list of all unique dot-paths.
func ListFields(objects []any) []string {
	paths := map[string]bool{}

	var walk func(prefix string, v any)
	walk = func(prefix string, v any) {
		switch val := v.(type) {
		case map[string]any:
			for k, child := range val {
				childPath := prefix + "." + k
				paths[childPath] = true
				walk(childPath, child)
			}
		case []any:
			for _, elem := range val {
				if _, ok := elem.(map[string]any); ok {
					walk(prefix+"[]", elem)
				}
			}
		}
	}

	for _, obj := range objects {
		walk("", obj)
	}

	result := make([]string, 0, len(paths))
	for p := range paths {
		result = append(result, p)
	}
	sort.Strings(result)
	return result
}

// Find performs full-text search across all values.
func Find(objects []any, pattern string, opts FindOpts) []FindResult {
	var results []FindResult

	var matcher func(s string) bool
	if opts.Regex {
		re := regexp.MustCompile(pattern)
		matcher = func(s string) bool { return re.MatchString(s) }
	} else if opts.CaseInsensitive {
		lowerPattern := strings.ToLower(pattern)
		matcher = func(s string) bool { return strings.Contains(strings.ToLower(s), lowerPattern) }
	} else {
		matcher = func(s string) bool { return strings.Contains(s, pattern) }
	}

	var walk func(idx int, prefix string, v any)
	walk = func(idx int, prefix string, v any) {
		if opts.First > 0 && len(results) >= opts.First {
			return
		}
		switch val := v.(type) {
		case string:
			if !opts.Keys && matcher(val) {
				results = append(results, FindResult{Index: idx, Path: prefix, Value: val})
			}
		case float64:
			s := fmt.Sprintf("%v", val)
			if !opts.Keys && matcher(s) {
				results = append(results, FindResult{Index: idx, Path: prefix, Value: s})
			}
		case bool:
			s := fmt.Sprintf("%v", val)
			if !opts.Keys && matcher(s) {
				results = append(results, FindResult{Index: idx, Path: prefix, Value: s})
			}
		case map[string]any:
			keys := make([]string, 0, len(val))
			for k := range val {
				keys = append(keys, k)
			}
			sort.Strings(keys)
			for _, k := range keys {
				childPath := prefix + "." + k
				if opts.Keys && matcher(k) {
					results = append(results, FindResult{Index: idx, Path: childPath, Value: k})
					if opts.First > 0 && len(results) >= opts.First {
						return
					}
				}
				walk(idx, childPath, val[k])
				if opts.First > 0 && len(results) >= opts.First {
					return
				}
			}
		case []any:
			for i, elem := range val {
				walk(idx, fmt.Sprintf("%s[%d]", prefix, i), elem)
				if opts.First > 0 && len(results) >= opts.First {
					return
				}
			}
		}
	}

	for i, obj := range objects {
		walk(i, "", obj)
		if opts.First > 0 && len(results) >= opts.First {
			break
		}
	}

	return results
}

// ComputeStats computes statistical summary of objects.
func ComputeStats(objects []any) *Stats {
	stats := &Stats{
		Count:        len(objects),
		NumericStats: map[string]NumericStat{},
		StringStats:  map[string]StringStat{},
		NullCounts:   map[string]int{},
	}

	if len(objects) == 0 {
		return stats
	}

	// Track schemas for SchemaCount
	schemas := map[string]bool{}
	allPaths := map[string]bool{}
	numericValues := map[string][]float64{}
	stringValues := map[string]map[string]int{}

	var walk func(prefix string, v any)
	walk = func(prefix string, v any) {
		allPaths[prefix] = true
		switch val := v.(type) {
		case nil:
			stats.NullCounts[prefix]++
		case float64:
			numericValues[prefix] = append(numericValues[prefix], val)
		case string:
			if stringValues[prefix] == nil {
				stringValues[prefix] = map[string]int{}
			}
			stringValues[prefix][val]++
		case map[string]any:
			for k, child := range val {
				walk(prefix+"."+k, child)
			}
		case []any:
			// don't recurse into arrays for stats purposes
		}
	}

	for _, obj := range objects {
		m, ok := obj.(map[string]any)
		if !ok {
			continue
		}

		// Compute schema shape
		shape := schemaShape(m)
		schemas[shape] = true

		// Walk fields, also detect missing top-level keys
		for k, v := range m {
			walk("."+k, v)
		}
	}

	// Detect nulls for missing keys across objects
	// For each known top-level key, count objects that don't have it
	topKeys := map[string]int{} // key -> count of objects that have it
	for _, obj := range objects {
		m, ok := obj.(map[string]any)
		if !ok {
			continue
		}
		for k := range m {
			topKeys[k]++
		}
	}
	for k, count := range topKeys {
		missing := len(objects) - count
		if missing > 0 {
			path := "." + k
			stats.NullCounts[path] += missing
		}
	}

	stats.SchemaCount = len(schemas)
	stats.Fields = len(allPaths)

	// Compute numeric stats
	for path, vals := range numericValues {
		sort.Float64s(vals)
		n := len(vals)
		sum := 0.0
		for _, v := range vals {
			sum += v
		}
		ns := NumericStat{
			Min:  vals[0],
			Max:  vals[n-1],
			Mean: sum / float64(n),
		}
		if n%2 == 0 {
			ns.Median = (vals[n/2-1] + vals[n/2]) / 2
		} else {
			ns.Median = vals[n/2]
		}
		ns.P95 = percentile(vals, 0.95)
		ns.P99 = percentile(vals, 0.99)
		stats.NumericStats[path] = ns
	}

	// Compute string stats
	for path, counts := range stringValues {
		ss := StringStat{
			Unique:    len(counts),
			TopValues: counts,
		}
		stats.StringStats[path] = ss
	}

	return stats
}

func percentile(sorted []float64, p float64) float64 {
	if len(sorted) == 0 {
		return 0
	}
	idx := p * float64(len(sorted)-1)
	lower := int(math.Floor(idx))
	upper := int(math.Ceil(idx))
	if lower == upper {
		return sorted[lower]
	}
	frac := idx - float64(lower)
	return sorted[lower]*(1-frac) + sorted[upper]*frac
}

func schemaShape(m map[string]any) string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return strings.Join(keys, ",")
}

// Head returns the first n objects.
func Head(objects []any, n int) []any {
	if n <= 0 {
		return nil
	}
	if n > len(objects) {
		n = len(objects)
	}
	result := make([]any, n)
	copy(result, objects[:n])
	return result
}

// Tail returns the last n objects.
func Tail(objects []any, n int) []any {
	if n <= 0 {
		return nil
	}
	if n > len(objects) {
		n = len(objects)
	}
	result := make([]any, n)
	copy(result, objects[len(objects)-n:])
	return result
}

// Count returns the number of objects.
func Count(objects []any) int {
	return len(objects)
}

// Sample returns n randomly selected objects.
func Sample(objects []any, n int) []any {
	if n <= 0 {
		return nil
	}
	if n >= len(objects) {
		result := make([]any, len(objects))
		copy(result, objects)
		return result
	}
	// Fisher-Yates shuffle on indices
	indices := make([]int, len(objects))
	for i := range indices {
		indices[i] = i
	}
	r := rand.New(rand.NewSource(42))
	for i := len(indices) - 1; i > 0; i-- {
		j := r.Intn(i + 1)
		indices[i], indices[j] = indices[j], indices[i]
	}
	result := make([]any, n)
	for i := 0; i < n; i++ {
		result[i] = objects[indices[i]]
	}
	return result
}
