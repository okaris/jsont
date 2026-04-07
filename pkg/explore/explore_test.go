package explore

import (
	"bufio"
	"bytes"
	"encoding/json"
	"os"
	"sort"
	"strings"
	"testing"
)

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

func loadJSONL(t *testing.T, path string) []any {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("failed to read %s: %v", path, err)
	}
	var objects []any
	scanner := bufio.NewScanner(bytes.NewReader(data))
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		var obj any
		if err := json.Unmarshal([]byte(line), &obj); err != nil {
			t.Fatalf("failed to unmarshal line %q: %v", line, err)
		}
		objects = append(objects, obj)
	}
	if err := scanner.Err(); err != nil {
		t.Fatalf("scanner error: %v", err)
	}
	return objects
}

func schemaFieldByPath(fields []SchemaField, path string) *SchemaField {
	for i := range fields {
		if fields[i].Path == path {
			return &fields[i]
		}
	}
	return nil
}

func containsString(slice []string, s string) bool {
	for _, v := range slice {
		if v == s {
			return true
		}
	}
	return false
}

func objectsEqual(a, b any) bool {
	aj, _ := json.Marshal(a)
	bj, _ := json.Marshal(b)
	return string(aj) == string(bj)
}

func containsObject(haystack []any, needle any) bool {
	for _, h := range haystack {
		if objectsEqual(h, needle) {
			return true
		}
	}
	return false
}

// ---------------------------------------------------------------------------
// TestSchema
// ---------------------------------------------------------------------------

func TestSchema(t *testing.T) {
	runs := loadJSONL(t, "../../testdata/runs.jsonl")

	t.Run("simple flat objects all fields present", func(t *testing.T) {
		// Every object has .id, .model, .status, .latency_ms
		schema := InferSchema(runs, SchemaOpts{MaxExamples: 3})

		for _, path := range []string{".id", ".model", ".status", ".latency_ms"} {
			f := schemaFieldByPath(schema, path)
			if f == nil {
				t.Fatalf("expected field %s in schema", path)
			}
			if f.Frequency != 1.0 {
				t.Errorf("field %s: expected frequency 1.0, got %f", path, f.Frequency)
			}
		}

		idField := schemaFieldByPath(schema, ".id")
		if !containsString(idField.Types, "string") {
			t.Errorf(".id types should contain 'string', got %v", idField.Types)
		}

		latField := schemaFieldByPath(schema, ".latency_ms")
		if !containsString(latField.Types, "number") {
			t.Errorf(".latency_ms types should contain 'number', got %v", latField.Types)
		}
	})

	t.Run("mixed presence frequency less than 1", func(t *testing.T) {
		// .error only on some objects (run_002, run_004, run_008)
		schema := InferSchema(runs, SchemaOpts{})
		errMsg := schemaFieldByPath(schema, ".error.message")
		if errMsg == nil {
			t.Fatal("expected .error.message in schema")
		}
		if errMsg.Frequency >= 1.0 {
			t.Errorf(".error.message frequency should be < 1.0, got %f", errMsg.Frequency)
		}
		if errMsg.Frequency <= 0.0 {
			t.Errorf(".error.message frequency should be > 0.0, got %f", errMsg.Frequency)
		}
	})

	t.Run("mixed types", func(t *testing.T) {
		// .message.content is string on most, but array on run_007
		schema := InferSchema(runs, SchemaOpts{})
		content := schemaFieldByPath(schema, ".message.content")
		if content == nil {
			t.Fatal("expected .message.content in schema")
		}
		if len(content.Types) < 2 {
			t.Errorf(".message.content should have mixed types (string and array), got %v", content.Types)
		}
		if !containsString(content.Types, "string") {
			t.Errorf(".message.content types should include 'string', got %v", content.Types)
		}
		if !containsString(content.Types, "array") {
			t.Errorf(".message.content types should include 'array', got %v", content.Types)
		}
	})

	t.Run("nested objects", func(t *testing.T) {
		schema := InferSchema(runs, SchemaOpts{})
		region := schemaFieldByPath(schema, ".metadata.region")
		if region == nil {
			t.Fatal("expected .metadata.region in schema")
		}
		if !containsString(region.Types, "string") {
			t.Errorf(".metadata.region should be string, got %v", region.Types)
		}
	})

	t.Run("array fields", func(t *testing.T) {
		schema := InferSchema(runs, SchemaOpts{})
		tags := schemaFieldByPath(schema, ".metadata.tags")
		if tags == nil {
			t.Fatal("expected .metadata.tags in schema")
		}
		hasArrayType := false
		for _, typ := range tags.Types {
			if strings.HasPrefix(typ, "array") {
				hasArrayType = true
			}
		}
		if !hasArrayType {
			t.Errorf(".metadata.tags should have an array type, got %v", tags.Types)
		}
	})

	t.Run("all null field", func(t *testing.T) {
		// Construct objects where a field is always null
		objects := []any{
			map[string]any{"a": "x", "b": nil},
			map[string]any{"a": "y", "b": nil},
			map[string]any{"a": "z", "b": nil},
		}
		schema := InferSchema(objects, SchemaOpts{})
		bField := schemaFieldByPath(schema, ".b")
		if bField == nil {
			t.Fatal("expected .b in schema")
		}
		if !containsString(bField.Types, "null") {
			t.Errorf(".b types should contain 'null', got %v", bField.Types)
		}
		if bField.Frequency != 1.0 {
			t.Errorf(".b frequency should be 1.0, got %f", bField.Frequency)
		}
	})

	t.Run("empty input", func(t *testing.T) {
		schema := InferSchema([]any{}, SchemaOpts{})
		if len(schema) != 0 {
			t.Errorf("expected empty schema for empty input, got %d fields", len(schema))
		}
	})

	t.Run("single object", func(t *testing.T) {
		objects := []any{
			map[string]any{"name": "test", "value": float64(42)},
		}
		schema := InferSchema(objects, SchemaOpts{})
		if len(schema) < 2 {
			t.Errorf("expected at least 2 fields, got %d", len(schema))
		}
		name := schemaFieldByPath(schema, ".name")
		if name == nil {
			t.Fatal("expected .name in schema")
		}
		if name.Frequency != 1.0 {
			t.Errorf(".name frequency should be 1.0 for single object, got %f", name.Frequency)
		}
	})
}

// ---------------------------------------------------------------------------
// TestTree
// ---------------------------------------------------------------------------

func TestTree(t *testing.T) {
	runs := loadJSONL(t, "../../testdata/runs.jsonl")

	t.Run("flat object root with direct children", func(t *testing.T) {
		objects := []any{
			map[string]any{"a": "x", "b": float64(1)},
		}
		tree := BuildTree(objects, TreeOpts{})
		if tree == nil {
			t.Fatal("expected non-nil tree")
		}
		if len(tree.Children) < 2 {
			t.Errorf("expected at least 2 children, got %d", len(tree.Children))
		}
	})

	t.Run("nested proper tree hierarchy", func(t *testing.T) {
		tree := BuildTree(runs, TreeOpts{})
		if tree == nil {
			t.Fatal("expected non-nil tree")
		}
		// Find .message child
		var msgNode *TreeNode
		for _, c := range tree.Children {
			if c.Path == "message" || c.Path == ".message" {
				msgNode = c
				break
			}
		}
		if msgNode == nil {
			t.Fatal("expected 'message' child node")
		}
		if len(msgNode.Children) == 0 {
			t.Error("expected message node to have children (role, content)")
		}
	})

	t.Run("optional fields marked correctly", func(t *testing.T) {
		tree := BuildTree(runs, TreeOpts{})
		// .error is not on every object, should be optional
		var errNode *TreeNode
		for _, c := range tree.Children {
			if c.Path == "error" || c.Path == ".error" {
				errNode = c
				break
			}
		}
		if errNode == nil {
			t.Fatal("expected 'error' child node")
		}
		if !errNode.Optional {
			t.Error("error node should be marked optional")
		}
	})

	t.Run("array types detected", func(t *testing.T) {
		tree := BuildTree(runs, TreeOpts{})
		// Find .metadata.tags — should show array type
		var metaNode *TreeNode
		for _, c := range tree.Children {
			if c.Path == "metadata" || c.Path == ".metadata" {
				metaNode = c
				break
			}
		}
		if metaNode == nil {
			t.Fatal("expected metadata node")
		}
		var tagsNode *TreeNode
		for _, c := range metaNode.Children {
			if c.Path == "tags" || c.Path == ".tags" || c.Path == "metadata.tags" || c.Path == ".metadata.tags" {
				tagsNode = c
				break
			}
		}
		if tagsNode == nil {
			t.Fatal("expected tags node under metadata")
		}
		if !strings.Contains(tagsNode.Type, "array") {
			t.Errorf("tags type should contain 'array', got %q", tagsNode.Type)
		}
	})

	t.Run("mixed types at same path", func(t *testing.T) {
		tree := BuildTree(runs, TreeOpts{})
		// .message.content is string on some, array on run_007
		var msgNode *TreeNode
		for _, c := range tree.Children {
			if c.Path == "message" || c.Path == ".message" {
				msgNode = c
				break
			}
		}
		if msgNode == nil {
			t.Fatal("expected message node")
		}
		var contentNode *TreeNode
		for _, c := range msgNode.Children {
			if c.Path == "content" || c.Path == ".content" || c.Path == "message.content" || c.Path == ".message.content" {
				contentNode = c
				break
			}
		}
		if contentNode == nil {
			t.Fatal("expected content node under message")
		}
		if !strings.Contains(contentNode.Type, "|") {
			t.Errorf("content type should show mixed types with '|', got %q", contentNode.Type)
		}
	})
}

// ---------------------------------------------------------------------------
// TestFields
// ---------------------------------------------------------------------------

func TestFields(t *testing.T) {
	runs := loadJSONL(t, "../../testdata/runs.jsonl")

	t.Run("returns all unique dot-paths sorted", func(t *testing.T) {
		fields := ListFields(runs)
		if len(fields) == 0 {
			t.Fatal("expected non-empty fields list")
		}
		// Must be sorted
		if !sort.StringsAreSorted(fields) {
			t.Error("fields should be sorted")
		}
	})

	t.Run("includes nested paths", func(t *testing.T) {
		fields := ListFields(runs)
		found := false
		for _, f := range fields {
			if f == ".metadata.region" || f == "metadata.region" {
				found = true
				break
			}
		}
		if !found {
			t.Error("expected .metadata.region in fields list")
		}
	})

	t.Run("includes array element paths", func(t *testing.T) {
		nested := loadJSONL(t, "../../testdata/nested.jsonl")
		fields := ListFields(nested)
		found := false
		for _, f := range fields {
			if strings.Contains(f, "items[]") || strings.Contains(f, "items[].name") {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("expected array element paths like items[].name, got %v", fields)
		}
	})

	t.Run("no duplicates", func(t *testing.T) {
		fields := ListFields(runs)
		seen := map[string]bool{}
		for _, f := range fields {
			if seen[f] {
				t.Errorf("duplicate field: %s", f)
			}
			seen[f] = true
		}
	})
}

// ---------------------------------------------------------------------------
// TestFind
// ---------------------------------------------------------------------------

func TestFind(t *testing.T) {
	runs := loadJSONL(t, "../../testdata/runs.jsonl")
	nested := loadJSONL(t, "../../testdata/nested.jsonl")

	t.Run("simple string match", func(t *testing.T) {
		results := Find(runs, "timeout", FindOpts{CaseInsensitive: true})
		if len(results) == 0 {
			t.Fatal("expected at least one match for 'timeout'")
		}
		for _, r := range results {
			if !strings.Contains(strings.ToLower(r.Value), "timeout") {
				t.Errorf("result value %q does not contain 'timeout'", r.Value)
			}
		}
	})

	t.Run("case insensitive by default", func(t *testing.T) {
		upper := Find(runs, "TIMEOUT", FindOpts{CaseInsensitive: true})
		lower := Find(runs, "timeout", FindOpts{CaseInsensitive: true})
		if len(upper) != len(lower) {
			t.Errorf("case-insensitive search should return same results: upper=%d, lower=%d", len(upper), len(lower))
		}
	})

	t.Run("finds in nested fields", func(t *testing.T) {
		results := Find(nested, "hidden failure", FindOpts{CaseInsensitive: true})
		if len(results) == 0 {
			t.Fatal("expected match in deeply nested field")
		}
		foundDeep := false
		for _, r := range results {
			if strings.Contains(r.Path, "deep") || strings.Contains(r.Path, "nested") {
				foundDeep = true
			}
		}
		if !foundDeep {
			t.Error("expected match path to reference deep/nested path")
		}
	})

	t.Run("finds across multiple objects", func(t *testing.T) {
		// "timeout" appears in error messages of multiple runs
		results := Find(runs, "timeout", FindOpts{CaseInsensitive: true})
		indices := map[int]bool{}
		for _, r := range results {
			indices[r.Index] = true
		}
		if len(indices) < 2 {
			t.Errorf("expected matches in multiple objects, found in %d", len(indices))
		}
	})

	t.Run("reports correct path for each match", func(t *testing.T) {
		results := Find(runs, "connection timeout", FindOpts{CaseInsensitive: true})
		if len(results) == 0 {
			t.Fatal("expected at least one match")
		}
		for _, r := range results {
			if r.Path == "" {
				t.Error("result path should not be empty")
			}
			if !strings.Contains(r.Path, "error") && !strings.Contains(r.Path, "message") {
				t.Errorf("expected path containing 'error' or 'message', got %q", r.Path)
			}
		}
	})

	t.Run("regex mode", func(t *testing.T) {
		results := Find(runs, `run_00[1-3]`, FindOpts{Regex: true})
		if len(results) == 0 {
			t.Fatal("expected regex matches for run_00[1-3]")
		}
		for _, r := range results {
			if r.Value != "run_001" && r.Value != "run_002" && r.Value != "run_003" {
				t.Errorf("unexpected match value: %q", r.Value)
			}
		}
	})

	t.Run("keys mode searches key names", func(t *testing.T) {
		results := Find(runs, "latency", FindOpts{Keys: true})
		if len(results) == 0 {
			t.Fatal("expected matches when searching key names for 'latency'")
		}
		for _, r := range results {
			if !strings.Contains(strings.ToLower(r.Path), "latency") {
				t.Errorf("key search result path should contain 'latency', got %q", r.Path)
			}
		}
	})

	t.Run("first N limits results", func(t *testing.T) {
		all := Find(runs, "prod", FindOpts{CaseInsensitive: true})
		limited := Find(runs, "prod", FindOpts{CaseInsensitive: true, First: 2})
		if len(all) <= 2 {
			t.Skip("not enough matches to test --first limit")
		}
		if len(limited) != 2 {
			t.Errorf("expected exactly 2 results with First=2, got %d", len(limited))
		}
	})

	t.Run("no matches returns empty", func(t *testing.T) {
		results := Find(runs, "zzz_nonexistent_pattern_zzz", FindOpts{CaseInsensitive: true})
		if len(results) != 0 {
			t.Errorf("expected 0 results, got %d", len(results))
		}
	})
}

// ---------------------------------------------------------------------------
// TestStats
// ---------------------------------------------------------------------------

func TestStats(t *testing.T) {
	runs := loadJSONL(t, "../../testdata/runs.jsonl")

	t.Run("correct total count", func(t *testing.T) {
		stats := ComputeStats(runs)
		if stats.Count != 10 {
			t.Errorf("expected count 10, got %d", stats.Count)
		}
	})

	t.Run("numeric fields min max mean", func(t *testing.T) {
		stats := ComputeStats(runs)
		lat, ok := stats.NumericStats[".latency_ms"]
		if !ok {
			// Try without leading dot
			lat, ok = stats.NumericStats["latency_ms"]
		}
		if !ok {
			t.Fatal("expected numeric stats for latency_ms")
		}
		if lat.Min != 0 {
			t.Errorf("latency_ms min should be 0, got %f", lat.Min)
		}
		if lat.Max != 8320 {
			t.Errorf("latency_ms max should be 8320, got %f", lat.Max)
		}
		// Mean of 142+3201+89+5400+201+0+178+8320+156+312 = 17999 / 10 = 1799.9
		expectedMean := 1799.9
		if lat.Mean < expectedMean-0.5 || lat.Mean > expectedMean+0.5 {
			t.Errorf("latency_ms mean should be ~%.1f, got %f", expectedMean, lat.Mean)
		}
	})

	t.Run("string fields unique counts and top values", func(t *testing.T) {
		stats := ComputeStats(runs)
		statusStat, ok := stats.StringStats[".status"]
		if !ok {
			statusStat, ok = stats.StringStats["status"]
		}
		if !ok {
			t.Fatal("expected string stats for status")
		}
		// Unique status values: ok, failed, pending -> 3
		if statusStat.Unique != 3 {
			t.Errorf("status unique count should be 3, got %d", statusStat.Unique)
		}
		// ok appears 6 times, failed 3, pending 1
		if statusStat.TopValues != nil {
			if count, exists := statusStat.TopValues["ok"]; exists && count != 6 {
				t.Errorf("status 'ok' count should be 6, got %d", count)
			}
		}
	})

	t.Run("null and missing counts", func(t *testing.T) {
		stats := ComputeStats(runs)
		// .metadata is null on run_006
		metaNull, ok := stats.NullCounts[".metadata"]
		if !ok {
			metaNull, ok = stats.NullCounts["metadata"]
		}
		if !ok {
			t.Fatal("expected null count for metadata")
		}
		if metaNull < 1 {
			t.Errorf("expected at least 1 null for metadata, got %d", metaNull)
		}
	})

	t.Run("schema count detects different shapes", func(t *testing.T) {
		stats := ComputeStats(runs)
		// Objects have different shapes (some have .error, some have null metadata, etc.)
		if stats.SchemaCount < 2 {
			t.Errorf("expected at least 2 distinct shapes, got %d", stats.SchemaCount)
		}
	})
}

// ---------------------------------------------------------------------------
// TestHead
// ---------------------------------------------------------------------------

func TestHead(t *testing.T) {
	runs := loadJSONL(t, "../../testdata/runs.jsonl")

	t.Run("head 3 returns first 3", func(t *testing.T) {
		result := Head(runs, 3)
		if len(result) != 3 {
			t.Errorf("expected 3 items, got %d", len(result))
		}
		for i := 0; i < 3; i++ {
			if !objectsEqual(result[i], runs[i]) {
				t.Errorf("item %d doesn't match", i)
			}
		}
	})

	t.Run("head 0 returns empty", func(t *testing.T) {
		result := Head(runs, 0)
		if len(result) != 0 {
			t.Errorf("expected 0 items, got %d", len(result))
		}
	})

	t.Run("head greater than len returns all", func(t *testing.T) {
		result := Head(runs, 100)
		if len(result) != len(runs) {
			t.Errorf("expected %d items, got %d", len(runs), len(result))
		}
	})

	t.Run("head 1 returns first", func(t *testing.T) {
		result := Head(runs, 1)
		if len(result) != 1 {
			t.Errorf("expected 1 item, got %d", len(result))
		}
		if !objectsEqual(result[0], runs[0]) {
			t.Error("first item doesn't match")
		}
	})
}

// ---------------------------------------------------------------------------
// TestTail
// ---------------------------------------------------------------------------

func TestTail(t *testing.T) {
	runs := loadJSONL(t, "../../testdata/runs.jsonl")

	t.Run("tail 3 returns last 3", func(t *testing.T) {
		result := Tail(runs, 3)
		if len(result) != 3 {
			t.Errorf("expected 3 items, got %d", len(result))
		}
		n := len(runs)
		for i := 0; i < 3; i++ {
			if !objectsEqual(result[i], runs[n-3+i]) {
				t.Errorf("item %d doesn't match", i)
			}
		}
	})

	t.Run("tail greater than len returns all", func(t *testing.T) {
		result := Tail(runs, 100)
		if len(result) != len(runs) {
			t.Errorf("expected %d items, got %d", len(runs), len(result))
		}
	})
}

// ---------------------------------------------------------------------------
// TestCount
// ---------------------------------------------------------------------------

func TestCount(t *testing.T) {
	runs := loadJSONL(t, "../../testdata/runs.jsonl")

	t.Run("returns correct count", func(t *testing.T) {
		c := Count(runs)
		if c != 10 {
			t.Errorf("expected 10, got %d", c)
		}
	})

	t.Run("empty input returns 0", func(t *testing.T) {
		c := Count([]any{})
		if c != 0 {
			t.Errorf("expected 0, got %d", c)
		}
	})
}

// ---------------------------------------------------------------------------
// TestSample
// ---------------------------------------------------------------------------

func TestSample(t *testing.T) {
	runs := loadJSONL(t, "../../testdata/runs.jsonl")

	t.Run("returns exactly N items", func(t *testing.T) {
		result := Sample(runs, 3)
		if len(result) != 3 {
			t.Errorf("expected 3 items, got %d", len(result))
		}
	})

	t.Run("returns all if N >= total", func(t *testing.T) {
		result := Sample(runs, 100)
		if len(result) != len(runs) {
			t.Errorf("expected %d items, got %d", len(runs), len(result))
		}
	})

	t.Run("each returned item exists in original", func(t *testing.T) {
		result := Sample(runs, 5)
		for i, item := range result {
			if !containsObject(runs, item) {
				t.Errorf("sampled item %d not found in original data", i)
			}
		}
	})

	t.Run("N=0 returns empty", func(t *testing.T) {
		result := Sample(runs, 0)
		if len(result) != 0 {
			t.Errorf("expected 0 items, got %d", len(result))
		}
	})
}
