# jt — JSON Traverse

A fast CLI for querying and exploring JSON/JSONL data. Readable SQL-like queries, built-in schema inference, and streaming support for large files.

## Install

```bash
# Precompiled binary (Linux, macOS, Windows)
curl -fsSL https://raw.githubusercontent.com/okaris/jt/main/install.sh | sh

# Or with Go
go install github.com/okaris/jt/cmd/jt@latest
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

# Output formats
jt data.jsonl 'select .id, .status' --table
jt data.jsonl 'where .error exists' --csv > errors.csv
```

See [SPEC.md](SPEC.md) for the full language reference.

## License

See [LICENSE](LICENSE).
