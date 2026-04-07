# Large Data Playbook

How to work efficiently with many files, large files, or unfamiliar JSON/JSONL data. The goal is minimum tool calls with maximum useful output.

## The 3-step sequence

### Step 1 — Understand the shape

Pick one command based on what you need to know:

```bash
jt data.jsonl count              # how big is this?
jt data.jsonl schema             # what fields exist, what types, how often?
jt data.jsonl tree               # hierarchical structure overview
jt data.jsonl head 3             # peek at actual objects
jt data.jsonl stats              # numeric distributions, top string values, null rates
```

`schema` is the most useful first command — it tells you field names, types, and frequency in one shot. Use the field paths it returns to write precise queries.

### Step 2 — Search for what you need

```bash
jt data.jsonl find "error"           # returns up to 20 matches, truncated
jt data.jsonl find "error" 50        # increase limit if needed
```

`find` is safe on any file size because:
- Default limit of 20 results
- Values truncated to 120 chars in text output
- Newlines collapsed for readability
- Case-insensitive by default

The output shows the **dot-path** where each match was found. Use these paths directly in step 3.

### Step 3 — Targeted query

Use the field paths from step 1 or 2 to write a precise query:

```bash
jt data.jsonl 'where .error.message contains "timeout" select .id, .error.message, .timestamp first 20' --table
```

**Always use `first N`** with `where` queries on large data. Without it, the query scans everything and dumps everything that matches.

## Common patterns by task

### "What's in this file?"
```bash
jt data.jsonl schema
```
One command, done.

### "Find all errors"
```bash
jt data.jsonl find "error"
```
One command, shows where errors live in the structure.

### "Count by category"
```bash
jt data.jsonl 'count by .status'
```
One command, shows distribution.

### "Top N by some metric"
```bash
jt data.jsonl 'sort by .latency_ms desc first 10' --table
```
One command, sorted table.

### "Export filtered data"
```bash
jt data.jsonl 'where .status == "failed" select .id, .error.message' --csv > errors.csv
```
One command, straight to CSV.

### "Search across many files"
```bash
jt *.jsonl find "timeout"
```
One command. `find` handles multiple files, shows truncated results. If you need more detail on a specific match, follow up with a targeted query on that file.

## What to avoid

### Don't chain jt through jt
```bash
# Wasteful — two tool calls, extra context
jt data.jsonl 'where .error exists' | jt 'count by .error.code'

# Better — one call
jt data.jsonl 'where .error exists count by .error.code'
```

### Don't use `where` with `..` on large data without `first`
```bash
# Scans every object at every depth — slow on large files
jt *.jsonl 'where ..error contains "timeout"'

# Safe — stops after 10 matches
jt *.jsonl 'where ..error contains "timeout" first 10'

# Even better — use find, which is designed for searching
jt *.jsonl find "timeout"
```

### Don't dump full objects when you only need a few fields
```bash
# Dumps entire objects — huge output, wastes context
jt data.jsonl 'where .status == "failed"'

# Clean — only the fields you need
jt data.jsonl 'where .status == "failed" select .id, .error.message first 20' --table
```

## Output format cheat sheet

| Need | Flag | Notes |
|---|---|---|
| Human-readable summary | `--table` | Truncates cells to 80 chars |
| Structured data for further use | `--json` | Pretty-printed JSON |
| One object per line | `--jsonl` | Compact, good for piping |
| Plain values for shell tools | `--raw` | No JSON quotes, one per line |
| Spreadsheet export | `--csv` | With header row |
