package output

import (
	"bytes"
	"encoding/json"
	"io"
	"strings"
	"testing"
)

// --- Shared test data ---

var testObjects = []any{
	map[string]any{"id": "1", "name": "Alice", "age": float64(30), "active": true},
	map[string]any{"id": "2", "name": "Bob", "age": float64(25), "active": false},
	map[string]any{"id": "3", "name": "Charlie", "age": float64(35), "active": true},
}

// helper to run FormatOutput and return the output string.
func formatTo(t *testing.T, objects []any, opts Opts) string {
	t.Helper()
	var buf bytes.Buffer
	if err := FormatOutput(&buf, objects, opts); err != nil {
		t.Fatalf("FormatOutput returned error: %v", err)
	}
	return buf.String()
}

// mustParseJSON unmarshals s into v; fails the test on error.
func mustParseJSON(t *testing.T, s string, v any) {
	t.Helper()
	if err := json.Unmarshal([]byte(s), v); err != nil {
		t.Fatalf("failed to parse JSON %q: %v", s, err)
	}
}

// -----------------------------------------------------------------------
// JSON
// -----------------------------------------------------------------------

func TestFormatJSON(t *testing.T) {
	opts := Opts{Format: FormatJSON}

	t.Run("single object pretty printed", func(t *testing.T) {
		objs := []any{map[string]any{"id": "1", "name": "Alice"}}
		out := formatTo(t, objs, opts)

		// Must be valid JSON.
		var parsed any
		mustParseJSON(t, out, &parsed)

		// Pretty-printed means it contains newlines and indentation.
		if !strings.Contains(out, "\n") {
			t.Error("expected pretty-printed JSON with newlines")
		}
		if !strings.Contains(out, "  ") && !strings.Contains(out, "\t") {
			t.Error("expected indentation in pretty-printed JSON")
		}
	})

	t.Run("multiple objects as JSON array", func(t *testing.T) {
		out := formatTo(t, testObjects, opts)

		var parsed []any
		mustParseJSON(t, out, &parsed)
		if len(parsed) != 3 {
			t.Fatalf("expected 3 elements, got %d", len(parsed))
		}
	})

	t.Run("empty input", func(t *testing.T) {
		out := formatTo(t, []any{}, opts)
		trimmed := strings.TrimSpace(out)
		// Should produce "[]" or be empty.
		if trimmed != "[]" && trimmed != "" {
			t.Errorf("expected [] or empty for empty input, got %q", trimmed)
		}
	})

	t.Run("nested objects properly indented", func(t *testing.T) {
		nested := []any{
			map[string]any{
				"user": map[string]any{
					"name": "Alice",
					"address": map[string]any{
						"city": "Wonderland",
					},
				},
			},
		}
		out := formatTo(t, nested, opts)

		var parsed any
		mustParseJSON(t, out, &parsed)

		// Verify nesting survived round-trip.
		arr, ok := parsed.([]any)
		if !ok {
			// Could be a single object if implementation unwraps single-element.
			_, ok = parsed.(map[string]any)
			if !ok {
				t.Fatal("unexpected top-level type")
			}
		} else {
			if len(arr) != 1 {
				t.Fatalf("expected 1 element, got %d", len(arr))
			}
		}
	})
}

// -----------------------------------------------------------------------
// JSONL
// -----------------------------------------------------------------------

func TestFormatJSONL(t *testing.T) {
	opts := Opts{Format: FormatJSONL}

	t.Run("multiple objects one per line", func(t *testing.T) {
		out := formatTo(t, testObjects, opts)
		lines := strings.Split(strings.TrimRight(out, "\n"), "\n")
		if len(lines) != 3 {
			t.Fatalf("expected 3 lines, got %d: %q", len(lines), out)
		}
		for i, line := range lines {
			var obj map[string]any
			mustParseJSON(t, line, &obj)
			if obj == nil {
				t.Errorf("line %d parsed to nil", i)
			}
			// Each line should be compact (no extra whitespace between keys).
			if strings.Contains(line, "\n") {
				t.Errorf("line %d contains embedded newline", i)
			}
		}
	})

	t.Run("single object one line", func(t *testing.T) {
		out := formatTo(t, testObjects[:1], opts)
		lines := strings.Split(strings.TrimRight(out, "\n"), "\n")
		if len(lines) != 1 {
			t.Fatalf("expected 1 line, got %d", len(lines))
		}
		var obj map[string]any
		mustParseJSON(t, lines[0], &obj)
	})

	t.Run("preserves all fields", func(t *testing.T) {
		out := formatTo(t, testObjects[:1], opts)
		var obj map[string]any
		mustParseJSON(t, strings.TrimSpace(out), &obj)

		for _, key := range []string{"id", "name", "age", "active"} {
			if _, ok := obj[key]; !ok {
				t.Errorf("missing field %q", key)
			}
		}
	})
}

// -----------------------------------------------------------------------
// Compact JSON
// -----------------------------------------------------------------------

func TestFormatCompact(t *testing.T) {
	opts := Opts{Format: FormatCompact}

	t.Run("single object no whitespace", func(t *testing.T) {
		objs := []any{map[string]any{"a": float64(1)}}
		out := formatTo(t, objs, opts)
		trimmed := strings.TrimSpace(out)

		// Valid JSON.
		var parsed any
		mustParseJSON(t, trimmed, &parsed)

		// No newlines inside the JSON value.
		if strings.Count(trimmed, "\n") > 0 {
			t.Error("compact JSON should not contain newlines inside a value")
		}
	})

	t.Run("multiple objects each on own line", func(t *testing.T) {
		out := formatTo(t, testObjects, opts)
		lines := strings.Split(strings.TrimRight(out, "\n"), "\n")
		// Depending on impl it may be a single compact array or one object per line.
		// Either way each line should be valid JSON.
		for _, line := range lines {
			line = strings.TrimSpace(line)
			if line == "" {
				continue
			}
			var v any
			mustParseJSON(t, line, &v)
		}
	})
}

// -----------------------------------------------------------------------
// Table
// -----------------------------------------------------------------------

func TestFormatTable(t *testing.T) {
	opts := Opts{Format: FormatTable}

	t.Run("flat objects with same keys have header", func(t *testing.T) {
		out := formatTo(t, testObjects, opts)
		lines := nonEmptyLines(out)
		if len(lines) < 4 { // header + 3 data rows minimum
			t.Fatalf("expected at least 4 lines (header + 3 data), got %d:\n%s", len(lines), out)
		}
		header := lines[0]
		// Header should mention our field names.
		for _, col := range []string{"id", "name", "age", "active"} {
			if !containsCI(header, col) {
				t.Errorf("header missing column %q: %q", col, header)
			}
		}
	})

	t.Run("objects with missing keys", func(t *testing.T) {
		objs := []any{
			map[string]any{"a": "1", "b": "2"},
			map[string]any{"a": "3", "c": "4"},
		}
		out := formatTo(t, objs, opts)
		// Should not error; output should contain all three columns a, b, c.
		for _, col := range []string{"a", "b", "c"} {
			if !containsCI(out, col) {
				t.Errorf("table missing column %q", col)
			}
		}
	})

	t.Run("nested objects handled", func(t *testing.T) {
		objs := []any{
			map[string]any{"x": map[string]any{"y": float64(1)}},
		}
		// Should not error.
		out := formatTo(t, objs, opts)
		if out == "" {
			t.Error("expected some table output for nested objects")
		}
	})

	t.Run("empty input", func(t *testing.T) {
		out := formatTo(t, []any{}, opts)
		trimmed := strings.TrimSpace(out)
		// Either empty or just a header.
		if strings.Count(trimmed, "\n") > 1 {
			t.Errorf("expected no data rows for empty input, got:\n%s", out)
		}
	})
}

// -----------------------------------------------------------------------
// CSV
// -----------------------------------------------------------------------

func TestFormatCSV(t *testing.T) {
	opts := Opts{Format: FormatCSV}

	t.Run("standard objects header plus rows", func(t *testing.T) {
		out := formatTo(t, testObjects, opts)
		lines := nonEmptyLines(out)
		if len(lines) < 4 {
			t.Fatalf("expected header + 3 rows, got %d lines", len(lines))
		}
		// Header should list field names separated by commas.
		if !strings.Contains(lines[0], ",") {
			t.Error("header should contain commas")
		}
	})

	t.Run("values with commas are quoted", func(t *testing.T) {
		objs := []any{map[string]any{"val": "hello, world"}}
		out := formatTo(t, objs, opts)
		// The value must be enclosed in quotes.
		if !strings.Contains(out, `"hello, world"`) {
			t.Errorf("comma-containing value should be quoted: %q", out)
		}
	})

	t.Run("values with quotes are escaped", func(t *testing.T) {
		objs := []any{map[string]any{"val": `say "hi"`}}
		out := formatTo(t, objs, opts)
		// CSV escapes quotes by doubling them.
		if !strings.Contains(out, `""hi""`) && !strings.Contains(out, `"say ""hi"""`) {
			t.Errorf("quote-containing value should be escaped: %q", out)
		}
	})

	t.Run("values with newlines are quoted", func(t *testing.T) {
		objs := []any{map[string]any{"val": "line1\nline2"}}
		out := formatTo(t, objs, opts)
		if !strings.Contains(out, `"line1`) {
			t.Errorf("newline-containing value should be quoted: %q", out)
		}
	})

	t.Run("null values as empty string", func(t *testing.T) {
		objs := []any{map[string]any{"a": "1", "b": nil}}
		out := formatTo(t, objs, opts)
		// Should not contain the literal word "null" as a CSV field (unless quoted).
		lines := nonEmptyLines(out)
		if len(lines) < 2 {
			t.Fatal("expected at least header + 1 row")
		}
		dataLine := lines[1]
		// The nil field should be empty, not "null".
		if strings.Contains(dataLine, "null") {
			t.Errorf("nil values should be empty, not 'null': %q", dataLine)
		}
	})

	t.Run("consistent column order", func(t *testing.T) {
		objs := []any{
			map[string]any{"z": "1", "a": "2"},
			map[string]any{"z": "3", "a": "4"},
		}
		out := formatTo(t, objs, opts)
		lines := nonEmptyLines(out)
		if len(lines) < 3 {
			t.Fatal("expected header + 2 rows")
		}
		headerCols := strings.Split(lines[0], ",")
		// Both rows should have the same number of columns as the header.
		for i := 1; i < len(lines); i++ {
			rowCols := splitCSVRow(lines[i])
			if len(rowCols) != len(headerCols) {
				t.Errorf("row %d has %d cols, header has %d", i, len(rowCols), len(headerCols))
			}
		}
	})
}

// -----------------------------------------------------------------------
// TSV
// -----------------------------------------------------------------------

func TestFormatTSV(t *testing.T) {
	opts := Opts{Format: FormatTSV}

	t.Run("standard objects tab separated", func(t *testing.T) {
		out := formatTo(t, testObjects, opts)
		lines := nonEmptyLines(out)
		if len(lines) < 4 {
			t.Fatalf("expected header + 3 rows, got %d lines", len(lines))
		}
		if !strings.Contains(lines[0], "\t") {
			t.Error("header should be tab-separated")
		}
	})

	t.Run("values with tabs escaped", func(t *testing.T) {
		objs := []any{map[string]any{"val": "a\tb"}}
		out := formatTo(t, objs, opts)
		lines := nonEmptyLines(out)
		if len(lines) < 2 {
			t.Fatal("expected header + 1 row")
		}
		dataLine := lines[1]
		// The embedded tab should be escaped or quoted so it doesn't break columns.
		// Count tabs in header vs data to verify column count matches.
		headerTabs := strings.Count(lines[0], "\t")
		dataTabs := strings.Count(dataLine, "\t")
		if dataTabs != headerTabs {
			t.Errorf("data line has %d tabs vs header's %d — embedded tab not escaped", dataTabs, headerTabs)
		}
	})
}

// -----------------------------------------------------------------------
// Raw
// -----------------------------------------------------------------------

func TestFormatRaw(t *testing.T) {
	opts := Opts{Format: FormatRaw}

	t.Run("string values without JSON quotes", func(t *testing.T) {
		objs := []any{"hello"}
		out := formatTo(t, objs, opts)
		trimmed := strings.TrimSpace(out)
		if trimmed != "hello" {
			t.Errorf("expected raw string 'hello', got %q", trimmed)
		}
	})

	t.Run("number values as plain number", func(t *testing.T) {
		objs := []any{float64(42)}
		out := formatTo(t, objs, opts)
		trimmed := strings.TrimSpace(out)
		if trimmed != "42" {
			t.Errorf("expected '42', got %q", trimmed)
		}
	})

	t.Run("null to empty or null", func(t *testing.T) {
		objs := []any{nil}
		out := formatTo(t, objs, opts)
		trimmed := strings.TrimSpace(out)
		if trimmed != "" && trimmed != "null" {
			t.Errorf("expected empty or 'null' for nil, got %q", trimmed)
		}
	})

	t.Run("array of strings one per line", func(t *testing.T) {
		objs := []any{"alpha", "beta", "gamma"}
		out := formatTo(t, objs, opts)
		lines := nonEmptyLines(out)
		if len(lines) != 3 {
			t.Fatalf("expected 3 lines, got %d: %q", len(lines), out)
		}
		expected := []string{"alpha", "beta", "gamma"}
		for i, line := range lines {
			if strings.TrimSpace(line) != expected[i] {
				t.Errorf("line %d: expected %q, got %q", i, expected[i], line)
			}
		}
	})
}

// -----------------------------------------------------------------------
// Nul
// -----------------------------------------------------------------------

func TestFormatNul(t *testing.T) {
	opts := Opts{Format: FormatNul}

	t.Run("null byte delimited", func(t *testing.T) {
		objs := []any{"aaa", "bbb", "ccc"}
		out := formatTo(t, objs, opts)
		parts := strings.Split(out, "\x00")
		// Expect at least 3 non-empty parts.
		var nonEmpty []string
		for _, p := range parts {
			if p != "" {
				nonEmpty = append(nonEmpty, p)
			}
		}
		if len(nonEmpty) != 3 {
			t.Fatalf("expected 3 nul-delimited values, got %d: %q", len(nonEmpty), parts)
		}
	})
}

// -----------------------------------------------------------------------
// ShowFile option
// -----------------------------------------------------------------------

func TestFormatShowFile(t *testing.T) {
	t.Run("JSON with file metadata", func(t *testing.T) {
		objs := []any{
			map[string]any{"_file": "data.json", "key": "value"},
		}
		opts := Opts{Format: FormatJSON, ShowFile: true}
		out := formatTo(t, objs, opts)
		if !strings.Contains(out, "data.json") {
			t.Errorf("expected file name in output: %q", out)
		}
	})

	t.Run("table with file metadata", func(t *testing.T) {
		objs := []any{
			map[string]any{"_file": "a.json", "x": "1"},
			map[string]any{"_file": "b.json", "x": "2"},
		}
		opts := Opts{Format: FormatTable, ShowFile: true}
		out := formatTo(t, objs, opts)
		if !strings.Contains(out, "a.json") || !strings.Contains(out, "b.json") {
			t.Errorf("expected file names in table output: %q", out)
		}
	})
}

// -----------------------------------------------------------------------
// ShowIndex option
// -----------------------------------------------------------------------

func TestFormatShowIndex(t *testing.T) {
	t.Run("objects with index metadata", func(t *testing.T) {
		objs := []any{
			map[string]any{"_index": float64(0), "val": "first"},
			map[string]any{"_index": float64(1), "val": "second"},
		}
		opts := Opts{Format: FormatTable, ShowIndex: true}
		out := formatTo(t, objs, opts)
		if !strings.Contains(out, "0") || !strings.Contains(out, "1") {
			t.Errorf("expected indices in output: %q", out)
		}
	})
}

// -----------------------------------------------------------------------
// Flatten option
// -----------------------------------------------------------------------

func TestFormatFlatten(t *testing.T) {
	t.Run("nested object to dot paths", func(t *testing.T) {
		objs := []any{
			map[string]any{"a": map[string]any{"b": float64(1)}},
		}
		opts := Opts{Format: FormatJSON, Flatten: true}
		out := formatTo(t, objs, opts)
		if !strings.Contains(out, "a.b") {
			t.Errorf("expected flattened key 'a.b' in output: %q", out)
		}
	})

	t.Run("deeply nested", func(t *testing.T) {
		objs := []any{
			map[string]any{
				"x": map[string]any{
					"y": map[string]any{
						"z": "deep",
					},
				},
			},
		}
		opts := Opts{Format: FormatJSON, Flatten: true}
		out := formatTo(t, objs, opts)
		if !strings.Contains(out, "x.y.z") {
			t.Errorf("expected flattened key 'x.y.z' in output: %q", out)
		}
	})

	t.Run("arrays flatten to indexed keys", func(t *testing.T) {
		objs := []any{
			map[string]any{"a": []any{"first", "second"}},
		}
		opts := Opts{Format: FormatJSON, Flatten: true}
		out := formatTo(t, objs, opts)
		// Accept either a.0/a.1 or a[0]/a[1] style.
		hasIndexed := strings.Contains(out, "a.0") || strings.Contains(out, "a[0]")
		if !hasIndexed {
			t.Errorf("expected flattened array keys (a.0 or a[0]) in output: %q", out)
		}
	})
}

// -----------------------------------------------------------------------
// Error handling
// -----------------------------------------------------------------------

func TestFormatOutput_WriterError(t *testing.T) {
	w := &failWriter{}
	err := FormatOutput(w, testObjects, Opts{Format: FormatJSON})
	if err == nil {
		t.Error("expected error when writer fails")
	}
}

// -----------------------------------------------------------------------
// Helpers
// -----------------------------------------------------------------------

// nonEmptyLines splits s into lines and discards empty / separator-only lines.
func nonEmptyLines(s string) []string {
	raw := strings.Split(s, "\n")
	var out []string
	for _, line := range raw {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			continue
		}
		// Skip separator lines (e.g. table dividers like "---+---").
		if allDashesOrPluses(trimmed) {
			continue
		}
		out = append(out, line)
	}
	return out
}

func allDashesOrPluses(s string) bool {
	for _, r := range s {
		if r != '-' && r != '+' && r != '=' && r != '|' && r != ' ' {
			return false
		}
	}
	return true
}

func containsCI(haystack, needle string) bool {
	return strings.Contains(strings.ToLower(haystack), strings.ToLower(needle))
}

// splitCSVRow naively splits a CSV row, respecting quoted fields.
func splitCSVRow(line string) []string {
	var fields []string
	var current strings.Builder
	inQuote := false
	for i := 0; i < len(line); i++ {
		c := line[i]
		switch {
		case c == '"' && !inQuote:
			inQuote = true
		case c == '"' && inQuote:
			if i+1 < len(line) && line[i+1] == '"' {
				current.WriteByte('"')
				i++
			} else {
				inQuote = false
			}
		case c == ',' && !inQuote:
			fields = append(fields, current.String())
			current.Reset()
		default:
			current.WriteByte(c)
		}
	}
	fields = append(fields, current.String())
	return fields
}

// failWriter is an io.Writer that always returns an error.
type failWriter struct{}

func (fw *failWriter) Write(p []byte) (int, error) {
	return 0, io.ErrClosedPipe
}
