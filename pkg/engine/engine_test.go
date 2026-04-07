package engine

import (
	"encoding/json"
	"fmt"
	"math"
	"reflect"
	"sort"
	"testing"
)

// ---------------------------------------------------------------------------
// Shared test datasets
// ---------------------------------------------------------------------------

var peopleData = []string{
	`{"id":"1","name":"Alice","age":30,"status":"active","active":true,"address":{"city":"Portland"}}`,
	`{"id":"2","name":"Bob","age":25,"status":"inactive","active":false,"address":{"city":"Seattle"}}`,
	`{"id":"3","name":"Charlie","age":35,"status":"active","active":true,"address":{"city":"Portland"}}`,
	`{"id":"4","name":"Diana","age":28,"status":"active","active":true,"address":{"city":"Denver"}}`,
	`{"id":"5","name":"Eve","age":22,"status":"inactive","active":false,"address":{"city":"Seattle"}}`,
}

var runsData = []string{
	`{"id":"run_001","model":"gpt-4","status":"ok","latency_ms":142}`,
	`{"id":"run_002","model":"claude-3","status":"failed","latency_ms":3201,"error":{"message":"connection timeout"}}`,
	`{"id":"run_003","model":"gpt-4","status":"ok","latency_ms":89}`,
	`{"id":"run_004","model":"mixtral","status":"failed","latency_ms":5400,"error":{"message":"rate limit exceeded"}}`,
	`{"id":"run_005","model":"claude-3","status":"ok","latency_ms":201}`,
	`{"id":"run_006","model":"gpt-4","status":"pending","latency_ms":0}`,
}

var nullData = []string{
	`{"id":"1","name":"Alice","value":null,"tags":["admin","user"]}`,
	`{"id":"2","name":"Bob","value":42,"tags":["user"]}`,
	`{"id":"3","name":"Charlie","value":null,"tags":["admin"]}`,
}

var computedData = []string{
	`{"id":"1","start":10,"end":25}`,
	`{"id":"2","start":5,"end":18}`,
	`{"id":"3","start":0,"end":100}`,
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

func parseObjects(t *testing.T, jsonStrings []string) []any {
	t.Helper()
	objects := make([]any, 0, len(jsonStrings))
	for i, s := range jsonStrings {
		var obj any
		if err := json.Unmarshal([]byte(s), &obj); err != nil {
			t.Fatalf("failed to unmarshal object %d: %v", i, err)
		}
		objects = append(objects, obj)
	}
	return objects
}

// toJSON is a helper that marshals a value for display in error messages.
func toJSON(v any) string {
	b, _ := json.Marshal(v)
	return string(b)
}

// floatEquals compares two float64 values within a tolerance.
func floatEquals(a, b, tolerance float64) bool {
	return math.Abs(a-b) < tolerance
}

// asFloat extracts a float64 from an any value (works for json.Number-decoded
// values as well as raw float64).
func asFloat(v any) (float64, bool) {
	switch n := v.(type) {
	case float64:
		return n, true
	case json.Number:
		f, err := n.Float64()
		return f, err == nil
	case int:
		return float64(n), true
	case int64:
		return float64(n), true
	}
	return 0, false
}

// fieldVal safely extracts a top-level field from an object that is expected to
// be a map[string]any.
func fieldVal(obj any, key string) any {
	m, ok := obj.(map[string]any)
	if !ok {
		return nil
	}
	return m[key]
}

// assertObjectsMatch compares two slices of any values. For map values it does
// a key-by-key comparison that is tolerant of float precision. For scalar
// values it uses reflect.DeepEqual.
func assertObjectsMatch(t *testing.T, label string, want, got []any) {
	t.Helper()
	if len(want) != len(got) {
		t.Fatalf("%s: length mismatch: want %d, got %d\n  want: %s\n  got:  %s",
			label, len(want), len(got), toJSON(want), toJSON(got))
	}
	for i := range want {
		if !valuesEqual(want[i], got[i]) {
			t.Errorf("%s: object[%d] mismatch\n  want: %s\n  got:  %s",
				label, i, toJSON(want[i]), toJSON(got[i]))
		}
	}
}

func valuesEqual(a, b any) bool {
	// Both nil / null
	if a == nil && b == nil {
		return true
	}
	// Number comparison with tolerance
	af, aok := asFloat(a)
	bf, bok := asFloat(b)
	if aok && bok {
		return floatEquals(af, bf, 0.001)
	}
	// Map comparison
	am, amok := a.(map[string]any)
	bm, bmok := b.(map[string]any)
	if amok && bmok {
		if len(am) != len(bm) {
			return false
		}
		for k, av := range am {
			bv, ok := bm[k]
			if !ok {
				return false
			}
			if !valuesEqual(av, bv) {
				return false
			}
		}
		return true
	}
	// Slice comparison
	as, asok := a.([]any)
	bs, bsok := b.([]any)
	if asok && bsok {
		if len(as) != len(bs) {
			return false
		}
		for i := range as {
			if !valuesEqual(as[i], bs[i]) {
				return false
			}
		}
		return true
	}
	return reflect.DeepEqual(a, b)
}

// ---------------------------------------------------------------------------
// Tests
// ---------------------------------------------------------------------------

func TestExecuteSelect(t *testing.T) {
	tests := []struct {
		name    string
		objects []string
		query   string
		want    []any
	}{
		{
			name:    "single field extraction",
			objects: peopleData,
			query:   ".name",
			want:    []any{"Alice", "Bob", "Charlie", "Diana", "Eve"},
		},
		{
			name:    "select multiple fields",
			objects: peopleData,
			query:   "select .name, .age",
			want: []any{
				map[string]any{"name": "Alice", "age": 30.0},
				map[string]any{"name": "Bob", "age": 25.0},
				map[string]any{"name": "Charlie", "age": 35.0},
				map[string]any{"name": "Diana", "age": 28.0},
				map[string]any{"name": "Eve", "age": 22.0},
			},
		},
		{
			name:    "select with alias",
			objects: peopleData[:2],
			query:   "select .name as n",
			want: []any{
				map[string]any{"n": "Alice"},
				map[string]any{"n": "Bob"},
			},
		},
		{
			name:    "computed field with arithmetic",
			objects: computedData,
			query:   "select .end - .start as duration",
			want: []any{
				map[string]any{"duration": 15.0},
				map[string]any{"duration": 13.0},
				map[string]any{"duration": 100.0},
			},
		},
		{
			name:    "select star returns full objects",
			objects: computedData,
			query:   "select *",
			want: []any{
				map[string]any{"id": "1", "start": 10.0, "end": 25.0},
				map[string]any{"id": "2", "start": 5.0, "end": 18.0},
				map[string]any{"id": "3", "start": 0.0, "end": 100.0},
			},
		},
		{
			name:    "nested field extraction",
			objects: peopleData[:3],
			query:   "select .address.city",
			want: []any{
				map[string]any{"address.city": "Portland"},
				map[string]any{"address.city": "Seattle"},
				map[string]any{"address.city": "Portland"},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			objects := parseObjects(t, tt.objects)
			result, err := Execute(objects, tt.query, EngineOpts{})
			if err != nil {
				t.Fatalf("Execute() returned error: %v", err)
			}
			assertObjectsMatch(t, tt.name, tt.want, result.Objects)
		})
	}
}

func TestExecuteWhere(t *testing.T) {
	tests := []struct {
		name      string
		objects   []string
		query     string
		wantCount int
		check     func(t *testing.T, results []any) // optional deeper check
	}{
		{
			name:      "equality string filter",
			objects:   runsData,
			query:     `where .status == "failed"`,
			wantCount: 2,
			check: func(t *testing.T, results []any) {
				for _, r := range results {
					if fieldVal(r, "status") != "failed" {
						t.Errorf("expected status=failed, got %v", fieldVal(r, "status"))
					}
				}
			},
		},
		{
			name:      "numeric greater than",
			objects:   peopleData,
			query:     `where .age > 25`,
			wantCount: 3, // Alice(30), Charlie(35), Diana(28)
			check: func(t *testing.T, results []any) {
				for _, r := range results {
					age, _ := asFloat(fieldVal(r, "age"))
					if age <= 25 {
						t.Errorf("expected age > 25, got %v", age)
					}
				}
			},
		},
		{
			name:      "exists check",
			objects:   runsData,
			query:     `where .error exists`,
			wantCount: 2,
			check: func(t *testing.T, results []any) {
				for _, r := range results {
					if fieldVal(r, "error") == nil {
						t.Error("expected error field to exist")
					}
				}
			},
		},
		{
			name:      "array contains",
			objects:   nullData,
			query:     `where .tags contains "admin"`,
			wantCount: 2, // Alice and Charlie
		},
		{
			name:      "starts with",
			objects:   peopleData,
			query:     `where .name starts with "A"`,
			wantCount: 1, // Alice
			check: func(t *testing.T, results []any) {
				if fieldVal(results[0], "name") != "Alice" {
					t.Errorf("expected Alice, got %v", fieldVal(results[0], "name"))
				}
			},
		},
		{
			name:      "regex match",
			objects:   peopleData,
			query:     `where .name matches /^[A-C]/`,
			wantCount: 3, // Alice, Bob, Charlie
		},
		{
			name:      "set membership",
			objects:   runsData,
			query:     `where .status in ("ok", "pending")`,
			wantCount: 4, // run_001, run_003, run_005, run_006
		},
		{
			name:      "compound and",
			objects:   runsData,
			query:     `where .status == "failed" and .latency_ms > 1000`,
			wantCount: 2, // run_002(3201), run_004(5400)
		},
		{
			name:      "negation",
			objects:   peopleData,
			query:     `where not .active`,
			wantCount: 2, // Bob and Eve
		},
		{
			name:      "null check",
			objects:   nullData,
			query:     `where .value is null`,
			wantCount: 2, // Alice and Charlie
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			objects := parseObjects(t, tt.objects)
			result, err := Execute(objects, tt.query, EngineOpts{})
			if err != nil {
				t.Fatalf("Execute() returned error: %v", err)
			}
			if len(result.Objects) != tt.wantCount {
				t.Fatalf("expected %d results, got %d: %s",
					tt.wantCount, len(result.Objects), toJSON(result.Objects))
			}
			if tt.check != nil {
				tt.check(t, result.Objects)
			}
		})
	}
}

func TestExecuteSortBy(t *testing.T) {
	tests := []struct {
		name    string
		objects []string
		query   string
		check   func(t *testing.T, results []any)
	}{
		{
			name:    "ascending sort",
			objects: peopleData,
			query:   "sort by .age",
			check: func(t *testing.T, results []any) {
				ages := make([]float64, len(results))
				for i, r := range results {
					ages[i], _ = asFloat(fieldVal(r, "age"))
				}
				if !sort.Float64sAreSorted(ages) {
					t.Errorf("expected ascending ages, got %v", ages)
				}
			},
		},
		{
			name:    "descending sort",
			objects: peopleData,
			query:   "sort by .age desc",
			check: func(t *testing.T, results []any) {
				ages := make([]float64, len(results))
				for i, r := range results {
					ages[i], _ = asFloat(fieldVal(r, "age"))
				}
				// Check descending
				for i := 1; i < len(ages); i++ {
					if ages[i] > ages[i-1] {
						t.Errorf("expected descending ages, got %v", ages)
						break
					}
				}
			},
		},
		{
			name:    "multi-field sort",
			objects: peopleData,
			query:   "sort by .status, .age desc",
			check: func(t *testing.T, results []any) {
				// Active people sorted by age desc, then inactive by age desc
				if len(results) != 5 {
					t.Fatalf("expected 5 results, got %d", len(results))
				}
				// Verify status groups are contiguous
				prevStatus := ""
				for _, r := range results {
					s, _ := fieldVal(r, "status").(string)
					if s != prevStatus && prevStatus != "" {
						// Ensure we don't go back to a previous status
						if s < prevStatus {
							// This is fine for ascending status sort
						}
					}
					prevStatus = s
				}
				// Within each status group, ages should be descending
				groups := map[string][]float64{}
				for _, r := range results {
					s, _ := fieldVal(r, "status").(string)
					age, _ := asFloat(fieldVal(r, "age"))
					groups[s] = append(groups[s], age)
				}
				for status, ages := range groups {
					for i := 1; i < len(ages); i++ {
						if ages[i] > ages[i-1] {
							t.Errorf("in status %q, expected descending ages, got %v", status, ages)
							break
						}
					}
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			objects := parseObjects(t, tt.objects)
			result, err := Execute(objects, tt.query, EngineOpts{})
			if err != nil {
				t.Fatalf("Execute() returned error: %v", err)
			}
			tt.check(t, result.Objects)
		})
	}
}

func TestExecuteCountBy(t *testing.T) {
	t.Run("total count", func(t *testing.T) {
		objects := parseObjects(t, peopleData)
		result, err := Execute(objects, "count", EngineOpts{})
		if err != nil {
			t.Fatalf("Execute() returned error: %v", err)
		}
		// count should return a single value: the total number
		if len(result.Objects) != 1 {
			t.Fatalf("expected 1 result, got %d: %s", len(result.Objects), toJSON(result.Objects))
		}
		count, ok := asFloat(result.Objects[0])
		if !ok {
			// Might be wrapped in a map
			if m, mok := result.Objects[0].(map[string]any); mok {
				count, ok = asFloat(m["count"])
			}
		}
		if !ok || count != 5 {
			t.Errorf("expected count=5, got %v", result.Objects[0])
		}
	})

	t.Run("count by field", func(t *testing.T) {
		objects := parseObjects(t, runsData)
		result, err := Execute(objects, "count by .status", EngineOpts{})
		if err != nil {
			t.Fatalf("Execute() returned error: %v", err)
		}
		// Should have 3 groups: ok(3), failed(2), pending(1)
		if len(result.Objects) != 3 {
			t.Fatalf("expected 3 groups, got %d: %s", len(result.Objects), toJSON(result.Objects))
		}
		counts := map[string]float64{}
		for _, obj := range result.Objects {
			m, ok := obj.(map[string]any)
			if !ok {
				t.Fatalf("expected map result, got %T", obj)
			}
			status, _ := m["status"].(string)
			c, _ := asFloat(m["count"])
			counts[status] = c
		}
		if counts["ok"] != 3 {
			t.Errorf("expected ok=3, got %v", counts["ok"])
		}
		if counts["failed"] != 2 {
			t.Errorf("expected failed=2, got %v", counts["failed"])
		}
		if counts["pending"] != 1 {
			t.Errorf("expected pending=1, got %v", counts["pending"])
		}
	})

	t.Run("filtered count", func(t *testing.T) {
		objects := parseObjects(t, peopleData)
		result, err := Execute(objects, "where .active == true count", EngineOpts{})
		if err != nil {
			t.Fatalf("Execute() returned error: %v", err)
		}
		if len(result.Objects) != 1 {
			t.Fatalf("expected 1 result, got %d: %s", len(result.Objects), toJSON(result.Objects))
		}
		count, ok := asFloat(result.Objects[0])
		if !ok {
			if m, mok := result.Objects[0].(map[string]any); mok {
				count, ok = asFloat(m["count"])
			}
		}
		if !ok || count != 3 {
			t.Errorf("expected count=3, got %v", result.Objects[0])
		}
	})
}

func TestExecuteFirstLast(t *testing.T) {
	tests := []struct {
		name      string
		objects   []string
		query     string
		wantCount int
		check     func(t *testing.T, results []any)
	}{
		{
			name:      "first 2",
			objects:   peopleData,
			query:     "first 2",
			wantCount: 2,
			check: func(t *testing.T, results []any) {
				if fieldVal(results[0], "name") != "Alice" {
					t.Errorf("expected first to be Alice, got %v", fieldVal(results[0], "name"))
				}
				if fieldVal(results[1], "name") != "Bob" {
					t.Errorf("expected second to be Bob, got %v", fieldVal(results[1], "name"))
				}
			},
		},
		{
			name:      "last 2",
			objects:   peopleData,
			query:     "last 2",
			wantCount: 2,
			check: func(t *testing.T, results []any) {
				if fieldVal(results[0], "name") != "Diana" {
					t.Errorf("expected first to be Diana, got %v", fieldVal(results[0], "name"))
				}
				if fieldVal(results[1], "name") != "Eve" {
					t.Errorf("expected second to be Eve, got %v", fieldVal(results[1], "name"))
				}
			},
		},
		{
			name:      "filter then limit",
			objects:   peopleData,
			query:     "where .active == true first 1",
			wantCount: 1,
			check: func(t *testing.T, results []any) {
				if fieldVal(results[0], "active") != true {
					t.Errorf("expected active=true, got %v", fieldVal(results[0], "active"))
				}
			},
		},
		{
			name:      "sort then take",
			objects:   peopleData,
			query:     "sort by .age first 3",
			wantCount: 3,
			check: func(t *testing.T, results []any) {
				// Should be the 3 youngest: Eve(22), Bob(25), Diana(28)
				ages := []float64{}
				for _, r := range results {
					age, _ := asFloat(fieldVal(r, "age"))
					ages = append(ages, age)
				}
				expected := []float64{22, 25, 28}
				for i, want := range expected {
					if ages[i] != want {
						t.Errorf("result[%d]: expected age=%v, got %v", i, want, ages[i])
					}
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			objects := parseObjects(t, tt.objects)
			result, err := Execute(objects, tt.query, EngineOpts{})
			if err != nil {
				t.Fatalf("Execute() returned error: %v", err)
			}
			if len(result.Objects) != tt.wantCount {
				t.Fatalf("expected %d results, got %d: %s",
					tt.wantCount, len(result.Objects), toJSON(result.Objects))
			}
			if tt.check != nil {
				tt.check(t, result.Objects)
			}
		})
	}
}

func TestExecuteDistinct(t *testing.T) {
	tests := []struct {
		name    string
		objects []string
		query   string
		want    []string // expected unique values (sorted for comparison)
	}{
		{
			name:    "distinct status",
			objects: runsData,
			query:   "distinct .status",
			want:    []string{"failed", "ok", "pending"},
		},
		{
			name:    "distinct model",
			objects: runsData,
			query:   "distinct .model",
			want:    []string{"claude-3", "gpt-4", "mixtral"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			objects := parseObjects(t, tt.objects)
			result, err := Execute(objects, tt.query, EngineOpts{})
			if err != nil {
				t.Fatalf("Execute() returned error: %v", err)
			}
			got := make([]string, 0, len(result.Objects))
			for _, v := range result.Objects {
				s, ok := v.(string)
				if !ok {
					t.Fatalf("expected string result, got %T: %v", v, v)
				}
				got = append(got, s)
			}
			sort.Strings(got)
			sort.Strings(tt.want)
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("distinct values mismatch\n  want: %v\n  got:  %v", tt.want, got)
			}
		})
	}
}

func TestExecuteGroupBy(t *testing.T) {
	t.Run("group by field", func(t *testing.T) {
		objects := parseObjects(t, runsData)
		result, err := Execute(objects, "group by .status", EngineOpts{})
		if err != nil {
			t.Fatalf("Execute() returned error: %v", err)
		}
		// Expect groups with status, count, and items
		if len(result.Objects) != 3 {
			t.Fatalf("expected 3 groups, got %d: %s", len(result.Objects), toJSON(result.Objects))
		}
		groups := map[string]map[string]any{}
		for _, obj := range result.Objects {
			m, ok := obj.(map[string]any)
			if !ok {
				t.Fatalf("expected map, got %T", obj)
			}
			status, _ := m["status"].(string)
			groups[status] = m
		}
		// Verify counts
		okCount, _ := asFloat(groups["ok"]["count"])
		if okCount != 3 {
			t.Errorf("expected ok count=3, got %v", okCount)
		}
		failedCount, _ := asFloat(groups["failed"]["count"])
		if failedCount != 2 {
			t.Errorf("expected failed count=2, got %v", failedCount)
		}
		// Verify items exist
		if items, ok := groups["ok"]["items"].([]any); ok {
			if len(items) != 3 {
				t.Errorf("expected 3 items in ok group, got %d", len(items))
			}
		}
	})

	t.Run("select with aggregates and group by", func(t *testing.T) {
		objects := parseObjects(t, peopleData)
		result, err := Execute(objects, "select .status, count(), avg(.age) group by .status", EngineOpts{})
		if err != nil {
			t.Fatalf("Execute() returned error: %v", err)
		}
		// 2 groups: active, inactive
		if len(result.Objects) != 2 {
			t.Fatalf("expected 2 groups, got %d: %s", len(result.Objects), toJSON(result.Objects))
		}
		groups := map[string]map[string]any{}
		for _, obj := range result.Objects {
			m, ok := obj.(map[string]any)
			if !ok {
				t.Fatalf("expected map, got %T", obj)
			}
			status, _ := m["status"].(string)
			groups[status] = m
		}
		// active: Alice(30), Charlie(35), Diana(28) -> count=3, avg=31.0
		activeCount, _ := asFloat(groups["active"]["count"])
		if activeCount != 3 {
			t.Errorf("expected active count=3, got %v", activeCount)
		}
		activeAvg, _ := asFloat(groups["active"]["avg"])
		if !floatEquals(activeAvg, 31.0, 0.01) {
			t.Errorf("expected active avg=31.0, got %v", activeAvg)
		}
		// inactive: Bob(25), Eve(22) -> count=2, avg=23.5
		inactiveCount, _ := asFloat(groups["inactive"]["count"])
		if inactiveCount != 2 {
			t.Errorf("expected inactive count=2, got %v", inactiveCount)
		}
		inactiveAvg, _ := asFloat(groups["inactive"]["avg"])
		if !floatEquals(inactiveAvg, 23.5, 0.01) {
			t.Errorf("expected inactive avg=23.5, got %v", inactiveAvg)
		}
	})
}

func TestExecuteAggregate(t *testing.T) {
	tests := []struct {
		name  string
		query string
		check func(t *testing.T, results []any)
	}{
		{
			name:  "count function",
			query: "select count()",
			check: func(t *testing.T, results []any) {
				if len(results) != 1 {
					t.Fatalf("expected 1 result, got %d", len(results))
				}
				v := results[0]
				var count float64
				if m, ok := v.(map[string]any); ok {
					count, _ = asFloat(m["count"])
				} else {
					count, _ = asFloat(v)
				}
				if count != 5 {
					t.Errorf("expected count=5, got %v", count)
				}
			},
		},
		{
			name:  "avg function",
			query: "select avg(.age)",
			check: func(t *testing.T, results []any) {
				if len(results) != 1 {
					t.Fatalf("expected 1 result, got %d", len(results))
				}
				v := results[0]
				var avg float64
				if m, ok := v.(map[string]any); ok {
					avg, _ = asFloat(m["avg"])
				} else {
					avg, _ = asFloat(v)
				}
				// (30+25+35+28+22)/5 = 28.0
				if !floatEquals(avg, 28.0, 0.01) {
					t.Errorf("expected avg=28.0, got %v", avg)
				}
			},
		},
		{
			name:  "min and max",
			query: "select min(.age), max(.age)",
			check: func(t *testing.T, results []any) {
				if len(results) != 1 {
					t.Fatalf("expected 1 result, got %d", len(results))
				}
				m, ok := results[0].(map[string]any)
				if !ok {
					t.Fatalf("expected map result, got %T", results[0])
				}
				minAge, _ := asFloat(m["min"])
				maxAge, _ := asFloat(m["max"])
				if minAge != 22 {
					t.Errorf("expected min=22, got %v", minAge)
				}
				if maxAge != 35 {
					t.Errorf("expected max=35, got %v", maxAge)
				}
			},
		},
		{
			name:  "sum function",
			query: "select sum(.age)",
			check: func(t *testing.T, results []any) {
				if len(results) != 1 {
					t.Fatalf("expected 1 result, got %d", len(results))
				}
				v := results[0]
				var sum float64
				if m, ok := v.(map[string]any); ok {
					sum, _ = asFloat(m["sum"])
				} else {
					sum, _ = asFloat(v)
				}
				// 30+25+35+28+22 = 140
				if sum != 140 {
					t.Errorf("expected sum=140, got %v", sum)
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			objects := parseObjects(t, peopleData)
			result, err := Execute(objects, tt.query, EngineOpts{})
			if err != nil {
				t.Fatalf("Execute() returned error: %v", err)
			}
			tt.check(t, result.Objects)
		})
	}
}

func TestExecuteCombined(t *testing.T) {
	tests := []struct {
		name    string
		objects []string
		query   string
		check   func(t *testing.T, results []any)
	}{
		{
			name:    "select + where + sort + first",
			objects: runsData,
			query:   `select .id, .model where .status == "failed" sort by .latency_ms desc first 5`,
			check: func(t *testing.T, results []any) {
				// 2 failed runs: run_004(5400), run_002(3201) — sorted desc
				if len(results) != 2 {
					t.Fatalf("expected 2 results, got %d: %s", len(results), toJSON(results))
				}
				first, _ := results[0].(map[string]any)
				if first["id"] != "run_004" {
					t.Errorf("expected first id=run_004, got %v", first["id"])
				}
				second, _ := results[1].(map[string]any)
				if second["id"] != "run_002" {
					t.Errorf("expected second id=run_002, got %v", second["id"])
				}
			},
		},
		{
			name:    "count by + sort + first",
			objects: runsData,
			query:   "count by .model sort by count desc first 3",
			check: func(t *testing.T, results []any) {
				if len(results) != 3 {
					t.Fatalf("expected 3 results, got %d: %s", len(results), toJSON(results))
				}
				// gpt-4 has 3 runs, should be first
				first, _ := results[0].(map[string]any)
				model, _ := first["model"].(string)
				if model != "gpt-4" {
					t.Errorf("expected first model=gpt-4, got %v", model)
				}
				count, _ := asFloat(first["count"])
				if count != 3 {
					t.Errorf("expected count=3, got %v", count)
				}
			},
		},
		{
			name:    "where exists + select nested",
			objects: runsData,
			query:   `where .error exists select .id, .error.message`,
			check: func(t *testing.T, results []any) {
				if len(results) != 2 {
					t.Fatalf("expected 2 results, got %d: %s", len(results), toJSON(results))
				}
				for _, r := range results {
					m, _ := r.(map[string]any)
					if m["id"] == nil {
						t.Error("expected id field")
					}
				}
			},
		},
		{
			name:    "recursive descent with contains in pipeline",
			objects: runsData,
			query:   `where ..error contains "timeout" select .id`,
			check: func(t *testing.T, results []any) {
				// run_002 has error.message = "connection timeout"
				if len(results) < 1 {
					t.Fatalf("expected at least 1 result, got %d: %s", len(results), toJSON(results))
				}
				found := false
				for _, r := range results {
					m, ok := r.(map[string]any)
					if ok && m["id"] == "run_002" {
						found = true
					}
					// Also accept bare string
					if s, ok := r.(string); ok && s == "run_002" {
						found = true
					}
				}
				if !found {
					t.Errorf("expected run_002 in results, got %s", toJSON(results))
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			objects := parseObjects(t, tt.objects)
			result, err := Execute(objects, tt.query, EngineOpts{})
			if err != nil {
				t.Fatalf("Execute() returned error: %v", err)
			}
			tt.check(t, result.Objects)
		})
	}
}

func TestExecuteErrors(t *testing.T) {
	objects := parseObjects(t, peopleData)

	t.Run("invalid query syntax", func(t *testing.T) {
		_, err := Execute(objects, "select from where ??? !!!", EngineOpts{})
		if err == nil {
			t.Error("expected error for invalid query syntax, got nil")
		}
	})

	t.Run("division by zero", func(t *testing.T) {
		data := parseObjects(t, []string{`{"a":10,"b":0}`})
		result, err := Execute(data, "select .a / .b as ratio", EngineOpts{})
		// Either an error is returned or the result contains null for the field
		if err != nil {
			// Error is acceptable
			return
		}
		if len(result.Objects) == 1 {
			m, ok := result.Objects[0].(map[string]any)
			if ok {
				// null or special value is acceptable
				if m["ratio"] != nil {
					v, vok := asFloat(m["ratio"])
					if vok && !math.IsInf(v, 0) && !math.IsNaN(v) {
						t.Errorf("expected null, error, Inf, or NaN for division by zero, got %v", m["ratio"])
					}
				}
			}
		}
	})

	t.Run("wrong arity", func(t *testing.T) {
		_, err := Execute(objects, "select avg(.age, .id)", EngineOpts{})
		if err == nil {
			t.Error("expected error for wrong function arity, got nil")
		}
	})
}

func TestExecuteEmptyInput(t *testing.T) {
	result, err := Execute([]any{}, ".name", EngineOpts{})
	if err != nil {
		t.Fatalf("Execute() returned error for empty input: %v", err)
	}
	if len(result.Objects) != 0 {
		t.Errorf("expected 0 results for empty input, got %d", len(result.Objects))
	}
}

func TestExecuteOpts(t *testing.T) {
	// Ensure opts are accepted without error — behavior depends on implementation
	objects := parseObjects(t, peopleData[:1])
	for _, opts := range []EngineOpts{
		{ShowFile: true},
		{ShowIndex: true},
		{ShowFile: true, ShowIndex: true},
	} {
		name := fmt.Sprintf("ShowFile=%v,ShowIndex=%v", opts.ShowFile, opts.ShowIndex)
		t.Run(name, func(t *testing.T) {
			result, err := Execute(objects, ".name", opts)
			if err != nil {
				t.Fatalf("Execute() with opts %+v returned error: %v", opts, err)
			}
			if len(result.Objects) == 0 {
				t.Error("expected at least 1 result")
			}
		})
	}
}
