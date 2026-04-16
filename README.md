# jsont — JSON Traverse

A fast CLI for querying and exploring JSON/JSONL data. Readable SQL-like queries, built-in schema inference, and streaming support for large files.

Also available as `jt` (alias installed automatically).

## Install

```bash
curl -fsSL i.jsont.sh | sh
```

Or with Go:

```bash
go install github.com/okaris/jsont/cmd/jt@latest
```

## Quick Start

```bash
# Explore
jt data.jsonl schema             # infer types, frequency, examples
jt data.jsonl tree               # structural overview
jt data.jsonl find "error"       # full-text search
jt data.jsonl stats              # statistical summary

# Query
jt data.jsonl 'where .status == "failed"'
jt data.jsonl 'select .id, .model where .latency_ms > 1000 sort by .latency_ms desc first 10'
jt data.jsonl 'count by .model'

# Array index + field access — reach into nested structures
jt data.jsonl 'where .message.content[0].type == "tool_use" select .message.content[0].name'
jt data.jsonl 'select .items[0].price, .items[-1].price as last_price'

# Stdin — implicit or explicit with -
cat data.jsonl | jt 'where .error exists'
cat data.jsonl | jt - 'count by .status'

# Output formats
jt data.jsonl 'select .id, .status' --table
jt data.jsonl 'where .error exists' --csv > errors.csv
```

Both `jsont` and `jt` work identically — use whichever you prefer.

## Teach it to your AI agent

```bash
npx skills add okaris/jsont
```

Adds the jsont skill to Claude Code (or any compatible agent), so it uses `jt` instead of writing throwaway scripts when working with JSON data.

See [SPEC.md](SPEC.md) for the full language reference.

## License

See [LICENSE](LICENSE).
