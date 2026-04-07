# jt Explore Commands — Full Reference

Read this when you need detailed behavior of explore commands, their options, and output format specifics.

## Table of Contents

1. [schema](#schema)
2. [tree](#tree)
3. [fields](#fields)
4. [find](#find)
5. [stats](#stats)
6. [head / tail / count / sample](#head--tail--count--sample)
7. [Output format interaction](#output-format-interaction)

---

## schema

```bash
jt data.jsonl schema
```

Scans all objects and infers a schema showing every field path with its type(s), frequency, and example values.

### Output columns

| Column | Description |
|---|---|
| Field | Dot-path (e.g., `.error.message`) |
| Type | Observed types, joined with `\|` for mixed (e.g., `string\|array`) |
| Frequency | Percentage of objects where this field is present (100% = every object) |
| Example Values | Up to 3 sample values from the data |

### Type detection

- Primitives: `string`, `number`, `boolean`, `null`
- Typed arrays: `array[string]`, `array[number]`, `array[object]`
- Mixed arrays: `array` (when elements have different types)
- Nested objects: flattened into separate field paths (`.address.city` not `.address`)
- Mixed types at same path: shown as `string|array`

### Schema sampling

By default, schema scans all objects. For very large files, the first 1000 objects are sampled. The frequency values reflect the sample, not the full file.

### JSON output

```bash
jt data.jsonl schema --json
```

Returns an array of objects with fields: `field`, `types`, `frequency`, `unique`.

---

## tree

```bash
jt data.jsonl tree
```

Displays a hierarchical tree of the data structure using box-drawing characters.

### Output format

```
 (14823 objects)
 ├── .id                  string
 ├── .model               string (12 unique)
 ├── .status              string (3 unique)
 ├── .error?
 │   ├── .message         string
 │   └── .code            number
 └── .metadata?
     ├── .tags            array[string]
     └── .region          string
```

### Markers

- `?` suffix — field is optional (not present in all objects)
- Type shown after field name
- Mixed types shown with `|` (e.g., `string|array`)
- Unique value counts shown in parentheses for low-cardinality fields

---

## fields

```bash
jt data.jsonl fields
```

Flat sorted list of every unique dot-path in the data. One per line.

### Output

```
.id
.model
.status
.error.message
.error.code
.message.role
.message.content
.items[].name
.metadata.tags
```

### Key behaviors

- Paths are sorted alphabetically
- Nested objects produce separate entries (`.error.message`, not just `.error`)
- Array element fields use `[]` notation: `.items[].name`
- No duplicates — each path appears exactly once
- Works across all objects (union of all shapes)

### Use case

Pipe to grep or use for scripting:

```bash
jt data.jsonl fields | grep error
jt data.jsonl fields | wc -l
```

---

## find

```bash
jt data.jsonl find "search text"        # returns up to 20 matches (default)
jt data.jsonl find "search text" 50     # returns up to 50 matches
```

Full-text search across all values in all objects. Reports where each match was found.

**Default limit: 20 results.** Pass a number after the pattern to override. This prevents accidentally dumping huge output on large files.

### Output format

```
 Object #42    .error.message          "connection timeout after 30s"
 Object #187   .metadata.retry_reason  "upstream timeout"
```

### Behavior

- **Case-insensitive** by default
- **Limited to 20 results** by default (pass N to override)
- Searches all string values at any nesting depth
- Searches inside arrays (checks each element)
- Reports the object index, the exact dot-path, and the matching value
- Does NOT search in keys (field names) by default

### JSON output

```bash
jt data.jsonl find "timeout" --json
```

Returns array of `{"index": N, "path": ".field", "value": "matched text"}`.

---

## stats

```bash
jt data.jsonl stats
```

Comprehensive statistical summary of the entire file.

### Output sections

**Header:**
- Total object count
- Number of distinct schema shapes (objects with different field sets)
- Number of unique field paths

**Numeric fields:**
For each numeric field: min, median, p95, p99, max.

```
 Numeric fields:
   .latency_ms          min=12  median=145  p95=2100  p99=5800  max=8320
   .error.code          min=429  median=429  p95=503  p99=503  max=503
```

**String fields:**
For each string field: unique value count and top 3 most common values with counts.

```
 String fields:
   .status              3 unique, top: "ok" (10682), "failed" (3412), "pending" (729)
   .model               12 unique, top: "gpt-4" (6377), "claude-3" (4592), "mixtral" (2103)
```

**Nulls / missing:**
Fields that are missing or null in some objects, with percentage and count.

```
 Nulls / missing:
   .error               77% missing (11411)
   .metadata            36% missing (5336)
```

---

## head / tail / count / sample

### head

```bash
jt data.jsonl head        # first 5 objects (default)
jt data.jsonl head 3      # first 3 objects
```

Returns the first N objects, pretty-printed. Respects output format flags.

### tail

```bash
jt data.jsonl tail        # last 5 objects (default)
jt data.jsonl tail 3      # last 3 objects
```

Returns the last N objects. Requires reading the entire file (can't stream).

### count

```bash
jt data.jsonl count       # prints: 14823
```

Outputs the total number of objects as a plain integer. Works on JSON arrays (counts elements) and JSONL (counts lines).

**Note:** Bare `count` is an explore command. `count by .field` is a query (different behavior).

### sample

```bash
jt data.jsonl sample       # 5 random objects (default)
jt data.jsonl sample 10    # 10 random objects
```

Returns a random sample using reservoir sampling. Different runs may return different objects.

---

## Output format interaction

All explore commands respect output format flags:

```bash
jt data.jsonl schema --json       # schema as JSON array
jt data.jsonl head 5 --table      # first 5 as ASCII table
jt data.jsonl head 5 --csv        # first 5 as CSV
jt data.jsonl find "x" --json     # find results as JSON
jt data.jsonl stats --json        # stats as JSON object
```

Without explicit format flags:
- `head`, `tail`, `sample` → pretty JSON (TTY) or JSONL (pipe)
- `schema`, `tree`, `fields`, `find`, `stats` → custom text rendering (TTY and pipe)
- `count` → plain integer

**Table output note:** `--table` truncates cell values to 80 characters (with `...`) and strips newlines. This keeps tables readable even with large nested objects. Use `--json` if you need full untruncated values.
