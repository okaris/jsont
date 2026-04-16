# jt — JSON Traverse

A fast, ergonomic CLI for querying and exploring JSON/JSONL data. Combines the power of jq with a readable query language and built-in data exploration.

## Philosophy

- **Files are tables, objects are rows** — no piping gymnastics
- **Queries, not programs** — if you know SQL, you know jt
- **Streaming by default** — handles multi-GB JSONL without blinking
- **Explore first, query second** — understand data before you extract it

## Installation

```bash
go install github.com/okaris/jsont@latest
```

Single static binary, zero dependencies.

---

## Input

### Auto-detection

jt auto-detects the input format:

- `.json` — single JSON value (object, array, scalar)
- `.jsonl` / `.ndjson` — newline-delimited JSON objects
- stdin — auto-detect based on first line

JSON arrays are automatically "unwrapped" into rows:

```bash
# these are equivalent:
jt items.jsonl 'select .name'
jt items.json 'select .name'        # if items.json is [{"name":"a"}, ...]
cat items.jsonl | jt 'select .name'
cat items.jsonl | jt - 'select .name'    # explicit stdin with -
```

### Multiple files

```bash
jt logs-*.jsonl 'where .level == "error"'
```

Multiple files are treated as a union of all rows. Use `--show-file` to include the source filename in output.

### Compression

Transparent support for `.gz`, `.zst`, `.bz2`:

```bash
jt logs.jsonl.gz schema
```

---

## Explore Commands

The primary differentiator. Point jt at unfamiliar data and understand it fast.

### `jt <file>`

Smart pretty-print. Streaming, colorized, auto-paginates in TTY.

```bash
jt data.jsonl              # pretty-print each object
jt data.json               # pretty-print with syntax highlighting
```

When piped, outputs compact JSON (one object per line for JSONL).

### `jt <file> schema`

Infer the schema from the data. Shows types, optionality, and value distributions.

```bash
$ jt runs.jsonl schema

 Field                  Type              Frequency   Example Values
 ─────────────────────────────────────────────────────────────────────
 .id                    string            100%        "run_abc123"
 .model                 string            100%        "gpt-4" (43%), "claude-3" (31%), ...
 .status                string            100%        "ok" (72%), "failed" (23%), "pending" (5%)
 .error                 object?           23%         —
 .error.message         string            23%         "timeout", "rate_limit", ...
 .error.code            number            18%         429 (60%), 500 (30%), 503 (10%)
 .latency_ms            number            97%         min=12, median=145, max=8320
 .message               object            100%        —
 .message.role          string            100%        "user" (50%), "assistant" (50%)
 .message.content       string|array      100%        —
 .metadata              object?           64%         —
 .metadata.tags         array?            31%         —
```

Options:
- `--sample N` — sample N objects for inference (default: 1000, 0 = all)
- `--depth N` — max nesting depth to display (default: 4)

### `jt <file> tree`

Structural overview, like the `tree` command for directories.

```bash
$ jt runs.jsonl tree

 runs.jsonl (14,823 objects)
 ├── .id                  string
 ├── .model               string (12 unique)
 ├── .status              string (3 unique)
 ├── .error?
 │   ├── .message         string
 │   └── .code            number
 ├── .message
 │   ├── .role            string
 │   └── .content         string | array[object]
 │       └── [].text      string
 └── .metadata?
     ├── .tags            array[string]
     └── .region          string
```

### `jt <file> fields`

Flat list of all unique dot-paths. Useful for piping.

```bash
$ jt runs.jsonl fields

.id
.model
.status
.error.message
.error.code
.message.role
.message.content
.message.content[].text
.metadata.tags
.metadata.region
```

### `jt <file> find <text>`

Full-text search across all values in all objects. Shows the dot-path where each match was found.

```bash
$ jt runs.jsonl find "timeout"

 Object #42    .error.message          "connection timeout after 30s"
 Object #187   .metadata.retry_reason  "upstream timeout"
 Object #2001  .message.content        "...the request timed out..."
```

Options:
- `-i` — case-insensitive (default)
- `-e` — exact case match
- `--regex` / `-r` — treat pattern as regex
- `--keys` — search in keys, not values
- `--first N` — stop after N matches

### `jt <file> stats`

Quick statistical summary of the entire file.

```bash
$ jt runs.jsonl stats

 Objects:   14,823
 Schemas:   3 distinct shapes
 Size:      284 MB (gzip: ~41 MB)
 Fields:    23 unique paths

 Numeric fields:
   .latency_ms    min=12  p50=145  p95=2100  p99=5800  max=8320
   .error.code    values: 429 (512x), 500 (256x), 503 (79x)

 String fields:
   .status        "ok" (10,682), "failed" (3,412), "pending" (729)
   .model         12 unique values, top: "gpt-4" (6,377)

 Nulls / missing:
   .error         77% missing
   .metadata      36% missing
```

### `jt <file> head [N]`

First N objects (default 5), pretty-printed.

```bash
jt data.jsonl head 3
```

### `jt <file> tail [N]`

Last N objects (default 5). For JSONL, requires a seek from end.

```bash
jt data.jsonl tail 3
```

### `jt <file> count`

Total number of objects.

```bash
$ jt data.jsonl count
14823
```

### `jt <file> sample [N]`

Random sample of N objects (default 5). Reservoir sampling for streaming.

```bash
jt data.jsonl sample 10
```

### `jt <file> diff <file2>`

Structural diff between two JSON files or schemas.

```bash
$ jt v1.json diff v2.json

 Added:    .metadata.version  (string)
 Removed:  .legacy_id
 Changed:  .status  string → number
 Changed:  .tags    array[string] → array[object]
```

---

## Query Language

### Basic syntax

```
jt <files...> '<query>'
```

A query is composed of clauses, in order:

```
[select FIELDS] [where CONDITION] [sort by FIELD [asc|desc]] [group by FIELD] [count [by FIELD]] [first N | last N | limit N offset M]
```

All clauses are optional. Bare dot-path is shorthand for `select`:

```bash
jt data.jsonl '.name'                    # same as: select .name
jt data.jsonl '.name, .age'              # same as: select .name, .age
```

### Dot-path expressions

```
.field                    — object field
.field.nested             — nested access
.field[0]                 — array index
.field[0].nested          — field access after array index
.field[0].a[1].b          — chained array index + field access
.field[2:5]               — array slice (index 2 to 4)
.field[-3:]               — last 3 elements
.field[:2]                — first 2 elements
.field[]                  — iterate array elements
.field[].nested           — nested inside each element
..field                   — recursive descent: find .field at any depth
.."field"                 — recursive descent with quoted key
."field with spaces"      — quoted field name
.field | length           — pipe to function
```

Accessing a missing field returns `null`, never errors.

#### Recursive descent

The `..` operator searches for a key at any nesting depth. This is one of jq's most powerful features, fully supported:

```bash
# find all "error" fields anywhere in the structure
jt data.jsonl '..error'

# filter on deeply nested fields without knowing the exact path
jt data.jsonl 'where ..status == "failed"'

# combine with other expressions
jt data.jsonl 'select ..message where ..code == 429'
```

When `..field` matches multiple locations in a single object, all matches are returned (the object is emitted once per match, or as an array if used in `select`).

#### Array slicing

Python-style slicing for arrays:

```bash
jt data.json '.items[0:3]'          # first 3 elements
jt data.json '.items[-2:]'          # last 2 elements
jt data.json '.items[::2]'          # every other element (step=2)
```

### select

Choose which fields to output.

```bash
jt data.jsonl 'select .id, .name, .error.message'
```

Aliasing:

```bash
jt data.jsonl 'select .id, .error.message as err'
```

Array index with field access:

```bash
jt data.jsonl 'select .items[0].name'                   # first element's name
jt data.jsonl 'select .message.content[0].type'          # nested array + field
jt data.jsonl 'select .rows[0].cells[1].value'           # chained: array → field → array → field
```

Computed fields:

```bash
jt data.jsonl 'select .id, .end - .start as duration'
jt data.jsonl 'select .id, .price * .quantity as total'
```

String templates:

```bash
jt data.jsonl 'select "\(.first) \(.last)" as full_name, .email'
jt data.jsonl 'select "\(.city), \(.state) \(.zip)" as address'
```

Wildcards:

```bash
jt data.jsonl 'select .metadata.*'       # all fields under metadata
jt data.jsonl 'select *'                 # all top-level fields (default)
```

### where

Filter objects.

```bash
jt data.jsonl 'where .status == "failed"'
jt data.jsonl 'where .latency_ms > 1000'
jt data.jsonl 'where .error exists'
jt data.jsonl 'where .tags contains "urgent"'
jt data.jsonl 'where .message.content[0].type == "tool_use"'    # array index in where
```

#### Arithmetic operators

Usable in `select` expressions, `where` conditions, and function arguments.

| Operator | Example | Notes |
|---|---|---|
| `+` | `.a + .b`, `"hello" + " world"` | Addition, string concatenation |
| `-` | `.end - .start` | Subtraction |
| `*` | `.price * .qty` | Multiplication |
| `/` | `.total / .count` | Division |
| `%` | `.index % 2` | Modulo |
| `-` (unary) | `-(.balance)` | Negation |

#### Comparison and logical operators

| Operator | Example | Notes |
|---|---|---|
| `==`, `!=` | `.status == "ok"` | Type-aware comparison |
| `>`, `<`, `>=`, `<=` | `.age > 18` | Numbers, strings (lexicographic) |
| `contains` | `.msg contains "error"` | String substring or array element |
| `starts with` | `.id starts with "run_"` | String prefix |
| `ends with` | `.name ends with ".json"` | String suffix |
| `matches` | `.err matches /time.*out/i` | Regex match |
| `in` | `.status in ("ok", "pending")` | Set membership |
| `exists` | `.error exists` | Field is present and not null |
| `is null` | `.meta is null` | Field is null or missing |
| `is type` | `.content is array` | Type check: string, number, bool, array, object, null |
| `and`, `or`, `not` | `.a > 1 and .b < 2` | Logical combinators |
| `( )` | `(.a or .b) and .c` | Grouping |

### sort by

```bash
jt data.jsonl 'sort by .latency_ms'          # ascending (default)
jt data.jsonl 'sort by .latency_ms desc'
jt data.jsonl 'sort by .status, .latency_ms desc'
```

Note: sort requires buffering all (matching) objects in memory.

### group by

Group objects and output each group.

```bash
jt data.jsonl 'group by .status'
```

Output:

```json
{"key": "ok", "count": 10682, "items": [...]}
{"key": "failed", "count": 3412, "items": [...]}
```

### count

```bash
jt data.jsonl 'count'                        # total count
jt data.jsonl 'where .status == "failed" count'
jt data.jsonl 'count by .model'              # count per unique value
jt data.jsonl 'count by .model sort by count desc'
```

`count by` output:

```
 gpt-4         6,377
 claude-3      4,592
 mixtral       2,103
```

### first / last / limit

```bash
jt data.jsonl 'first 10'
jt data.jsonl 'where .error exists first 5'
jt data.jsonl 'last 10'
jt data.jsonl 'limit 20 offset 100'
```

`first N` is streaming (stops after N). `last N` requires reading the whole file.

### distinct

```bash
jt data.jsonl 'distinct .model'              # unique values
jt data.jsonl 'select distinct .status'      # unique rows
```

### Aggregate functions

Usable in `select` when combined with `group by`, or standalone over all rows.

| Function | Example |
|---|---|
| `count()` | `select .model, count()` |
| `sum(expr)` | `select sum(.latency_ms)` |
| `avg(expr)` | `select .model, avg(.latency_ms)` |
| `min(expr)` | `select min(.created_at)` |
| `max(expr)` | `select max(.latency_ms)` |
| `p50(expr)` | `select p50(.latency_ms)` |
| `p95(expr)` | `select p95(.latency_ms)` |
| `p99(expr)` | `select p99(.latency_ms)` |
| `array_agg(expr)` | `select .status, array_agg(.id)` |

```bash
jt data.jsonl 'select .model, count(), avg(.latency_ms), p99(.latency_ms) group by .model sort by count() desc'
```

### Scalar functions

Usable anywhere an expression is expected.

| Function | Description |
|---|---|
| `abs(expr)` | Absolute value |
| `floor(expr)` | Round down |
| `ceil(expr)` | Round up |
| `round(expr)` | Round to nearest integer |
| `round(expr, N)` | Round to N decimal places |
| `log(expr)` | Natural log |
| `pow(a, b)` | Exponentiation |
| `sqrt(expr)` | Square root |
| `length(expr)` | String length or array length |
| `keys(expr)` | Object keys as array |
| `values(expr)` | Object values as array |
| `type(expr)` | Type name as string |
| `flatten(expr)` | Flatten nested arrays |
| `lower(expr)` | Lowercase string |
| `upper(expr)` | Uppercase string |
| `trim(expr)` | Strip whitespace |
| `split(expr, sep)` | Split string into array |
| `join(expr, sep)` | Join array into string |
| `replace(expr, old, new)` | String replacement |
| `substr(expr, start, len)` | Substring |
| `to_number(expr)` | Parse string as number |
| `to_string(expr)` | Coerce to string |
| `now()` | Current unix timestamp |
| `time(expr, fmt)` | Parse time string |
| `duration(expr)` | Human-readable duration from ms |
| `if(cond, then, else)` | Conditional expression |
| `coalesce(a, b, ...)` | First non-null value |
| `regex_extract(expr, pat)` | Extract first regex match |
| `regex_extract_all(expr, pat)` | Extract all regex matches as array |
| `json_parse(expr)` | Parse JSON string into object |

---

## Output

### Auto-format

- **TTY** — pretty-printed, colorized, auto-paged
- **Pipe** — compact JSON, one object per line (JSONL)
- **Single scalar** — raw value, no quotes

### Explicit format flags

| Flag | Output |
|---|---|
| `--json` / `-j` | Pretty JSON |
| `--jsonl` | Compact JSONL (one per line) |
| `--compact` / `-c` | Compact JSON (no whitespace) |
| `--table` / `-t` | ASCII table |
| `--csv` | CSV with header row |
| `--tsv` | TSV with header row |
| `--raw` / `-r` | Raw strings (no JSON quotes) |
| `--nul` / `-0` | Null-delimited (for xargs -0) |

### Table output

```bash
$ jt runs.jsonl 'select .id, .model, .status, .latency_ms first 5' --table

 id          model      status   latency_ms
 ──────────────────────────────────────────
 run_abc123  gpt-4      ok       142
 run_def456  claude-3   failed   3201
 run_ghi789  mixtral    ok       89
 run_jkl012  gpt-4      ok       201
 run_mno345  claude-3   pending  —
```

### Color

- `--color always|never|auto` (default: auto based on TTY)
- `--no-color` shorthand

### Other output options

- `--show-file` — prepend source filename to each output object
- `--show-index` — prepend the object index (0-based) within the file
- `--flatten` — flatten nested objects to dot-path keys
- `--unwrap <path>` — extract a nested field as the root object

---

## Global Flags

| Flag | Description |
|---|---|
| `--help`, `-h` | Help |
| `--version`, `-v` | Version |
| `--verbose` | Show timing and debug info |
| `--silent`, `-s` | Suppress errors (skip malformed lines) |
| `--strict` | Error on malformed lines (default: warn and skip) |
| `--max-errors N` | Abort after N parse errors (default: 100) |
| `--workers N` | Parallel workers for multi-file (default: num CPUs) |
| `--mem-limit N` | Max memory for sort/group operations (default: 1GB) |
| `--no-mmap` | Disable memory-mapped file reading |

---

## Piping and Composition

jt works naturally in pipelines:

```bash
# jt as source
jt data.jsonl 'where .status == "failed"' | jq '.error'

# jt as sink
curl -s api.example.com/data | jt 'where .active == true first 10'

# chain jt commands
jt data.jsonl 'where .error exists' | jt 'count by .error.code'

# with standard tools
jt data.jsonl 'select .email' --raw | sort -u | wc -l
```

---

## Recipes

### Explore unknown data

```bash
jt mystery.jsonl schema          # what's in here?
jt mystery.jsonl tree            # structural overview
jt mystery.jsonl head 3          # peek at a few objects
jt mystery.jsonl find "error"    # hunt for problems
```

### Log analysis

```bash
# error rate by model
jt logs.jsonl 'select .model, count(), sum(if(.status == "failed", 1, 0)) as errors group by .model' --table

# slow requests
jt logs.jsonl 'where .latency_ms > 5000 sort by .latency_ms desc first 20'

# regex search in nested content
jt logs.jsonl 'where .message.content matches /timeout|deadline/i select .id, regex_extract(.message.content, /\w*time\w*/) as match'
```

### Data cleanup

```bash
# find objects with unexpected types
jt data.jsonl 'where .age is string'

# find nulls
jt data.jsonl 'where .email is null count'

# deduplicate by field
jt data.jsonl 'distinct .email select *'
```

### Multi-file

```bash
# union across files, track source
jt 2024-*.jsonl 'where .error exists' --show-file --table

# compare schemas between files
jt v1.jsonl diff v2.jsonl
```

---

## Performance Targets

- **Streaming**: constant memory for filter/select/first/find operations
- **Throughput**: >500 MB/s for simple filters on modern hardware
- **Startup**: <10ms cold start
- **Large files**: tested up to 50GB JSONL

---

## Project Structure

```
jt/
├── cmd/jt/main.go              — CLI entry point, arg parsing
├── pkg/
│   ├── input/
│   │   ├── reader.go           — streaming JSON/JSONL reader
│   │   ├── detect.go           — format auto-detection
│   │   └── compress.go         — gzip/zstd/bz2 decompression
│   ├── query/
│   │   ├── lexer.go            — tokenizer
│   │   ├── parser.go           — query string → AST
│   │   ├── ast.go              — AST node types
│   │   └── eval.go             — evaluate expressions against objects
│   ├── explore/
│   │   ├── schema.go           — schema inference
│   │   ├── tree.go             — tree display
│   │   ├── find.go             — full-text search
│   │   ├── stats.go            — statistical summary
│   │   └── diff.go             — structural diff
│   ├── engine/
│   │   ├── pipeline.go         — query execution pipeline
│   │   ├── aggregate.go        — group by, count, sum, etc.
│   │   ├── sort.go             — external sort for large data
│   │   └── distinct.go         — deduplication
│   └── output/
│       ├── formatter.go        — output format dispatcher
│       ├── json.go             — JSON/JSONL output
│       ├── table.go            — ASCII table
│       ├── csv.go              — CSV/TSV output
│       └── color.go            — terminal colors
├── SPEC.md                     — this file
├── go.mod
└── go.sum
```

---

## v0.1 Scope

Ship a useful tool fast. v0.1 includes:

1. **Input**: JSON/JSONL auto-detect, stdin, multiple files, gzip
2. **Explore**: `schema`, `tree`, `fields`, `find`, `stats`, `head`, `tail`, `count`, `sample`
3. **Query**: `select`, `where` (all operators), `sort by`, `first`/`last`, `count by`, `distinct`
4. **Functions**: `length`, `type`, `lower`, `upper`, `contains` (as function), `coalesce`, `if`, `regex_extract`
5. **Output**: auto-format, `--json`, `--jsonl`, `--table`, `--csv`, `--raw`, `--color`
6. **Flags**: `--show-file`, `--show-index`, `--silent`, `--strict`

### Deferred to v0.2+

- `group by` with full aggregate functions
- `diff` command
- Compression: zstd, bz2
- `--mem-limit`, external sort
- `--workers` parallel multi-file
- percentile functions (p50, p95, p99)
- `limit`/`offset`
- `json_parse()`, `time()` functions

---

## Gap Analysis: jt vs jq

jq is a Turing-complete functional language. jt intentionally does not replicate its full programming model. This section documents what jq can do that jt cannot, for future consideration.

### Covered by jt (with better syntax)

| jq | jt | Notes |
|---|---|---|
| `.foo.bar` | `.foo.bar` | Identical |
| `.[] \| select(.x > 1)` | `where .x > 1` | SQL-like |
| `[.[] \| .name]` | `select .name` | Implicit collection |
| `group_by(.x) \| map({k:.[0].x, n:length})` | `count by .x` | One clause vs pipeline |
| `sort_by(.x)` | `sort by .x` | Readable |
| `unique_by(.x)` | `distinct .x` | Readable |
| `..` (recursive descent) | `..field` | Supported |
| `.[2:5]` | `.[2:5]` | Supported |
| `\(.x) is \(.y)` (interpolation) | `"\(.x) is \(.y)"` | Supported |
| `+`, `-`, `*`, `/` | `+`, `-`, `*`, `/` | Supported |
| `floor`, `ceil`, `round` | `floor()`, `ceil()`, `round()` | Supported |
| `.x // "default"` (alternative) | `coalesce(.x, "default")` | Supported |
| `if-then-else` | `if(cond, then, else)` | Function form |
| `length`, `keys`, `values` | `length()`, `keys()`, `values()` | Supported |
| `test("regex")` | `matches /regex/` | Supported |
| `type` | `type()` | Supported |
| `@csv`, `@tsv` | `--csv`, `--tsv` | Output flags |

### NOT covered — intentional gaps (may revisit)

| jq Feature | What it does | Why jt skips it | Workaround |
|---|---|---|---|
| **Variable binding** `as $var` | Bind intermediate results: `.x as $v \| .y + $v` | Adds programming language complexity | Use computed fields with `as` aliases in select |
| **Reduce** | `reduce .[] as $x (0; . + $x)` — general fold | jt has `sum()`, `count()`, `avg()` for common cases | Use aggregate functions; pipe to jq for custom reduce |
| **User-defined functions** | `def double: . * 2; .items[] \| double` | jt is a query tool, not a programming language | Write the expression inline |
| **Update expressions** | `.foo \|= . + 1` — modify in place | jt is read-only by design | Pipe jt output through jq for transforms |
| **Recursive rewrite** | `walk(if type == "string" then ascii_downcase else . end)` | Deep structural transforms aren't queries | Pipe to jq |
| **Try/catch** | `try .foo.bar catch "n/a"` | jt already returns null for missing paths | `coalesce(.foo.bar, "n/a")` |
| **Label/break** | `label $out \| foreach .[] as $x (...)` | Control flow for streaming programs | Not applicable to query model |
| **`@base64`/`@uri`/`@html`** | Format strings as base64, URI, HTML | Rare in data exploration | Pipe to standard tools |
| **`env`** | Access environment variables | Out of scope | Use shell: `jt data.jsonl "where .key == \"$VAR\""` |
| **`input`/`inputs`** | Read additional inputs during processing | jt handles multi-file natively | Use multiple file args |
| **Object construction** `{a,b}` | Build new objects from scratch: `{id: .foo, sum: (.a+.b)}` | `select .foo as id, .a + .b as sum` covers most cases | For complex reshaping, pipe to jq |

### The line

jt's position: **if it reads like a query, jt does it. If it reads like a program, use jq.**

The moment you need variables, loops, recursion, or structural transforms — that's jq territory. jt should never become a second jq. Instead, jt should be so good at the query/explore use case that you reach for jq 10x less often.
