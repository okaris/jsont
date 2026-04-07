package output

import (
	"encoding/csv"
	"encoding/json"
	"fmt"
	"io"
	"sort"
	"strconv"
	"strings"
)

type Format int

const (
	FormatAuto Format = iota
	FormatJSON
	FormatJSONL
	FormatCompact
	FormatTable
	FormatCSV
	FormatTSV
	FormatRaw
	FormatNul
)

type Opts struct {
	Format    Format
	Color     bool
	ShowFile  bool
	ShowIndex bool
	Flatten   bool
}

func FormatOutput(w io.Writer, objects []any, opts Opts) error {
	// Apply transformations
	objects = applyTransforms(objects, opts)

	switch opts.Format {
	case FormatAuto, FormatJSON:
		return formatJSON(w, objects)
	case FormatJSONL:
		return formatJSONL(w, objects)
	case FormatCompact:
		return formatCompact(w, objects)
	case FormatTable:
		return formatTable(w, objects)
	case FormatCSV:
		return formatCSV(w, objects)
	case FormatTSV:
		return formatTSV(w, objects)
	case FormatRaw:
		return formatRaw(w, objects)
	case FormatNul:
		return formatNul(w, objects)
	default:
		return formatJSON(w, objects)
	}
}

func applyTransforms(objects []any, opts Opts) []any {
	if !opts.Flatten && !opts.ShowFile && !opts.ShowIndex {
		return objects
	}

	result := make([]any, len(objects))
	for i, obj := range objects {
		if opts.Flatten {
			if m, ok := obj.(map[string]any); ok {
				obj = flattenMap("", m)
			}
		}
		// ShowFile and ShowIndex: the metadata is already in the objects (_file, _index keys).
		// We just need to not strip them. They're kept by default.
		result[i] = obj
	}
	return result
}

func flattenMap(prefix string, m map[string]any) map[string]any {
	result := make(map[string]any)
	for k, v := range m {
		key := k
		if prefix != "" {
			key = prefix + "." + k
		}
		switch val := v.(type) {
		case map[string]any:
			for fk, fv := range flattenMap(key, val) {
				result[fk] = fv
			}
		case []any:
			for i, item := range val {
				indexKey := key + "." + strconv.Itoa(i)
				if sub, ok := item.(map[string]any); ok {
					for fk, fv := range flattenMap(indexKey, sub) {
						result[fk] = fv
					}
				} else {
					result[indexKey] = item
				}
			}
		default:
			result[key] = v
		}
	}
	return result
}

func formatJSON(w io.Writer, objects []any) error {
	var data any
	if len(objects) == 0 {
		data = []any{}
	} else if len(objects) == 1 {
		data = objects[0]
	} else {
		data = objects
	}
	b, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		return err
	}
	_, err = w.Write(append(b, '\n'))
	return err
}

func formatJSONL(w io.Writer, objects []any) error {
	for _, obj := range objects {
		b, err := json.Marshal(obj)
		if err != nil {
			return err
		}
		if _, err := w.Write(append(b, '\n')); err != nil {
			return err
		}
	}
	return nil
}

func formatCompact(w io.Writer, objects []any) error {
	// Same as JSONL: one compact JSON per line
	return formatJSONL(w, objects)
}

func formatTable(w io.Writer, objects []any) error {
	if len(objects) == 0 {
		return nil
	}

	// Collect all keys and rows
	keys := collectKeys(objects)
	rows := make([][]string, len(objects))
	for i, obj := range objects {
		rows[i] = objectToRow(obj, keys)
	}

	// Calculate column widths
	widths := make([]int, len(keys))
	for i, k := range keys {
		widths[i] = len(k)
	}
	for _, row := range rows {
		for i, cell := range row {
			if len(cell) > widths[i] {
				widths[i] = len(cell)
			}
		}
	}

	// Print header
	headerParts := make([]string, len(keys))
	for i, k := range keys {
		headerParts[i] = padRight(k, widths[i])
	}
	line := strings.Join(headerParts, " | ")
	if _, err := fmt.Fprintf(w, " %s \n", line); err != nil {
		return err
	}

	// Separator
	sepParts := make([]string, len(keys))
	for i := range keys {
		sepParts[i] = strings.Repeat("-", widths[i])
	}
	sep := strings.Join(sepParts, "-+-")
	if _, err := fmt.Fprintf(w, "-%s-\n", sep); err != nil {
		return err
	}

	// Data rows
	for _, row := range rows {
		parts := make([]string, len(keys))
		for i, cell := range row {
			parts[i] = padRight(cell, widths[i])
		}
		if _, err := fmt.Fprintf(w, " %s \n", strings.Join(parts, " | ")); err != nil {
			return err
		}
	}
	return nil
}

func collectKeys(objects []any) []string {
	seen := make(map[string]bool)
	var keys []string
	for _, obj := range objects {
		if m, ok := obj.(map[string]any); ok {
			for k := range m {
				if !seen[k] {
					seen[k] = true
					keys = append(keys, k)
				}
			}
		}
	}
	sort.Strings(keys)
	return keys
}

func objectToRow(obj any, keys []string) []string {
	row := make([]string, len(keys))
	m, ok := obj.(map[string]any)
	if !ok {
		// Not a map, just marshal the whole thing into first column
		if len(keys) > 0 {
			b, _ := json.Marshal(obj)
			row[0] = string(b)
		}
		return row
	}
	for i, k := range keys {
		v, exists := m[k]
		if !exists {
			row[i] = ""
			continue
		}
		row[i] = valueToString(v)
	}
	return row
}

func valueToString(v any) string {
	if v == nil {
		return ""
	}
	switch val := v.(type) {
	case string:
		return val
	case float64:
		if val == float64(int64(val)) {
			return strconv.FormatInt(int64(val), 10)
		}
		return strconv.FormatFloat(val, 'f', -1, 64)
	case bool:
		return strconv.FormatBool(val)
	case json.Number:
		return val.String()
	default:
		// Nested objects/arrays: marshal to JSON
		b, _ := json.Marshal(val)
		return string(b)
	}
}

func padRight(s string, width int) string {
	if len(s) >= width {
		return s
	}
	return s + strings.Repeat(" ", width-len(s))
}

func formatCSV(w io.Writer, objects []any) error {
	if len(objects) == 0 {
		return nil
	}

	keys := collectKeys(objects)
	cw := csv.NewWriter(w)

	// Header
	if err := cw.Write(keys); err != nil {
		return err
	}

	// Data rows
	for _, obj := range objects {
		row := objectToRow(obj, keys)
		if err := cw.Write(row); err != nil {
			return err
		}
	}
	cw.Flush()
	return cw.Error()
}

func formatTSV(w io.Writer, objects []any) error {
	if len(objects) == 0 {
		return nil
	}

	keys := collectKeys(objects)

	// Header
	if _, err := fmt.Fprintf(w, "%s\n", strings.Join(keys, "\t")); err != nil {
		return err
	}

	// Data rows
	for _, obj := range objects {
		row := objectToRow(obj, keys)
		// Escape embedded tabs
		escaped := make([]string, len(row))
		for i, cell := range row {
			escaped[i] = strings.ReplaceAll(cell, "\t", "\\t")
		}
		if _, err := fmt.Fprintf(w, "%s\n", strings.Join(escaped, "\t")); err != nil {
			return err
		}
	}
	return nil
}

func formatRaw(w io.Writer, objects []any) error {
	for _, obj := range objects {
		val := rawValue(obj)
		if _, err := fmt.Fprintf(w, "%s\n", val); err != nil {
			return err
		}
	}
	return nil
}

func rawValue(v any) string {
	if v == nil {
		return ""
	}
	switch val := v.(type) {
	case string:
		return val
	case float64:
		if val == float64(int64(val)) {
			return strconv.FormatInt(int64(val), 10)
		}
		return strconv.FormatFloat(val, 'f', -1, 64)
	case bool:
		return strconv.FormatBool(val)
	default:
		b, _ := json.Marshal(val)
		return string(b)
	}
}

func formatNul(w io.Writer, objects []any) error {
	for _, obj := range objects {
		val := rawValue(obj)
		if _, err := fmt.Fprintf(w, "%s\x00", val); err != nil {
			return err
		}
	}
	return nil
}
