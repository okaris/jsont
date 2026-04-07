package main

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

var jtBin string
var projectRoot string

func TestMain(m *testing.M) {
	// Determine project root (two dirs up from cmd/jt/).
	wd, err := os.Getwd()
	if err != nil {
		fmt.Fprintf(os.Stderr, "getwd: %s\n", err)
		os.Exit(1)
	}
	projectRoot = filepath.Join(wd, "..", "..")

	// Build binary to temp dir.
	tmp, err := os.MkdirTemp("", "jt-test")
	if err != nil {
		fmt.Fprintf(os.Stderr, "mktemp: %s\n", err)
		os.Exit(1)
	}
	jtBin = filepath.Join(tmp, "jt")
	cmd := exec.Command("go", "build", "-o", jtBin, "./cmd/jt/")
	cmd.Dir = projectRoot
	if out, err := cmd.CombinedOutput(); err != nil {
		fmt.Fprintf(os.Stderr, "build failed: %s\n%s", err, out)
		os.Exit(1)
	}
	code := m.Run()
	os.RemoveAll(tmp)
	os.Exit(code)
}

// testdata returns the absolute path to a fixture file.
func testdata(name string) string {
	return filepath.Join(projectRoot, "testdata", name)
}

// runJT executes the jt binary with the given args and optional stdin.
func runJT(t *testing.T, stdin string, args ...string) (stdout string, stderr string, exitCode int) {
	t.Helper()
	cmd := exec.Command(jtBin, args...)
	cmd.Dir = projectRoot
	if stdin != "" {
		cmd.Stdin = strings.NewReader(stdin)
	}
	var outBuf, errBuf strings.Builder
	cmd.Stdout = &outBuf
	cmd.Stderr = &errBuf
	err := cmd.Run()
	exitCode = 0
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			exitCode = exitErr.ExitCode()
		} else {
			t.Fatalf("failed to run jt: %v", err)
		}
	}
	return outBuf.String(), errBuf.String(), exitCode
}

// countJSONObjects counts the number of top-level JSON objects/values in output.
// It tries JSONL first (one per line), then JSON array, then single object.
func countJSONObjects(s string) int {
	s = strings.TrimSpace(s)
	if s == "" {
		return 0
	}
	// Try as JSON array.
	var arr []json.RawMessage
	if err := json.Unmarshal([]byte(s), &arr); err == nil {
		return len(arr)
	}
	// Count JSONL lines.
	count := 0
	for _, line := range strings.Split(s, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		var v any
		if json.Unmarshal([]byte(line), &v) == nil {
			count++
		}
	}
	if count > 0 {
		return count
	}
	// Try single object.
	var v any
	if json.Unmarshal([]byte(s), &v) == nil {
		return 1
	}
	return 0
}

// --- Pretty Print ---

func TestPrettyPrint(t *testing.T) {
	t.Run("simple_json", func(t *testing.T) {
		out, _, code := runJT(t, "", testdata("simple.json"), "--json")
		if code != 0 {
			t.Fatalf("exit code %d", code)
		}
		if !strings.Contains(out, "Alice") {
			t.Errorf("expected output to contain 'Alice', got:\n%s", out)
		}
	})

	t.Run("items_json", func(t *testing.T) {
		out, _, code := runJT(t, "", testdata("items.json"), "--json")
		if code != 0 {
			t.Fatalf("exit code %d", code)
		}
		n := countJSONObjects(out)
		if n != 5 {
			t.Errorf("expected 5 objects, got %d", n)
		}
	})

	t.Run("runs_jsonl", func(t *testing.T) {
		out, _, code := runJT(t, "", testdata("runs.jsonl"), "--json")
		if code != 0 {
			t.Fatalf("exit code %d", code)
		}
		n := countJSONObjects(out)
		if n != 10 {
			t.Errorf("expected 10 objects, got %d", n)
		}
	})

	t.Run("stdin", func(t *testing.T) {
		out, _, code := runJT(t, `{"a":1}`, "--json")
		if code != 0 {
			t.Fatalf("exit code %d", code)
		}
		if !strings.Contains(out, `"a"`) {
			t.Errorf("expected output to contain '\"a\"', got:\n%s", out)
		}
	})
}

// --- Explore: Schema ---

func TestExploreSchema(t *testing.T) {
	t.Run("fields_and_types", func(t *testing.T) {
		out, _, code := runJT(t, "", testdata("runs.jsonl"), "schema")
		if code != 0 {
			t.Fatalf("exit code %d", code)
		}
		for _, want := range []string{".id", ".model", ".status", ".latency_ms", "string", "number"} {
			if !strings.Contains(out, want) {
				t.Errorf("expected schema output to contain %q", want)
			}
		}
	})

	t.Run("frequency_indicators", func(t *testing.T) {
		out, _, code := runJT(t, "", testdata("runs.jsonl"), "schema")
		if code != 0 {
			t.Fatalf("exit code %d", code)
		}
		if !strings.Contains(out, "100%") && !strings.Contains(out, "1.00") {
			t.Errorf("expected frequency indicators in schema output")
		}
	})
}

// --- Explore: Tree ---

func TestExploreTree(t *testing.T) {
	out, _, code := runJT(t, "", testdata("runs.jsonl"), "tree")
	if code != 0 {
		t.Fatalf("exit code %d", code)
	}
	// Check tree characters.
	hasTree := strings.Contains(out, "├") || strings.Contains(out, "└") || strings.Contains(out, "│")
	if !hasTree {
		t.Errorf("expected tree characters in output")
	}
	for _, field := range []string{"id", "model", "status"} {
		if !strings.Contains(out, field) {
			t.Errorf("expected tree output to contain field %q", field)
		}
	}
}

// --- Explore: Fields ---

func TestExploreFields(t *testing.T) {
	out, _, code := runJT(t, "", testdata("runs.jsonl"), "fields")
	if code != 0 {
		t.Fatalf("exit code %d", code)
	}
	for _, want := range []string{".id", ".model", ".error.message"} {
		if !strings.Contains(out, want) {
			t.Errorf("expected fields output to contain %q", want)
		}
	}
	// One path per line.
	lines := strings.Split(strings.TrimSpace(out), "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		if !strings.HasPrefix(line, ".") {
			t.Errorf("expected field path to start with '.', got %q", line)
		}
	}
}

// --- Explore: Find ---

func TestExploreFind(t *testing.T) {
	t.Run("find_timeout", func(t *testing.T) {
		out, _, code := runJT(t, "", testdata("runs.jsonl"), "find", "timeout")
		if code != 0 {
			t.Fatalf("exit code %d", code)
		}
		if !strings.Contains(strings.ToLower(out), "timeout") {
			t.Errorf("expected find output to mention 'timeout'")
		}
	})

	t.Run("find_nested_failure", func(t *testing.T) {
		out, _, code := runJT(t, "", testdata("nested.jsonl"), "find", "failure")
		if code != 0 {
			t.Fatalf("exit code %d", code)
		}
		if !strings.Contains(strings.ToLower(out), "failure") {
			t.Errorf("expected find output to mention 'failure'")
		}
	})
}

// --- Explore: Stats ---

func TestExploreStats(t *testing.T) {
	out, _, code := runJT(t, "", testdata("runs.jsonl"), "stats")
	if code != 0 {
		t.Fatalf("exit code %d", code)
	}
	if !strings.Contains(out, "10") {
		t.Errorf("expected stats to contain object count '10'")
	}
	// Check for numeric or string stats sections.
	hasStats := strings.Contains(out, "Numeric") || strings.Contains(out, "String") ||
		strings.Contains(out, "min=") || strings.Contains(out, "unique")
	if !hasStats {
		t.Errorf("expected stats output to contain numeric/string stats")
	}
}

// --- Explore: Head ---

func TestExploreHead(t *testing.T) {
	t.Run("head_3", func(t *testing.T) {
		out, _, code := runJT(t, "", testdata("runs.jsonl"), "head", "3", "--jsonl")
		if code != 0 {
			t.Fatalf("exit code %d", code)
		}
		n := countJSONObjects(out)
		if n != 3 {
			t.Errorf("expected 3 objects, got %d", n)
		}
	})

	t.Run("head_default", func(t *testing.T) {
		out, _, code := runJT(t, "", testdata("runs.jsonl"), "head", "--jsonl")
		if code != 0 {
			t.Fatalf("exit code %d", code)
		}
		n := countJSONObjects(out)
		if n != 5 {
			t.Errorf("expected 5 objects (default), got %d", n)
		}
	})
}

// --- Explore: Tail ---

func TestExploreTail(t *testing.T) {
	out, _, code := runJT(t, "", testdata("runs.jsonl"), "tail", "2", "--jsonl")
	if code != 0 {
		t.Fatalf("exit code %d", code)
	}
	n := countJSONObjects(out)
	if n != 2 {
		t.Errorf("expected 2 objects, got %d", n)
	}
	if !strings.Contains(out, "run_010") {
		t.Errorf("expected last object to contain run_010")
	}
}

// --- Explore: Count ---

func TestExploreCount(t *testing.T) {
	t.Run("runs_count", func(t *testing.T) {
		out, _, code := runJT(t, "", testdata("runs.jsonl"), "count")
		if code != 0 {
			t.Fatalf("exit code %d", code)
		}
		if strings.TrimSpace(out) != "10" {
			t.Errorf("expected '10', got %q", strings.TrimSpace(out))
		}
	})

	t.Run("items_count", func(t *testing.T) {
		out, _, code := runJT(t, "", testdata("items.json"), "count")
		if code != 0 {
			t.Fatalf("exit code %d", code)
		}
		if strings.TrimSpace(out) != "5" {
			t.Errorf("expected '5', got %q", strings.TrimSpace(out))
		}
	})
}

// --- Explore: Sample ---

func TestExploreSample(t *testing.T) {
	out, _, code := runJT(t, "", testdata("runs.jsonl"), "sample", "3", "--jsonl")
	if code != 0 {
		t.Fatalf("exit code %d", code)
	}
	n := countJSONObjects(out)
	if n != 3 {
		t.Errorf("expected 3 objects, got %d", n)
	}
}

// --- Query: Select ---

func TestQuerySelect(t *testing.T) {
	t.Run("select_fields", func(t *testing.T) {
		out, _, code := runJT(t, "", testdata("runs.jsonl"), "select .id, .model", "--jsonl")
		if code != 0 {
			t.Fatalf("exit code %d", code)
		}
		// Check that output has id and model but not other fields like latency_ms.
		if !strings.Contains(out, "id") || !strings.Contains(out, "model") {
			t.Errorf("expected output to contain id and model fields")
		}
		if strings.Contains(out, "latency_ms") {
			t.Errorf("expected output to NOT contain latency_ms")
		}
	})

	t.Run("bare_dot_path", func(t *testing.T) {
		out, _, code := runJT(t, "", testdata("runs.jsonl"), ".id", "--jsonl")
		if code != 0 {
			t.Fatalf("exit code %d", code)
		}
		if !strings.Contains(out, "run_001") {
			t.Errorf("expected output to contain 'run_001'")
		}
	})
}

// --- Query: Where ---

func TestQueryWhere(t *testing.T) {
	t.Run("where_status_failed", func(t *testing.T) {
		out, _, code := runJT(t, "", testdata("runs.jsonl"), `where .status == "failed"`, "--jsonl")
		if code != 0 {
			t.Fatalf("exit code %d", code)
		}
		n := countJSONObjects(out)
		if n != 3 {
			t.Errorf("expected 3 failed objects, got %d", n)
		}
		if strings.Contains(out, `"ok"`) {
			t.Errorf("expected no 'ok' status in output")
		}
	})

	t.Run("where_latency_gt_1000", func(t *testing.T) {
		out, _, code := runJT(t, "", testdata("runs.jsonl"), "where .latency_ms > 1000", "--jsonl")
		if code != 0 {
			t.Fatalf("exit code %d", code)
		}
		n := countJSONObjects(out)
		if n < 1 {
			t.Errorf("expected at least 1 high-latency object, got %d", n)
		}
		// run_002 (3201), run_004 (5400), run_008 (8320) should match.
		if n != 3 {
			t.Errorf("expected 3 high-latency objects, got %d", n)
		}
	})

	t.Run("where_error_exists", func(t *testing.T) {
		out, _, code := runJT(t, "", testdata("runs.jsonl"), "where .error exists", "--jsonl")
		if code != 0 {
			t.Fatalf("exit code %d", code)
		}
		n := countJSONObjects(out)
		if n != 3 {
			t.Errorf("expected 3 objects with errors, got %d", n)
		}
	})
}

// --- Query: Sort By ---

func TestQuerySortBy(t *testing.T) {
	out, _, code := runJT(t, "", testdata("runs.jsonl"), "sort by .latency_ms desc first 3", "--jsonl")
	if code != 0 {
		t.Fatalf("exit code %d", code)
	}
	n := countJSONObjects(out)
	if n != 3 {
		t.Errorf("expected 3 objects, got %d", n)
	}
	// First result should have highest latency (8320).
	if !strings.Contains(out, "8320") {
		t.Errorf("expected first result to contain 8320 (highest latency)")
	}
	// Verify order: 8320 appears before 5400.
	idx8320 := strings.Index(out, "8320")
	idx5400 := strings.Index(out, "5400")
	if idx8320 >= 0 && idx5400 >= 0 && idx8320 > idx5400 {
		t.Errorf("expected 8320 to appear before 5400 in desc sort")
	}
}

// --- Query: Count By ---

func TestQueryCountBy(t *testing.T) {
	t.Run("count_by_model", func(t *testing.T) {
		out, _, code := runJT(t, "", testdata("runs.jsonl"), "count by .model", "--jsonl")
		if code != 0 {
			t.Fatalf("exit code %d", code)
		}
		if !strings.Contains(out, "gpt-4") || !strings.Contains(out, "claude-3") || !strings.Contains(out, "mixtral") {
			t.Errorf("expected model names in count by output:\n%s", out)
		}
	})

	t.Run("count_by_status", func(t *testing.T) {
		out, _, code := runJT(t, "", testdata("runs.jsonl"), "count by .status", "--jsonl")
		if code != 0 {
			t.Fatalf("exit code %d", code)
		}
		if !strings.Contains(out, "ok") || !strings.Contains(out, "failed") {
			t.Errorf("expected status values in count by output:\n%s", out)
		}
	})
}

// --- Query: First ---

func TestQueryFirst(t *testing.T) {
	out, _, code := runJT(t, "", testdata("runs.jsonl"), "first 2", "--jsonl")
	if code != 0 {
		t.Fatalf("exit code %d", code)
	}
	n := countJSONObjects(out)
	if n != 2 {
		t.Errorf("expected 2 objects, got %d", n)
	}
}

// --- Query: Distinct ---

func TestQueryDistinct(t *testing.T) {
	out, _, code := runJT(t, "", testdata("runs.jsonl"), "distinct .status", "--jsonl")
	if code != 0 {
		t.Fatalf("exit code %d", code)
	}
	for _, val := range []string{"ok", "failed", "pending"} {
		if !strings.Contains(out, val) {
			t.Errorf("expected distinct output to contain %q", val)
		}
	}
	// Count unique values.
	n := countJSONObjects(out)
	if n != 3 {
		t.Errorf("expected 3 distinct values, got %d", n)
	}
}

// --- Query: Combined ---

func TestQueryCombined(t *testing.T) {
	t.Run("select_where_sort_first", func(t *testing.T) {
		out, _, code := runJT(t, "", testdata("runs.jsonl"),
			`select .id, .model where .status == "failed" sort by .latency_ms desc first 2`, "--jsonl")
		if code != 0 {
			t.Fatalf("exit code %d", code)
		}
		n := countJSONObjects(out)
		if n != 2 {
			t.Errorf("expected 2 objects, got %d", n)
		}
		// Highest latency failed run is run_008 (8320).
		if !strings.Contains(out, "run_008") {
			t.Errorf("expected run_008 to be in results")
		}
		if strings.Contains(out, "latency_ms") {
			t.Errorf("expected only selected fields (no latency_ms)")
		}
	})

	t.Run("where_exists_select", func(t *testing.T) {
		out, _, code := runJT(t, "", testdata("runs.jsonl"),
			"where .error exists select .id, .error.message", "--jsonl")
		if code != 0 {
			t.Fatalf("exit code %d", code)
		}
		n := countJSONObjects(out)
		if n != 3 {
			t.Errorf("expected 3 error objects, got %d", n)
		}
		if !strings.Contains(out, "timeout") || !strings.Contains(out, "rate limit") {
			t.Errorf("expected error messages in output:\n%s", out)
		}
	})
}

// --- Output Formats ---

func TestOutputFormats(t *testing.T) {
	t.Run("json", func(t *testing.T) {
		out, _, code := runJT(t, "", testdata("runs.jsonl"), "head", "2", "--json")
		if code != 0 {
			t.Fatalf("exit code %d", code)
		}
		var v any
		if err := json.Unmarshal([]byte(out), &v); err != nil {
			t.Errorf("expected valid JSON output, got parse error: %v", err)
		}
	})

	t.Run("jsonl", func(t *testing.T) {
		out, _, code := runJT(t, "", testdata("runs.jsonl"), "head", "2", "--jsonl")
		if code != 0 {
			t.Fatalf("exit code %d", code)
		}
		lines := strings.Split(strings.TrimSpace(out), "\n")
		if len(lines) != 2 {
			t.Errorf("expected 2 JSONL lines, got %d", len(lines))
		}
		for i, line := range lines {
			var v any
			if err := json.Unmarshal([]byte(line), &v); err != nil {
				t.Errorf("line %d is not valid JSON: %v", i, err)
			}
		}
	})

	t.Run("table", func(t *testing.T) {
		out, _, code := runJT(t, "", testdata("runs.jsonl"), "select .id, .status first 3", "--table")
		if code != 0 {
			t.Fatalf("exit code %d", code)
		}
		lines := strings.Split(strings.TrimSpace(out), "\n")
		// Table should have header + separator + data rows.
		if len(lines) < 4 {
			t.Errorf("expected at least 4 lines (header + sep + 3 rows), got %d:\n%s", len(lines), out)
		}
		// Check for header-like content.
		if !strings.Contains(strings.ToLower(out), "id") || !strings.Contains(strings.ToLower(out), "status") {
			t.Errorf("expected table to contain header with 'id' and 'status'")
		}
	})

	t.Run("csv", func(t *testing.T) {
		out, _, code := runJT(t, "", testdata("runs.jsonl"), "select .id, .status first 3", "--csv")
		if code != 0 {
			t.Fatalf("exit code %d", code)
		}
		lines := strings.Split(strings.TrimSpace(out), "\n")
		// Header + 3 data rows.
		if len(lines) != 4 {
			t.Errorf("expected 4 CSV lines (header + 3 rows), got %d:\n%s", len(lines), out)
		}
		// Check comma-separated.
		if !strings.Contains(lines[0], ",") {
			t.Errorf("expected CSV header to contain commas")
		}
	})

	t.Run("raw", func(t *testing.T) {
		out, _, code := runJT(t, "", testdata("runs.jsonl"), ".id", "--raw")
		if code != 0 {
			t.Fatalf("exit code %d", code)
		}
		// Raw should not have JSON quotes around string values.
		if strings.Contains(out, `"run_001"`) {
			t.Errorf("expected raw output without JSON quotes, got:\n%s", out)
		}
		if !strings.Contains(out, "run_001") {
			t.Errorf("expected raw output to contain 'run_001'")
		}
	})
}

// --- Output Flags ---

func TestOutputFlags(t *testing.T) {
	t.Run("show_file", func(t *testing.T) {
		// --show-file should at minimum not crash; verify command succeeds.
		_, _, code := runJT(t, "", testdata("runs.jsonl"), `where .status == "failed"`, "--show-file", "--jsonl")
		if code != 0 {
			t.Fatalf("exit code %d", code)
		}
	})

	t.Run("compact", func(t *testing.T) {
		out, _, code := runJT(t, "", testdata("runs.jsonl"), "head", "2", "--compact")
		if code != 0 {
			t.Fatalf("exit code %d", code)
		}
		// Compact JSON should not have pretty-print whitespace.
		// Each object should be on its own line without indentation.
		lines := strings.Split(strings.TrimSpace(out), "\n")
		for _, line := range lines {
			line = strings.TrimSpace(line)
			if line == "" {
				continue
			}
			// Should not have leading spaces within the JSON (no indentation).
			if strings.Contains(line, "  \"") {
				t.Errorf("expected compact output without indentation, got:\n%s", line)
			}
		}
	})
}

// --- Multiple Files ---

func TestMultipleFiles(t *testing.T) {
	out, _, code := runJT(t, "", testdata("items.json"), testdata("runs.jsonl"), "count")
	if code != 0 {
		t.Fatalf("exit code %d", code)
	}
	if strings.TrimSpace(out) != "15" {
		t.Errorf("expected '15', got %q", strings.TrimSpace(out))
	}
}

// --- Stdin Pipe ---

func TestStdinPipe(t *testing.T) {
	input := `{"id":1,"status":"ok"}
{"id":2,"status":"failed"}
{"id":3,"status":"ok"}
`
	out, _, code := runJT(t, input, `where .status == "failed"`, "--jsonl")
	if code != 0 {
		t.Fatalf("exit code %d", code)
	}
	n := countJSONObjects(out)
	if n != 1 {
		t.Errorf("expected 1 filtered object from stdin, got %d", n)
	}
	if !strings.Contains(out, `"id":2`) || !strings.Contains(out, `"id": 2`) {
		// Check both compact and pretty forms.
		if !strings.Contains(out, "2") {
			t.Errorf("expected object with id 2 in output")
		}
	}
}

// --- Error Cases ---

func TestErrorCases(t *testing.T) {
	t.Run("nonexistent_file", func(t *testing.T) {
		_, stderr, code := runJT(t, "", "nonexistent.json")
		if code == 0 {
			t.Errorf("expected non-zero exit code for nonexistent file")
		}
		if stderr == "" {
			t.Errorf("expected error message on stderr")
		}
	})

	t.Run("invalid_query", func(t *testing.T) {
		_, stderr, code := runJT(t, "", testdata("runs.jsonl"), "where invalid syntax !!!")
		if code == 0 {
			t.Errorf("expected non-zero exit code for invalid query")
		}
		if stderr == "" {
			t.Errorf("expected error message on stderr")
		}
	})
}

// --- Version ---

func TestVersion(t *testing.T) {
	out, _, code := runJT(t, "", "--version")
	if code != 0 {
		t.Fatalf("exit code %d", code)
	}
	if !strings.Contains(out, "jt") || !strings.Contains(out, "0.") {
		t.Errorf("expected version string, got %q", out)
	}
}

// --- Help ---

func TestHelp(t *testing.T) {
	out, _, code := runJT(t, "", "--help")
	if code != 0 {
		t.Fatalf("exit code %d", code)
	}
	if !strings.Contains(out, "Usage:") {
		t.Errorf("expected help to contain 'Usage:', got:\n%s", out)
	}
	if !strings.Contains(out, "jt") {
		t.Errorf("expected help to mention 'jt'")
	}
}
