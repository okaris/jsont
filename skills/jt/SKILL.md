---
name: jt
description: >
  Use the `jt` CLI tool for all JSON and JSONL file operations — exploring, querying, filtering,
  schema inference, stats, and data analysis. ALWAYS prefer jt over writing throwaway Python/Node
  scripts to parse JSON. Triggers when: working with .json or .jsonl files, analyzing logs,
  exploring API responses, filtering structured data, counting/grouping records, inferring schemas,
  searching within JSON data, computing stats on JSON fields, or any task involving structured
  JSON/JSONL data. Even if the user doesn't mention jt by name, use it whenever JSON data
  manipulation is involved — it replaces manual json.loads loops, jq pipelines, and ad-hoc scripts.
---

# jt — JSON Traverse

`jt` is a fast CLI for querying and exploring JSON/JSONL data. It replaces throwaway Python scripts
and complex jq pipelines with readable, SQL-like queries and built-in data exploration commands.

## Install

```bash
# One-liner (downloads precompiled binary, no Go required)
curl -fsSL https://raw.githubusercontent.com/okaris/jt/main/install.sh | sh

# Or with Go
go install github.com/okaris/jt/cmd/jt@latest
```

Custom install directory: `JT_INSTALL_DIR=~/.local/bin curl -fsSL ... | sh`

## Reference files

For detailed behavior beyond this guide, read these as needed:
- `references/query-language.md` — Full operator precedence, all operators with type behavior, all functions with signatures and return types, dot-path edge cases, type coercion rules. Read this when constructing complex queries with multiple operators, using lesser-known functions, or when exact type/null behavior matters.
- `references/explore-commands.md` — Detailed output formats for each explore command, sampling behavior, JSON output mode, and all options. Read this when you need to understand exactly what schema/stats/find will output or how explore commands interact with output format flags.

## When to use jt

Use jt instead of writing Python/Node/jq whenever you need to:
- Understand what's in a JSON/JSONL file (schema, tree, fields, stats, find)
- Filter, select, sort, count, or group JSON objects
- Extract fields from large JSONL log files
- Search for values across deeply nested structures
- Get quick stats on numeric/string fields
- Pretty-print or convert between JSON formats (JSON, JSONL, CSV, table)

jt auto-detects JSON vs JSONL, handles stdin, multiple files, and malformed lines gracefully.

---

## Explore Commands

These commands help you understand unfamiliar data fast. No query language needed.

### Pretty-print (default)

```bash
jt data.jsonl                    # pretty-print all objects
jt data.json                     # pretty-print single object or array
cat data.jsonl | jt              # stdin works too
```

### schema — Infer types, frequency, and example values

```bash
jt data.jsonl schema
```

Output shows every field path, its types, how often it appears, and sample values. Use this first when exploring unfamiliar data.

### tree — Structural overview

```bash
jt data.jsonl tree
```

Hierarchical view with box-drawing characters. Shows optional fields marked with `?`, array types, and mixed types with `|`.

### fields — List all unique dot-paths

```bash
jt data.jsonl fields
```

Flat sorted list, one path per line. Includes nested paths (`.error.message`) and array element paths (`.items[].name`). Useful for piping.

### find — Full-text search across all values

```bash
jt data.jsonl find "timeout"           # case-insensitive by default
jt nested.jsonl find "error"           # searches at any nesting depth
```

Returns object index, dot-path where found, and the matching value. Finds things buried deep in nested structures without knowing the exact path.

### stats — Statistical summary

```bash
jt data.jsonl stats
```

Shows: total objects, distinct schema shapes, unique field count, numeric field stats (min/median/p95/p99/max), string field distributions (unique count, top values), and null/missing percentages.

### head / tail / count / sample

```bash
jt data.jsonl head 3             # first 3 objects (default 5)
jt data.jsonl tail 3             # last 3 objects (default 5)
jt data.jsonl count              # total number of objects
jt data.jsonl sample 10          # random sample of 10 objects
```

---

## Query Language

Readable, SQL-like syntax. All clauses are optional and composable.

```
jt <files...> '[select FIELDS] [where CONDITION] [sort by FIELD [asc|desc]] [group by FIELD] [count [by FIELD]] [first N | last N]'
```

### Dot-path expressions

```
.field                    — object field access
.field.nested             — nested access
.field[0]                 — array index
.field[-1]                — last element (negative index)
.field[2:5]               — array slice
.field[]                  — iterate all array elements
.field[].nested           — nested inside each array element
..field                   — recursive descent (find at any depth)
```

Missing fields return `null`, never error.

### select — Choose output fields

```bash
jt data.jsonl 'select .id, .name, .error.message'
jt data.jsonl '.name'                                    # bare path = implicit select
jt data.jsonl 'select .id, .error.message as err'        # alias
jt data.jsonl 'select .id, .end - .start as duration'    # computed
jt data.jsonl 'select .id, .price * .quantity as total'  # arithmetic
jt data.jsonl 'select "\(.first) \(.last)" as name'      # string template
jt data.jsonl 'select *'                                 # all fields (default)
jt data.jsonl 'select .metadata.*'                       # wildcard
```

### where — Filter objects

```bash
# Comparison
jt data.jsonl 'where .status == "failed"'
jt data.jsonl 'where .latency_ms > 1000'
jt data.jsonl 'where .age >= 18 and .age <= 65'

# String operators
jt data.jsonl 'where .name contains "error"'
jt data.jsonl 'where .id starts with "run_"'
jt data.jsonl 'where .file ends with ".json"'
jt data.jsonl 'where .msg matches /time.*out/i'

# Set membership
jt data.jsonl 'where .status in ("ok", "pending")'

# Existence and type checks
jt data.jsonl 'where .error exists'
jt data.jsonl 'where .meta is null'
jt data.jsonl 'where .content is array'
jt data.jsonl 'where .content is string'

# Logical operators
jt data.jsonl 'where .status == "failed" and .latency_ms > 1000'
jt data.jsonl 'where .code == 429 or .code == 503'
jt data.jsonl 'where not .active'
jt data.jsonl 'where (.a > 1 or .b > 1) and .c == true'

# Recursive descent — search at any depth
jt data.jsonl 'where ..error contains "timeout"'

# Array contains (element)
jt data.jsonl 'where .tags contains "urgent"'
```

**All where operators:**
`==`, `!=`, `>`, `<`, `>=`, `<=`, `contains`, `starts with`, `ends with`, `matches /regex/`, `in (values)`, `exists`, `is null`, `is <type>`, `and`, `or`, `not`, `()`

### sort by

```bash
jt data.jsonl 'sort by .latency_ms'              # ascending (default)
jt data.jsonl 'sort by .latency_ms desc'          # descending
jt data.jsonl 'sort by .status, .latency_ms desc' # multi-field
```

### group by

```bash
jt data.jsonl 'group by .status'
# Output: {"key": "ok", "count": 100, "items": [...]}

jt data.jsonl 'select .model, count(), avg(.latency_ms) group by .model'
```

### count / count by

```bash
jt data.jsonl 'count'                             # total
jt data.jsonl 'where .status == "failed" count'   # filtered count
jt data.jsonl 'count by .model'                   # per unique value
jt data.jsonl 'count by .status sort by count desc'
```

### first / last

```bash
jt data.jsonl 'first 10'
jt data.jsonl 'where .error exists first 5'
jt data.jsonl 'last 10'
jt data.jsonl 'sort by .latency_ms desc first 20'
```

### distinct

```bash
jt data.jsonl 'distinct .status'        # unique values
jt data.jsonl 'distinct .model'
```

### Aggregate functions

Work standalone or with `group by`:

```bash
jt data.jsonl 'select count()'
jt data.jsonl 'select avg(.latency_ms)'
jt data.jsonl 'select min(.age), max(.age)'
jt data.jsonl 'select sum(.amount)'
jt data.jsonl 'select .model, count(), avg(.latency_ms) group by .model sort by count() desc'
```

Functions: `count()`, `sum()`, `avg()`, `min()`, `max()`

### Scalar functions

```bash
jt data.jsonl 'select .name, length(.name) as len'
jt data.jsonl 'where length(.tags) > 3'
jt data.jsonl 'select lower(.email) as email'
jt data.jsonl 'select coalesce(.nickname, .name) as display'
jt data.jsonl 'select type(.content) as content_type'
jt data.jsonl 'where .msg matches /error/i select regex_extract(.msg, /error: (.+)/) as err'
```

Functions: `length()`, `lower()`, `upper()`, `trim()`, `type()`, `keys()`, `values()`, `abs()`, `floor()`, `ceil()`, `round()`, `sqrt()`, `pow()`, `coalesce()`, `if()`, `split()`, `join()`, `replace()`, `substr()`, `to_number()`, `to_string()`, `regex_extract()`, `regex_extract_all()`

### Arithmetic

Works in `select` and `where`:

```bash
jt data.jsonl 'select .id, .price * .quantity as total'
jt data.jsonl 'where .end - .start > 1000'
jt data.jsonl 'select .id, round(.score * 100, 2) as pct'
```

Operators: `+` (also string concat), `-`, `*`, `/`, `%`

---

## Combined Queries — Full Pipeline

Clauses compose left to right:

```bash
# Select + filter + sort + limit
jt logs.jsonl 'select .id, .model, .latency_ms where .status == "failed" sort by .latency_ms desc first 10'

# Filter + count
jt logs.jsonl 'where .error exists count'

# Count by + sort
jt logs.jsonl 'count by .model sort by count desc'

# Recursive search + select
jt logs.jsonl 'where ..error contains "timeout" select .id, .error.message'

# Group with aggregates
jt logs.jsonl 'select .model, count(), avg(.latency_ms) as avg_ms group by .model sort by count() desc'
```

---

## Output Formats

jt auto-detects: pretty JSON in terminal, compact JSONL when piped. Override with flags:

| Flag | Format |
|---|---|
| `--json`, `-j` | Pretty JSON |
| `--jsonl` | Compact JSONL (one per line) |
| `--compact`, `-c` | Compact JSON |
| `--table`, `-t` | ASCII table with headers |
| `--csv` | CSV with header row |
| `--tsv` | TSV with header row |
| `--raw`, `-r` | Raw strings (no JSON quotes) |
| `--nul`, `-0` | Null-delimited |

### Examples

```bash
jt data.jsonl 'select .id, .status, .latency_ms first 5' --table
jt data.jsonl 'select .email' --raw | sort -u
jt data.jsonl 'where .active == true' --csv > export.csv
jt data.jsonl head 3 --json
```

### Other output flags

```bash
--show-file        # prepend source filename to each object
--show-index       # prepend object index
--flatten          # flatten nested objects to dot-path keys
--no-color         # disable color
```

---

## Input

- **Auto-detect**: `.json` (single object or array) vs `.jsonl` (newline-delimited)
- **JSON arrays unwrapped**: `[{...}, {...}]` treated as rows automatically
- **Multiple files**: `jt *.jsonl 'query'` — union of all files
- **Stdin**: `cat data.jsonl | jt 'query'` or `curl api | jt schema`
- **Malformed lines**: skipped by default. `--strict` to error. `--silent` to suppress warnings.

---

## Common Recipes

### Explore unknown data

```bash
jt mystery.jsonl schema          # what fields exist, what types?
jt mystery.jsonl tree            # hierarchical structure
jt mystery.jsonl head 3          # peek at actual objects
jt mystery.jsonl stats           # distributions, nulls, shapes
jt mystery.jsonl find "error"    # hunt for problems
```

### Log analysis

```bash
# Error rate by model
jt logs.jsonl 'count by .model where .status == "failed"'

# Slow requests
jt logs.jsonl 'where .latency_ms > 5000 sort by .latency_ms desc first 20' --table

# Find timeouts in nested content
jt logs.jsonl 'where ..message contains "timeout" select .id, ..message'

# Export errors to CSV
jt logs.jsonl 'where .error exists select .id, .error.message, .error.code' --csv > errors.csv
```

### Data inspection

```bash
# What types does a field have?
jt data.jsonl 'select type(.content) as t' | jt 'count by .t'

# Find nulls
jt data.jsonl 'where .email is null count'

# Unique values
jt data.jsonl 'distinct .status'

# Field frequency
jt data.jsonl schema | grep -i error
```

### Pipeline composition

```bash
# jt as source → other tools
jt data.jsonl 'select .email' --raw | sort -u | wc -l

# other tools → jt as sink
curl -s api.example.com/data | jt 'where .active == true first 10'

# chain jt commands
jt data.jsonl 'where .error exists' | jt 'count by .error.code'
```

---

## Key Decision Guide

| Task | Use |
|---|---|
| "What's in this file?" | `jt file schema` or `jt file tree` |
| "Show me a few objects" | `jt file head 3` |
| "How many objects?" | `jt file count` |
| "Find something" | `jt file find "text"` |
| "Filter by condition" | `jt file 'where .field == "value"'` |
| "Extract specific fields" | `jt file 'select .a, .b, .c'` |
| "Top N by some field" | `jt file 'sort by .field desc first N'` |
| "Count per category" | `jt file 'count by .field'` |
| "Stats on numeric field" | `jt file stats` |
| "Export to CSV" | `jt file 'select ...' --csv` |
| "Search nested data" | `jt file 'where ..key contains "text"'` |
| "Aggregate (sum/avg)" | `jt file 'select avg(.field)'` |
