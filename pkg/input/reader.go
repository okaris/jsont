package input

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"
)

// Format describes the JSON format of input data.
type Format int

const (
	FormatAuto Format = iota
	FormatJSON
	FormatJSONL
)

// ReaderOpts configures the streaming reader.
type ReaderOpts struct {
	Format    Format
	Strict    bool // error on malformed lines
	Silent    bool // suppress warnings
	MaxErrors int  // abort after N errors (0 = unlimited)
}

// Object represents one JSON object with its source metadata.
type Object struct {
	Value any    // the parsed JSON value
	File  string // source filename ("stdin" for stdin)
	Index int    // 0-based index within the file
}

// Reader streams JSON objects from files or stdin.
type Reader struct {
	files     []string
	opts      ReaderOpts
	fileIdx   int
	index     int
	errCount  int
	source    *fileSource
}

type fileSource struct {
	name    string
	file    *os.File // nil for stdin
	seeker  io.ReadSeeker    // seekable source (file or bytes.Reader for stdin)
	format  Format
	scanner *bufio.Scanner   // for JSONL
	decoder *json.Decoder    // for JSON
	inArray bool             // currently inside a JSON array
	done    bool
	isStdin bool
}

// DetectFormat examines the first non-whitespace bytes to determine the format.
// [ -> FormatJSON (array), single {...} -> FormatJSON, multiple {...}\n{...} -> FormatJSONL.
func DetectFormat(peek []byte) Format {
	// Find first non-whitespace byte.
	trimmed := bytes.TrimLeft(peek, " \t\n\r")
	if len(trimmed) == 0 {
		return FormatJSON
	}

	if trimmed[0] == '[' {
		return FormatJSON
	}

	if trimmed[0] == '{' {
		// Check if there are multiple JSON objects on separate lines.
		// Find the end of the first line containing '{'.
		lines := strings.Split(string(trimmed), "\n")
		objectLines := 0
		for _, line := range lines {
			line = strings.TrimSpace(line)
			if line == "" {
				continue
			}
			if strings.HasPrefix(line, "{") {
				objectLines++
			}
			if objectLines >= 2 {
				return FormatJSONL
			}
		}
		return FormatJSON
	}

	return FormatJSON
}

// NewReader creates a reader that will stream objects from the given files.
// Use "-" to read from stdin.
func NewReader(files []string, opts ReaderOpts) (*Reader, error) {
	return &Reader{
		files: files,
		opts:  opts,
	}, nil
}

// Next returns the next Object or (nil, nil) at EOF.
func (r *Reader) Next() (*Object, error) {
	for {
		// Open next file source if needed.
		if r.source == nil {
			if r.fileIdx >= len(r.files) {
				return nil, nil
			}
			src, err := r.openSource(r.files[r.fileIdx])
			if err != nil {
				return nil, err
			}
			r.source = src
			r.index = 0
		}

		obj, err := r.readNext()
		if err != nil {
			return nil, err
		}
		if obj != nil {
			return obj, nil
		}

		// Current source exhausted, move to next file.
		r.closeSource()
		r.fileIdx++
	}
}

func (r *Reader) openSource(path string) (*fileSource, error) {
	src := &fileSource{}

	if path == "-" {
		src.name = "stdin"
		src.isStdin = true
		// Read all of stdin to detect format.
		data, err := io.ReadAll(os.Stdin)
		if err != nil {
			return nil, fmt.Errorf("reading stdin: %w", err)
		}
		format := r.opts.Format
		if format == FormatAuto {
			format = DetectFormat(data)
		}
		src.format = format

		br := bytes.NewReader(data)
		src.seeker = br
		if format == FormatJSONL {
			src.scanner = bufio.NewScanner(bytes.NewReader(data))
		} else {
			src.decoder = json.NewDecoder(br)
		}
		return src, nil
	}

	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	src.name = path
	src.file = f
	src.seeker = f

	format := r.opts.Format
	if format == FormatAuto {
		// Peek at the file to detect format.
		peek := make([]byte, 4096)
		n, _ := f.Read(peek)
		peek = peek[:n]
		format = DetectFormat(peek)
		// Seek back to start.
		if _, err := f.Seek(0, io.SeekStart); err != nil {
			f.Close()
			return nil, err
		}
	}
	src.format = format

	if format == FormatJSONL {
		src.scanner = bufio.NewScanner(f)
		src.scanner.Buffer(make([]byte, 1024*1024), 1024*1024)
	} else {
		src.decoder = json.NewDecoder(f)
	}

	return src, nil
}

func (r *Reader) closeSource() {
	if r.source != nil && r.source.file != nil {
		r.source.file.Close()
	}
	r.source = nil
}

func (r *Reader) readNext() (*Object, error) {
	src := r.source
	if src.done {
		return nil, nil
	}

	if src.format == FormatJSONL {
		return r.readNextJSONL()
	}
	return r.readNextJSON()
}

func (r *Reader) readNextJSONL() (*Object, error) {
	src := r.source
	for src.scanner.Scan() {
		line := strings.TrimSpace(src.scanner.Text())
		if line == "" {
			continue
		}

		var val any
		if err := json.Unmarshal([]byte(line), &val); err != nil {
			r.errCount++
			if r.opts.Strict {
				return nil, fmt.Errorf("%s line %d: %w", src.name, r.index, err)
			}
			if r.opts.MaxErrors > 0 && r.errCount >= r.opts.MaxErrors {
				return nil, fmt.Errorf("too many errors (%d), aborting", r.errCount)
			}
			if !r.opts.Silent {
				fmt.Fprintf(os.Stderr, "warning: %s: skipping malformed line: %s\n", src.name, err)
			}
			continue
		}

		obj := &Object{
			Value: val,
			File:  src.name,
			Index: r.index,
		}
		r.index++
		return obj, nil
	}

	src.done = true
	return nil, nil
}

func (r *Reader) readNextJSON() (*Object, error) {
	src := r.source
	dec := src.decoder

	// On first call, check if it's an array or single value.
	if !src.inArray && r.index == 0 {
		// Try to peek at the first token.
		if !dec.More() {
			src.done = true
			return nil, nil
		}

		tok, err := dec.Token()
		if err != nil {
			if err == io.EOF {
				src.done = true
				return nil, nil
			}
			return nil, fmt.Errorf("%s: %w", src.name, err)
		}

		// Check if it's the start of an array.
		if delim, ok := tok.(json.Delim); ok && delim == '[' {
			src.inArray = true
			// Now read elements from within the array.
			return r.readNextJSON()
		}

		// Not an array start and not a delim — this shouldn't happen for valid JSON.
		// It could be that the file contains a single object. We need to re-parse.
		// Unfortunately we consumed the token. Let's use a different approach:
		// Seek back and decode the whole value.
		if src.seeker != nil {
			src.seeker.Seek(0, io.SeekStart)
			dec = json.NewDecoder(src.seeker)
			src.decoder = dec
		}

		var val any
		if err := dec.Decode(&val); err != nil {
			if err == io.EOF {
				src.done = true
				return nil, nil
			}
			return nil, fmt.Errorf("%s: %w", src.name, err)
		}
		src.done = true
		obj := &Object{
			Value: val,
			File:  src.name,
			Index: 0,
		}
		r.index++
		return obj, nil
	}

	if src.inArray {
		if !dec.More() {
			// Read closing bracket.
			dec.Token()
			src.done = true
			return nil, nil
		}

		var val any
		if err := dec.Decode(&val); err != nil {
			return nil, fmt.Errorf("%s element %d: %w", src.name, r.index, err)
		}

		obj := &Object{
			Value: val,
			File:  src.name,
			Index: r.index,
		}
		r.index++
		return obj, nil
	}

	src.done = true
	return nil, nil
}
