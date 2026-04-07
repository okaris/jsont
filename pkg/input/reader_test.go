package input

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

func writeTempFile(t *testing.T, name, content string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), name)
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatalf("writeTempFile: %v", err)
	}
	return path
}

// collectAll drains the reader into a slice of Objects.
func collectAll(t *testing.T, r *Reader) []*Object {
	t.Helper()
	var out []*Object
	for {
		obj, err := r.Next()
		if err != nil {
			t.Fatalf("Next returned unexpected error: %v", err)
		}
		if obj == nil {
			break
		}
		out = append(out, obj)
	}
	return out
}

// objectMap converts an Object.Value to map[string]any for easier assertions.
func objectMap(t *testing.T, obj *Object) map[string]any {
	t.Helper()
	m, ok := obj.Value.(map[string]any)
	if !ok {
		t.Fatalf("expected map[string]any, got %T", obj.Value)
	}
	return m
}

// testdataPath returns the absolute path to a file in the testdata directory.
func testdataPath(name string) string {
	return filepath.Join("..", "..", "testdata", name)
}

// ---------------------------------------------------------------------------
// TestDetectFormat
// ---------------------------------------------------------------------------

func TestDetectFormat(t *testing.T) {
	tests := []struct {
		name   string
		input  string
		expect Format
	}{
		{
			name:   "single JSON object",
			input:  `{"key": "value"}`,
			expect: FormatJSON,
		},
		{
			name:   "JSON array",
			input:  `[{"a":1},{"b":2}]`,
			expect: FormatJSON,
		},
		{
			name:   "JSONL two lines",
			input:  "{\"a\":1}\n{\"b\":2}\n",
			expect: FormatJSONL,
		},
		{
			name:   "empty input",
			input:  "",
			expect: FormatJSON,
		},
		{
			name:   "whitespace then object",
			input:  "   \t\n  {\"x\":1}",
			expect: FormatJSON,
		},
		{
			name:   "whitespace then array",
			input:  "  \n [1,2,3]",
			expect: FormatJSON,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := DetectFormat([]byte(tc.input))
			if got != tc.expect {
				t.Errorf("DetectFormat(%q) = %d, want %d", tc.input, got, tc.expect)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// TestReadJSON
// ---------------------------------------------------------------------------

func TestReadJSON(t *testing.T) {
	r, err := NewReader([]string{testdataPath("simple.json")}, ReaderOpts{})
	if err != nil {
		t.Fatalf("NewReader: %v", err)
	}

	objs := collectAll(t, r)

	if len(objs) != 1 {
		t.Fatalf("expected 1 object, got %d", len(objs))
	}

	obj := objs[0]

	// Check metadata.
	if !strings.HasSuffix(obj.File, "simple.json") {
		t.Errorf("File = %q, want suffix simple.json", obj.File)
	}
	if obj.Index != 0 {
		t.Errorf("Index = %d, want 0", obj.Index)
	}

	// Check some fields.
	m := objectMap(t, obj)
	if m["id"] != "item_1" {
		t.Errorf("id = %v, want item_1", m["id"])
	}
	if m["name"] != "Alice" {
		t.Errorf("name = %v, want Alice", m["name"])
	}
	// age is a float64 after JSON unmarshal.
	if m["age"] != float64(30) {
		t.Errorf("age = %v, want 30", m["age"])
	}
}

// ---------------------------------------------------------------------------
// TestReadJSONArray
// ---------------------------------------------------------------------------

func TestReadJSONArray(t *testing.T) {
	r, err := NewReader([]string{testdataPath("items.json")}, ReaderOpts{})
	if err != nil {
		t.Fatalf("NewReader: %v", err)
	}

	objs := collectAll(t, r)

	if len(objs) != 5 {
		t.Fatalf("expected 5 objects, got %d", len(objs))
	}

	expectedNames := []string{"Alice", "Bob", "Charlie", "Diana", "Eve"}
	for i, obj := range objs {
		if obj.Index != i {
			t.Errorf("objs[%d].Index = %d, want %d", i, obj.Index, i)
		}
		m := objectMap(t, obj)
		if m["name"] != expectedNames[i] {
			t.Errorf("objs[%d] name = %v, want %s", i, m["name"], expectedNames[i])
		}
		if !strings.HasSuffix(obj.File, "items.json") {
			t.Errorf("objs[%d].File = %q, want suffix items.json", i, obj.File)
		}
	}
}

// ---------------------------------------------------------------------------
// TestReadJSONL
// ---------------------------------------------------------------------------

func TestReadJSONL(t *testing.T) {
	r, err := NewReader([]string{testdataPath("runs.jsonl")}, ReaderOpts{})
	if err != nil {
		t.Fatalf("NewReader: %v", err)
	}

	objs := collectAll(t, r)

	if len(objs) != 10 {
		t.Fatalf("expected 10 objects, got %d", len(objs))
	}

	// Sequential indices.
	for i, obj := range objs {
		if obj.Index != i {
			t.Errorf("objs[%d].Index = %d, want %d", i, obj.Index, i)
		}
	}

	// First object.
	first := objectMap(t, objs[0])
	if first["id"] != "run_001" {
		t.Errorf("first id = %v, want run_001", first["id"])
	}

	// Last object.
	last := objectMap(t, objs[9])
	if last["id"] != "run_010" {
		t.Errorf("last id = %v, want run_010", last["id"])
	}

	// File metadata.
	for _, obj := range objs {
		if !strings.HasSuffix(obj.File, "runs.jsonl") {
			t.Errorf("File = %q, want suffix runs.jsonl", obj.File)
		}
	}
}

// ---------------------------------------------------------------------------
// TestReadMultipleFiles
// ---------------------------------------------------------------------------

func TestReadMultipleFiles(t *testing.T) {
	r, err := NewReader([]string{
		testdataPath("items.json"),
		testdataPath("runs.jsonl"),
	}, ReaderOpts{})
	if err != nil {
		t.Fatalf("NewReader: %v", err)
	}

	objs := collectAll(t, r)

	if len(objs) != 15 {
		t.Fatalf("expected 15 objects (5+10), got %d", len(objs))
	}

	// First 5 from items.json.
	for i := 0; i < 5; i++ {
		if !strings.HasSuffix(objs[i].File, "items.json") {
			t.Errorf("objs[%d].File = %q, want suffix items.json", i, objs[i].File)
		}
		if objs[i].Index != i {
			t.Errorf("objs[%d].Index = %d, want %d", i, objs[i].Index, i)
		}
	}

	// Next 10 from runs.jsonl — index resets per file.
	for i := 5; i < 15; i++ {
		if !strings.HasSuffix(objs[i].File, "runs.jsonl") {
			t.Errorf("objs[%d].File = %q, want suffix runs.jsonl", i, objs[i].File)
		}
		expectedIdx := i - 5
		if objs[i].Index != expectedIdx {
			t.Errorf("objs[%d].Index = %d, want %d", i, objs[i].Index, expectedIdx)
		}
	}
}

// ---------------------------------------------------------------------------
// TestReadStdin
// ---------------------------------------------------------------------------

func TestReadStdin(t *testing.T) {
	// To simulate stdin we write a temp file and pass "-" as the filename,
	// replacing os.Stdin with our file for the duration of the test.
	content := "{\"x\":1}\n{\"x\":2}\n{\"x\":3}\n"
	tmp := writeTempFile(t, "stdin.jsonl", content)

	f, err := os.Open(tmp)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()

	origStdin := os.Stdin
	os.Stdin = f
	defer func() { os.Stdin = origStdin }()

	r, err := NewReader([]string{"-"}, ReaderOpts{})
	if err != nil {
		t.Fatalf("NewReader: %v", err)
	}

	objs := collectAll(t, r)

	if len(objs) != 3 {
		t.Fatalf("expected 3 objects, got %d", len(objs))
	}

	for _, obj := range objs {
		if obj.File != "stdin" {
			t.Errorf("File = %q, want stdin", obj.File)
		}
	}
}

// ---------------------------------------------------------------------------
// TestReadMalformed
// ---------------------------------------------------------------------------

func TestReadMalformed(t *testing.T) {
	t.Run("default skips bad lines", func(t *testing.T) {
		content := "{\"a\":1}\nNOT JSON\n{\"b\":2}\n"
		path := writeTempFile(t, "bad.jsonl", content)

		r, err := NewReader([]string{path}, ReaderOpts{})
		if err != nil {
			t.Fatalf("NewReader: %v", err)
		}

		var objs []*Object
		for {
			obj, err := r.Next()
			if err != nil {
				t.Fatalf("unexpected error in default mode: %v", err)
			}
			if obj == nil {
				break
			}
			objs = append(objs, obj)
		}

		if len(objs) != 2 {
			t.Fatalf("expected 2 objects (skipping bad line), got %d", len(objs))
		}
	})

	t.Run("strict mode errors on bad line", func(t *testing.T) {
		content := "{\"a\":1}\nNOT JSON\n{\"b\":2}\n"
		path := writeTempFile(t, "bad_strict.jsonl", content)

		r, err := NewReader([]string{path}, ReaderOpts{Strict: true})
		if err != nil {
			t.Fatalf("NewReader: %v", err)
		}

		// First object should succeed.
		obj, err := r.Next()
		if err != nil {
			t.Fatalf("first Next: unexpected error: %v", err)
		}
		if obj == nil {
			t.Fatal("first Next: expected object, got nil")
		}

		// Second call should return an error for the malformed line.
		_, err = r.Next()
		if err == nil {
			t.Fatal("expected error for malformed line in strict mode, got nil")
		}
	})

	t.Run("silent mode suppresses warnings", func(t *testing.T) {
		content := "{\"a\":1}\nNOT JSON\n{\"b\":2}\n"
		path := writeTempFile(t, "bad_silent.jsonl", content)

		r, err := NewReader([]string{path}, ReaderOpts{Silent: true})
		if err != nil {
			t.Fatalf("NewReader: %v", err)
		}

		objs := collectAll(t, r)
		if len(objs) != 2 {
			t.Fatalf("expected 2 objects, got %d", len(objs))
		}
	})

	t.Run("MaxErrors aborts after N errors", func(t *testing.T) {
		content := "{\"a\":1}\nBAD1\nBAD2\nBAD3\n{\"b\":2}\n"
		path := writeTempFile(t, "bad_max.jsonl", content)

		r, err := NewReader([]string{path}, ReaderOpts{MaxErrors: 2})
		if err != nil {
			t.Fatalf("NewReader: %v", err)
		}

		// First object OK.
		obj, err := r.Next()
		if err != nil {
			t.Fatalf("first Next: %v", err)
		}
		if obj == nil {
			t.Fatal("first Next: expected object")
		}

		// Continue reading — should eventually get an error after 2 bad lines.
		var gotErr bool
		for {
			obj, err = r.Next()
			if err != nil {
				gotErr = true
				break
			}
			if obj == nil {
				break
			}
		}

		if !gotErr {
			t.Fatal("expected error after MaxErrors reached, got EOF")
		}
	})
}

// ---------------------------------------------------------------------------
// TestReadEmpty
// ---------------------------------------------------------------------------

func TestReadEmpty(t *testing.T) {
	t.Run("empty file", func(t *testing.T) {
		path := writeTempFile(t, "empty.json", "")

		r, err := NewReader([]string{path}, ReaderOpts{})
		if err != nil {
			t.Fatalf("NewReader: %v", err)
		}

		objs := collectAll(t, r)
		if len(objs) != 0 {
			t.Fatalf("expected 0 objects, got %d", len(objs))
		}
	})

	t.Run("whitespace only", func(t *testing.T) {
		path := writeTempFile(t, "ws.json", "   \n\t\n  \n")

		r, err := NewReader([]string{path}, ReaderOpts{})
		if err != nil {
			t.Fatalf("NewReader: %v", err)
		}

		objs := collectAll(t, r)
		if len(objs) != 0 {
			t.Fatalf("expected 0 objects, got %d", len(objs))
		}
	})
}

// ---------------------------------------------------------------------------
// TestReadLargeObject
// ---------------------------------------------------------------------------

func TestReadLargeObject(t *testing.T) {
	t.Run("many fields", func(t *testing.T) {
		obj := make(map[string]any)
		for i := 0; i < 200; i++ {
			obj[fmt.Sprintf("field_%03d", i)] = i
		}
		data, _ := json.Marshal(obj)
		path := writeTempFile(t, "large.json", string(data))

		r, err := NewReader([]string{path}, ReaderOpts{})
		if err != nil {
			t.Fatalf("NewReader: %v", err)
		}

		objs := collectAll(t, r)
		if len(objs) != 1 {
			t.Fatalf("expected 1 object, got %d", len(objs))
		}

		m := objectMap(t, objs[0])
		if len(m) != 200 {
			t.Errorf("expected 200 fields, got %d", len(m))
		}
	})

	t.Run("deeply nested 10 levels", func(t *testing.T) {
		// Build {"l0":{"l1":{..."l9":"deep"}...}}
		var nested any = "deep"
		for i := 9; i >= 0; i-- {
			nested = map[string]any{fmt.Sprintf("l%d", i): nested}
		}
		data, _ := json.Marshal(nested)
		path := writeTempFile(t, "nested.json", string(data))

		r, err := NewReader([]string{path}, ReaderOpts{})
		if err != nil {
			t.Fatalf("NewReader: %v", err)
		}

		objs := collectAll(t, r)
		if len(objs) != 1 {
			t.Fatalf("expected 1 object, got %d", len(objs))
		}

		// Walk down 10 levels.
		var cur any = objs[0].Value
		for i := 0; i <= 9; i++ {
			m, ok := cur.(map[string]any)
			if !ok {
				t.Fatalf("level %d: expected map, got %T", i, cur)
			}
			key := fmt.Sprintf("l%d", i)
			cur, ok = m[key]
			if !ok {
				t.Fatalf("level %d: key %q not found", i, key)
			}
		}
		if cur != "deep" {
			t.Errorf("leaf value = %v, want deep", cur)
		}
	})
}

// ---------------------------------------------------------------------------
// TestReadStreaming
// ---------------------------------------------------------------------------

func TestReadStreaming(t *testing.T) {
	const lineCount = 10000

	var b strings.Builder
	for i := 0; i < lineCount; i++ {
		fmt.Fprintf(&b, "{\"i\":%d}\n", i)
	}
	path := writeTempFile(t, "large.jsonl", b.String())

	r, err := NewReader([]string{path}, ReaderOpts{})
	if err != nil {
		t.Fatalf("NewReader: %v", err)
	}

	count := 0
	for {
		obj, err := r.Next()
		if err != nil {
			t.Fatalf("Next error at count %d: %v", count, err)
		}
		if obj == nil {
			break
		}
		count++
	}

	if count != lineCount {
		t.Fatalf("expected %d objects, got %d", lineCount, count)
	}
}

// ---------------------------------------------------------------------------
// TestReadFormatOverride
// ---------------------------------------------------------------------------

func TestReadFormatOverride(t *testing.T) {
	// Force JSONL format on a file that contains one object per line.
	content := "{\"a\":1}\n{\"b\":2}\n"
	path := writeTempFile(t, "forced.txt", content)

	r, err := NewReader([]string{path}, ReaderOpts{Format: FormatJSONL})
	if err != nil {
		t.Fatalf("NewReader: %v", err)
	}

	objs := collectAll(t, r)
	if len(objs) != 2 {
		t.Fatalf("expected 2 objects with JSONL override, got %d", len(objs))
	}
}
