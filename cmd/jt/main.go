package main

import (
	"fmt"
	"math"
	"os"
	"sort"
	"strconv"
	"strings"

	"github.com/okaris/jsont/pkg/engine"
	"github.com/okaris/jsont/pkg/explore"
	"github.com/okaris/jsont/pkg/input"
	"github.com/okaris/jsont/pkg/output"
)

var version = "0.1.0" // overridden by -ldflags at build time

func main() {
	if err := run(os.Args[1:]); err != nil {
		fmt.Fprintf(os.Stderr, "jt: %s\n", err)
		os.Exit(1)
	}
}

// flags holds all parsed CLI flags.
type flags struct {
	format    output.Format
	formatSet bool
	color     bool
	noColor   bool
	showFile  bool
	showIndex bool
	flatten   bool
	silent    bool
	strict    bool
	maxErrors int
	depth     int
	depthSet  bool
	verbose   bool
	version   bool
	help      bool
}

func run(args []string) error {
	f, positional := parseFlags(args)

	if f.help {
		printHelp()
		return nil
	}
	if f.version {
		fmt.Println("jt " + version)
		return nil
	}

	// Separate files from command/query among positional args.
	var files []string
	var rest []string
	for _, arg := range positional {
		if fileExists(arg) {
			files = append(files, arg)
		} else {
			rest = append(rest, arg)
		}
	}

	// If no files and stdin is not a TTY, read from stdin.
	if len(files) == 0 {
		if stdinIsPiped() {
			files = []string{"-"}
		} else if len(rest) == 0 {
			printHelp()
			return nil
		}
	}

	if len(files) == 0 && len(rest) > 0 {
		// Maybe the first rest arg was intended as a file?
		return fmt.Errorf("no input files (file %q not found)", rest[0])
	}

	// Read all objects (with metadata for find --show-file).
	metaObjects, err := readAllWithMeta(files, f)
	if err != nil {
		return err
	}
	objects := make([]any, len(metaObjects))
	for i, mo := range metaObjects {
		objects[i] = mo.Value
	}

	// Determine output format.
	outFmt := output.FormatJSON
	if f.formatSet {
		outFmt = f.format
	} else if !stdoutIsTTY() {
		outFmt = output.FormatJSONL
	}

	outOpts := output.Opts{
		Format:    outFmt,
		Color:     f.color && !f.noColor,
		ShowFile:  f.showFile,
		ShowIndex: f.showIndex,
		Flatten:   f.flatten,
	}

	// No rest args: pretty-print.
	if len(rest) == 0 {
		return output.FormatOutput(os.Stdout, objects, outOpts)
	}

	// Check for explore commands.
	cmd := rest[0]
	switch cmd {
	case "schema":
		return runSchema(objects, f, outOpts)
	case "tree":
		return runTree(objects, f, outOpts)
	case "fields":
		return runFields(objects, f, outOpts)
	case "find":
		pattern := ""
		findLimit := 20 // sensible default
		if len(rest) > 1 {
			pattern = rest[1]
		}
		if len(rest) > 2 {
			if parsed, err := strconv.Atoi(rest[2]); err == nil && parsed > 0 {
				findLimit = parsed
			}
		}
		return runFindWithMeta(metaObjects, pattern, findLimit, f, outOpts)
	case "stats":
		return runStats(objects, f, outOpts)
	case "head":
		n := 5
		if len(rest) > 1 {
			parsed, err := strconv.Atoi(rest[1])
			if err == nil && parsed > 0 {
				n = parsed
			}
		}
		return runHead(objects, n, outOpts)
	case "tail":
		n := 5
		if len(rest) > 1 {
			parsed, err := strconv.Atoi(rest[1])
			if err == nil && parsed > 0 {
				n = parsed
			}
		}
		return runTail(objects, n, outOpts)
	case "count":
		// Bare "count" is explore. "count by .field" is a query.
		if len(rest) > 1 && rest[1] == "by" {
			// It's a query: "count by .field ..."
			queryStr := strings.Join(rest, " ")
			return runQuery(objects, queryStr, f, outOpts)
		}
		return runCount(objects)
	case "sample":
		n := 5
		if len(rest) > 1 {
			parsed, err := strconv.Atoi(rest[1])
			if err == nil && parsed > 0 {
				n = parsed
			}
		}
		return runSample(objects, n, outOpts)
	}

	// Check if it looks like a query.
	queryStr := strings.Join(rest, " ")
	if isQuery(queryStr) {
		return runQuery(objects, queryStr, f, outOpts)
	}

	return fmt.Errorf("unknown command or query: %s", queryStr)
}

// isQuery returns true if the string looks like a jt query rather than an explore command.
func isQuery(s string) bool {
	if strings.HasPrefix(s, ".") {
		return true
	}
	lower := strings.ToLower(s)
	keywords := []string{"select ", "where ", "sort ", "group ", "distinct ", "first ", "last ", "limit ", "count by "}
	for _, kw := range keywords {
		if strings.HasPrefix(lower, kw) || strings.Contains(lower, " "+kw) {
			return true
		}
	}
	// Also treat bare "count" in a multi-word context as query (e.g., "where .x exists count")
	if strings.Contains(lower, " count") {
		return true
	}
	return false
}

func runQuery(objects []any, queryStr string, f flags, outOpts output.Opts) error {
	result, err := engine.Execute(objects, queryStr, engine.EngineOpts{
		ShowFile:  f.showFile,
		ShowIndex: f.showIndex,
	})
	if err != nil {
		return err
	}
	// Special case: plain count returns a single number.
	if len(result.Objects) == 1 {
		if n, ok := result.Objects[0].(float64); ok && n == math.Trunc(n) {
			// Could be a plain count result. Check if query is just "count" or ends with "count".
			lower := strings.TrimSpace(strings.ToLower(queryStr))
			if lower == "count" || strings.HasSuffix(lower, " count") {
				fmt.Println(int(n))
				return nil
			}
		}
	}
	return output.FormatOutput(os.Stdout, result.Objects, outOpts)
}

func pathDepth(path string) int {
	if path == "" {
		return 0
	}
	// Count dots in path: ".a.b.c" = depth 3, ".a" = depth 1
	return strings.Count(path, ".")
}

func runSchema(objects []any, f flags, outOpts output.Opts) error {
	schema := explore.InferSchema(objects, explore.SchemaOpts{MaxExamples: 5})

	// Apply depth filter (default 4)
	maxDepth := 4
	if f.depthSet {
		maxDepth = f.depth
	}
	var filtered []explore.SchemaField
	skipped := 0
	for _, sf := range schema {
		if pathDepth(sf.Path) <= maxDepth {
			filtered = append(filtered, sf)
		} else {
			skipped++
		}
	}

	if f.formatSet {
		var items []any
		for _, sf := range filtered {
			items = append(items, map[string]any{
				"field":     sf.Path,
				"types":     sf.Types,
				"frequency": sf.Frequency,
				"unique":    sf.Unique,
			})
		}
		return output.FormatOutput(os.Stdout, items, outOpts)
	}
	// Text-based rendering.
	fmt.Fprintf(os.Stdout, " %-30s %-20s %-12s %s\n", "Field", "Type", "Frequency", "Example Values")
	fmt.Fprintf(os.Stdout, " %s\n", strings.Repeat("\u2500", 80))
	for _, sf := range filtered {
		typeStr := strings.Join(sf.Types, "|")
		// Filter out "array" if a typed array is present.
		if len(sf.Types) > 1 {
			filtered := make([]string, 0)
			hasTyped := false
			for _, t := range sf.Types {
				if strings.HasPrefix(t, "array[") {
					hasTyped = true
				}
			}
			for _, t := range sf.Types {
				if t == "array" && hasTyped {
					continue
				}
				filtered = append(filtered, t)
			}
			typeStr = strings.Join(filtered, "|")
		}
		freq := fmt.Sprintf("%.0f%%", sf.Frequency*100)
		examples := formatExamples(sf.Examples, 3)
		fmt.Fprintf(os.Stdout, " %-30s %-20s %-12s %s\n", sf.Path, typeStr, freq, examples)
	}
	if skipped > 0 {
		fmt.Fprintf(os.Stdout, "\n %d fields hidden at depth > %d (use --depth N to show more)\n", skipped, maxDepth)
	}
	return nil
}

func formatExamples(examples []any, max int) string {
	if len(examples) == 0 {
		return "\u2014"
	}
	seen := map[string]bool{}
	var parts []string
	for _, ex := range examples {
		s := fmt.Sprintf("%v", ex)
		if _, ok := ex.(string); ok {
			s = fmt.Sprintf("%q", ex)
		}
		if seen[s] {
			continue
		}
		seen[s] = true
		// Truncate long examples
		s = strings.ReplaceAll(s, "\n", " ")
		s = strings.ReplaceAll(s, "\t", " ")
		if len(s) > 50 {
			s = s[:50] + "..."
		}
		parts = append(parts, s)
		if len(parts) >= max {
			break
		}
	}
	return strings.Join(parts, ", ")
}

func runTree(objects []any, f flags, outOpts output.Opts) error {
	tree := explore.BuildTree(objects, explore.TreeOpts{})
	if f.formatSet {
		return output.FormatOutput(os.Stdout, []any{treeToMap(tree)}, outOpts)
	}
	fmt.Fprintf(os.Stdout, " (%d objects)\n", len(objects))
	printTree(os.Stdout, tree.Children, "")
	return nil
}

func treeToMap(n *explore.TreeNode) map[string]any {
	m := map[string]any{
		"path": n.Path,
		"type": n.Type,
	}
	if n.Optional {
		m["optional"] = true
	}
	if len(n.Children) > 0 {
		children := make([]any, len(n.Children))
		for i, c := range n.Children {
			children[i] = treeToMap(c)
		}
		m["children"] = children
	}
	return m
}

func printTree(w *os.File, nodes []*explore.TreeNode, prefix string) {
	for i, n := range nodes {
		isLast := i == len(nodes)-1
		connector := "\u251C\u2500\u2500"
		childPrefix := "\u2502   "
		if isLast {
			connector = "\u2514\u2500\u2500"
			childPrefix = "    "
		}
		opt := ""
		if n.Optional {
			opt = "?"
		}
		typeStr := ""
		if n.Type != "" && n.Type != "object" {
			typeStr = "  " + n.Type
		} else if n.Type == "object" && len(n.Children) == 0 {
			typeStr = "  object"
		}
		fmt.Fprintf(w, " %s%s .%s%s%s\n", prefix, connector, n.Path, opt, typeStr)
		if len(n.Children) > 0 {
			printTree(w, n.Children, prefix+childPrefix)
		}
	}
}

func runFields(objects []any, f flags, outOpts output.Opts) error {
	fields := explore.ListFields(objects)
	if f.formatSet {
		items := make([]any, len(fields))
		for i, f := range fields {
			items[i] = f
		}
		return output.FormatOutput(os.Stdout, items, outOpts)
	}
	for _, path := range fields {
		fmt.Println(path)
	}
	return nil
}

func runFind(objects []any, pattern string, limit int, f flags, outOpts output.Opts) error {
	if pattern == "" {
		return fmt.Errorf("find requires a search pattern")
	}
	results := explore.Find(objects, pattern, explore.FindOpts{CaseInsensitive: true, First: limit})
	if f.formatSet {
		items := make([]any, len(results))
		for i, r := range results {
			val := r.Value
			if len(val) > 200 {
				val = val[:200] + "..."
			}
			items[i] = map[string]any{
				"index": r.Index,
				"path":  r.Path,
				"value": val,
				"file":  r.File,
			}
		}
		return output.FormatOutput(os.Stdout, items, outOpts)
	}
	for _, r := range results {
		val := r.Value
		// Collapse newlines and truncate for readable output
		val = strings.ReplaceAll(val, "\n", " ")
		val = strings.ReplaceAll(val, "\t", " ")
		if len(val) > 120 {
			val = val[:120] + "..."
		}
		file := ""
		if f.showFile && r.File != "" {
			file = fmt.Sprintf("[%s] ", r.File)
		}
		fmt.Fprintf(os.Stdout, " %s#%-6d %-30s %s\n", file, r.Index, r.Path, val)
	}
	if len(results) == 0 {
		fmt.Println("No matches found.")
	}
	return nil
}

func runFindWithMeta(metaObjects []input.Object, pattern string, limit int, f flags, outOpts output.Opts) error {
	if pattern == "" {
		return fmt.Errorf("find requires a search pattern")
	}

	// Build a file lookup: global index -> source filename.
	fileMap := make([]string, len(metaObjects))
	objects := make([]any, len(metaObjects))
	for i, mo := range metaObjects {
		objects[i] = mo.Value
		fileMap[i] = mo.File
	}

	results := explore.Find(objects, pattern, explore.FindOpts{CaseInsensitive: true, First: limit})

	// Populate File field from our metadata.
	for i := range results {
		if results[i].Index >= 0 && results[i].Index < len(fileMap) {
			results[i].File = fileMap[results[i].Index]
		}
	}

	if f.formatSet {
		items := make([]any, len(results))
		for i, r := range results {
			val := r.Value
			if len(val) > 200 {
				val = val[:200] + "..."
			}
			items[i] = map[string]any{
				"index": r.Index,
				"path":  r.Path,
				"value": val,
				"file":  r.File,
			}
		}
		return output.FormatOutput(os.Stdout, items, outOpts)
	}
	for _, r := range results {
		val := r.Value
		val = strings.ReplaceAll(val, "\n", " ")
		val = strings.ReplaceAll(val, "\t", " ")
		if len(val) > 120 {
			val = val[:120] + "..."
		}
		file := ""
		if f.showFile && r.File != "" {
			file = fmt.Sprintf("[%s] ", r.File)
		}
		fmt.Fprintf(os.Stdout, " %s#%-6d %-30s %s\n", file, r.Index, r.Path, val)
	}
	if len(results) == 0 {
		fmt.Println("No matches found.")
	}
	return nil
}

func runStats(objects []any, f flags, outOpts output.Opts) error {
	stats := explore.ComputeStats(objects)

	maxDepth := 4
	if f.depthSet {
		maxDepth = f.depth
	}

	if f.formatSet {
		item := map[string]any{
			"count":   stats.Count,
			"schemas": stats.SchemaCount,
			"fields":  stats.Fields,
		}
		return output.FormatOutput(os.Stdout, []any{item}, outOpts)
	}
	fmt.Fprintf(os.Stdout, " Objects:   %d\n", stats.Count)
	fmt.Fprintf(os.Stdout, " Schemas:   %d distinct shapes\n", stats.SchemaCount)
	fmt.Fprintf(os.Stdout, " Fields:    %d unique paths\n", stats.Fields)

	skipped := 0
	if len(stats.NumericStats) > 0 {
		fmt.Fprintln(os.Stdout, "\n Numeric fields:")
		paths := sortedKeys(stats.NumericStats)
		for _, path := range paths {
			if pathDepth(path) > maxDepth {
				skipped++
				continue
			}
			ns := stats.NumericStats[path]
			fmt.Fprintf(os.Stdout, "   %-20s min=%.4g  median=%.4g  p95=%.4g  p99=%.4g  max=%.4g\n",
				path, ns.Min, ns.Median, ns.P95, ns.P99, ns.Max)
		}
	}

	if len(stats.StringStats) > 0 {
		fmt.Fprintln(os.Stdout, "\n String fields:")
		paths := sortedKeys(stats.StringStats)
		for _, path := range paths {
			if pathDepth(path) > maxDepth {
				skipped++
				continue
			}
			ss := stats.StringStats[path]
			top := topN(ss.TopValues, 3)
			fmt.Fprintf(os.Stdout, "   %-20s %d unique, top: %s\n", path, ss.Unique, top)
		}
	}

	if len(stats.NullCounts) > 0 {
		fmt.Fprintln(os.Stdout, "\n Nulls / missing:")
		paths := sortedKeys(stats.NullCounts)
		for _, path := range paths {
			if pathDepth(path) > maxDepth {
				skipped++
				continue
			}
			cnt := stats.NullCounts[path]
			pct := float64(cnt) / float64(stats.Count) * 100
			fmt.Fprintf(os.Stdout, "   %-20s %.0f%% missing (%d)\n", path, pct, cnt)
		}
	}
	if skipped > 0 {
		fmt.Fprintf(os.Stdout, "\n %d fields hidden at depth > %d (use --depth N to show more)\n", skipped, maxDepth)
	}
	return nil
}

func runHead(objects []any, n int, outOpts output.Opts) error {
	result := explore.Head(objects, n)
	return output.FormatOutput(os.Stdout, result, outOpts)
}

func runTail(objects []any, n int, outOpts output.Opts) error {
	result := explore.Tail(objects, n)
	return output.FormatOutput(os.Stdout, result, outOpts)
}

func runCount(objects []any) error {
	fmt.Println(explore.Count(objects))
	return nil
}

func runSample(objects []any, n int, outOpts output.Opts) error {
	result := explore.Sample(objects, n)
	return output.FormatOutput(os.Stdout, result, outOpts)
}

// readAll reads all objects from the given files.
func readAll(files []string, f flags) ([]any, error) {
	reader, err := input.NewReader(files, input.ReaderOpts{
		Strict:    f.strict,
		Silent:    f.silent,
		MaxErrors: f.maxErrors,
	})
	if err != nil {
		return nil, err
	}
	var objects []any
	for {
		obj, err := reader.Next()
		if err != nil {
			return nil, err
		}
		if obj == nil {
			break
		}
		objects = append(objects, obj.Value)
	}
	return objects, nil
}

// readAllWithMeta reads all objects from the given files, preserving source metadata.
func readAllWithMeta(files []string, f flags) ([]input.Object, error) {
	reader, err := input.NewReader(files, input.ReaderOpts{
		Strict:    f.strict,
		Silent:    f.silent,
		MaxErrors: f.maxErrors,
	})
	if err != nil {
		return nil, err
	}
	var objects []input.Object
	for {
		obj, err := reader.Next()
		if err != nil {
			return nil, err
		}
		if obj == nil {
			break
		}
		objects = append(objects, *obj)
	}
	return objects, nil
}

// parseFlags extracts flags from args, returning (flags, remaining positional args).
func parseFlags(args []string) (flags, []string) {
	f := flags{
		maxErrors: 100,
		color:     true,
	}
	var positional []string
	for i := 0; i < len(args); i++ {
		arg := args[i]
		switch arg {
		case "--json", "-j":
			f.format = output.FormatJSON
			f.formatSet = true
		case "--jsonl":
			f.format = output.FormatJSONL
			f.formatSet = true
		case "--table", "-t":
			f.format = output.FormatTable
			f.formatSet = true
		case "--csv":
			f.format = output.FormatCSV
			f.formatSet = true
		case "--tsv":
			f.format = output.FormatTSV
			f.formatSet = true
		case "--raw", "-r":
			f.format = output.FormatRaw
			f.formatSet = true
		case "--compact", "-c":
			f.format = output.FormatCompact
			f.formatSet = true
		case "--nul", "-0":
			f.format = output.FormatNul
			f.formatSet = true
		case "--color":
			f.color = true
		case "--no-color":
			f.noColor = true
		case "--show-file":
			f.showFile = true
		case "--show-index":
			f.showIndex = true
		case "--flatten":
			f.flatten = true
		case "--silent", "-s":
			f.silent = true
		case "--strict":
			f.strict = true
		case "--verbose":
			f.verbose = true
		case "--version", "-v":
			f.version = true
		case "--help", "-h":
			f.help = true
		case "--depth":
			if i+1 < len(args) {
				i++
				if n, err := strconv.Atoi(args[i]); err == nil {
					f.depth = n
					f.depthSet = true
				}
			}
		case "--max-errors":
			if i+1 < len(args) {
				i++
				if n, err := strconv.Atoi(args[i]); err == nil {
					f.maxErrors = n
				}
			}
		default:
			positional = append(positional, arg)
		}
	}
	return f, positional
}

func fileExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && !info.IsDir()
}

func stdinIsPiped() bool {
	fi, err := os.Stdin.Stat()
	if err != nil {
		return false
	}
	return (fi.Mode() & os.ModeCharDevice) == 0
}

func stdoutIsTTY() bool {
	fi, err := os.Stdout.Stat()
	if err != nil {
		return false
	}
	return (fi.Mode() & os.ModeCharDevice) != 0
}

func sortedKeys[V any](m map[string]V) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

func topN(counts map[string]int, n int) string {
	type kv struct {
		key   string
		count int
	}
	var items []kv
	for k, v := range counts {
		items = append(items, kv{k, v})
	}
	sort.Slice(items, func(i, j int) bool { return items[i].count > items[j].count })
	var parts []string
	for i, item := range items {
		if i >= n {
			break
		}
		parts = append(parts, fmt.Sprintf("%q (%d)", item.key, item.count))
	}
	return strings.Join(parts, ", ")
}

func printHelp() {
	fmt.Print(`jt - JSON Traverse

Usage:
  jt <files...> [query|command] [flags]

Explore commands:
  jt data.jsonl                    Pretty-print
  jt data.jsonl schema             Infer schema
  jt data.jsonl tree               Structural overview
  jt data.jsonl fields             List all field paths
  jt data.jsonl find "text" [N]    Full-text search (default 20 results)
  jt data.jsonl stats              Statistical summary
  jt data.jsonl head [N]           First N objects (default 5)
  jt data.jsonl tail [N]           Last N objects (default 5)
  jt data.jsonl count              Total count
  jt data.jsonl sample [N]         Random sample (default 5)

Query examples:
  jt data.jsonl 'select .id, .name where .status == "failed"'
  jt data.jsonl '.name'
  jt data.jsonl 'where .error exists first 5'
  jt data.jsonl 'count by .model'

Stdin:
  cat data.jsonl | jt 'where .status == "failed"'
  echo '{"a":1}' | jt

Output flags:
  --json, -j       Pretty JSON
  --jsonl           Compact JSONL
  --table, -t       ASCII table
  --csv             CSV
  --tsv             TSV
  --raw, -r         Raw strings
  --compact, -c     Compact JSON
  --nul, -0         Null-delimited

Other flags:
  --color           Force color
  --no-color        Disable color
  --show-file       Show source filename
  --show-index      Show object index
  --flatten         Flatten nested objects
  --silent, -s      Suppress warnings
  --strict          Error on malformed lines
  --depth N         Max field depth for schema/stats (default 4)
  --max-errors N    Abort after N errors (default 100)
  --verbose         Show timing info
  --version, -v     Show version
  --help, -h        Show this help
`)
}
