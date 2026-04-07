package query

import (
	"testing"
)

func tok(typ TokenType, lit string) Token {
	return Token{Type: typ, Literal: lit}
}

func TestLexer(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  []Token
	}{
		// ── Simple keywords ────────────────────────────────────────
		{
			name:  "select keyword",
			input: "select",
			want: []Token{
				tok(TOKEN_SELECT, "select"),
				tok(TOKEN_EOF, ""),
			},
		},
		{
			name:  "where keyword",
			input: "where",
			want: []Token{
				tok(TOKEN_WHERE, "where"),
				tok(TOKEN_EOF, ""),
			},
		},
		{
			name:  "count by keywords",
			input: "count by",
			want: []Token{
				tok(TOKEN_COUNT, "count"),
				tok(TOKEN_BY, "by"),
				tok(TOKEN_EOF, ""),
			},
		},
		{
			name:  "sort by asc",
			input: "sort by .name asc",
			want: []Token{
				tok(TOKEN_SORT, "sort"),
				tok(TOKEN_BY, "by"),
				tok(TOKEN_DOT_PATH, ".name"),
				tok(TOKEN_ASC, "asc"),
				tok(TOKEN_EOF, ""),
			},
		},
		{
			name:  "sort by desc",
			input: "sort by .latency desc",
			want: []Token{
				tok(TOKEN_SORT, "sort"),
				tok(TOKEN_BY, "by"),
				tok(TOKEN_DOT_PATH, ".latency"),
				tok(TOKEN_DESC, "desc"),
				tok(TOKEN_EOF, ""),
			},
		},
		{
			name:  "first keyword",
			input: "first 5",
			want: []Token{
				tok(TOKEN_FIRST, "first"),
				tok(TOKEN_NUMBER, "5"),
				tok(TOKEN_EOF, ""),
			},
		},
		{
			name:  "last keyword",
			input: "last 3",
			want: []Token{
				tok(TOKEN_LAST, "last"),
				tok(TOKEN_NUMBER, "3"),
				tok(TOKEN_EOF, ""),
			},
		},
		{
			name:  "limit offset keywords",
			input: "limit 10 offset 20",
			want: []Token{
				tok(TOKEN_LIMIT, "limit"),
				tok(TOKEN_NUMBER, "10"),
				tok(TOKEN_OFFSET, "offset"),
				tok(TOKEN_NUMBER, "20"),
				tok(TOKEN_EOF, ""),
			},
		},
		{
			name:  "distinct keyword",
			input: "distinct .model",
			want: []Token{
				tok(TOKEN_DISTINCT, "distinct"),
				tok(TOKEN_DOT_PATH, ".model"),
				tok(TOKEN_EOF, ""),
			},
		},
		{
			name:  "group by keyword",
			input: "group by .status",
			want: []Token{
				tok(TOKEN_GROUP, "group"),
				tok(TOKEN_BY, "by"),
				tok(TOKEN_DOT_PATH, ".status"),
				tok(TOKEN_EOF, ""),
			},
		},
		{
			name:  "boolean keywords",
			input: "true false",
			want: []Token{
				tok(TOKEN_TRUE, "true"),
				tok(TOKEN_FALSE, "false"),
				tok(TOKEN_EOF, ""),
			},
		},
		{
			name:  "null and is keywords",
			input: "is null",
			want: []Token{
				tok(TOKEN_IS, "is"),
				tok(TOKEN_NULL, "null"),
				tok(TOKEN_EOF, ""),
			},
		},
		{
			name:  "not keyword",
			input: "not in",
			want: []Token{
				tok(TOKEN_NOT, "not"),
				tok(TOKEN_IN, "in"),
				tok(TOKEN_EOF, ""),
			},
		},
		{
			name:  "starts with keywords",
			input: "starts with",
			want: []Token{
				tok(TOKEN_STARTS, "starts"),
				tok(TOKEN_WITH, "with"),
				tok(TOKEN_EOF, ""),
			},
		},
		{
			name:  "ends with keywords",
			input: "ends with",
			want: []Token{
				tok(TOKEN_ENDS, "ends"),
				tok(TOKEN_WITH, "with"),
				tok(TOKEN_EOF, ""),
			},
		},
		{
			name:  "as keyword",
			input: "as alias",
			want: []Token{
				tok(TOKEN_AS, "as"),
				tok(TOKEN_IDENT, "alias"),
				tok(TOKEN_EOF, ""),
			},
		},
		{
			name:  "keywords are case insensitive",
			input: "SELECT WHERE AND OR NOT",
			want: []Token{
				tok(TOKEN_SELECT, "SELECT"),
				tok(TOKEN_WHERE, "WHERE"),
				tok(TOKEN_AND, "AND"),
				tok(TOKEN_OR, "OR"),
				tok(TOKEN_NOT, "NOT"),
				tok(TOKEN_EOF, ""),
			},
		},

		// ── Dot paths ──────────────────────────────────────────────
		{
			name:  "simple dot path",
			input: ".field",
			want: []Token{
				tok(TOKEN_DOT_PATH, ".field"),
				tok(TOKEN_EOF, ""),
			},
		},
		{
			name:  "nested dot path",
			input: ".field.nested",
			want: []Token{
				tok(TOKEN_DOT_PATH, ".field.nested"),
				tok(TOKEN_EOF, ""),
			},
		},
		{
			name:  "deeply nested dot path",
			input: ".a.b.c.d",
			want: []Token{
				tok(TOKEN_DOT_PATH, ".a.b.c.d"),
				tok(TOKEN_EOF, ""),
			},
		},
		{
			name:  "dot path with array index",
			input: ".field[0]",
			want: []Token{
				tok(TOKEN_DOT_PATH, ".field"),
				tok(TOKEN_LBRACKET, "["),
				tok(TOKEN_NUMBER, "0"),
				tok(TOKEN_RBRACKET, "]"),
				tok(TOKEN_EOF, ""),
			},
		},
		{
			name:  "dot path with empty brackets",
			input: ".field[]",
			want: []Token{
				tok(TOKEN_DOT_PATH, ".field"),
				tok(TOKEN_LBRACKET, "["),
				tok(TOKEN_RBRACKET, "]"),
				tok(TOKEN_EOF, ""),
			},
		},
		{
			name:  "dot path with slice",
			input: ".field[2:5]",
			want: []Token{
				tok(TOKEN_DOT_PATH, ".field"),
				tok(TOKEN_LBRACKET, "["),
				tok(TOKEN_NUMBER, "2"),
				tok(TOKEN_COLON, ":"),
				tok(TOKEN_NUMBER, "5"),
				tok(TOKEN_RBRACKET, "]"),
				tok(TOKEN_EOF, ""),
			},
		},
		{
			name:  "multiple dot paths",
			input: ".id, .name",
			want: []Token{
				tok(TOKEN_DOT_PATH, ".id"),
				tok(TOKEN_COMMA, ","),
				tok(TOKEN_DOT_PATH, ".name"),
				tok(TOKEN_EOF, ""),
			},
		},

		// ── Recursive descent (..) ─────────────────────────────────
		{
			name:  "recursive descent path",
			input: "..field",
			want: []Token{
				tok(TOKEN_DOTDOT, ".."),
				tok(TOKEN_IDENT, "field"),
				tok(TOKEN_EOF, ""),
			},
		},
		{
			name:  "recursive descent in where clause",
			input: `where ..error contains "timeout"`,
			want: []Token{
				tok(TOKEN_WHERE, "where"),
				tok(TOKEN_DOTDOT, ".."),
				tok(TOKEN_IDENT, "error"),
				tok(TOKEN_CONTAINS, "contains"),
				tok(TOKEN_STRING, "timeout"),
				tok(TOKEN_EOF, ""),
			},
		},

		// ── String literals ────────────────────────────────────────
		{
			name:  "simple string",
			input: `"hello"`,
			want: []Token{
				tok(TOKEN_STRING, "hello"),
				tok(TOKEN_EOF, ""),
			},
		},
		{
			name:  "string with spaces",
			input: `"hello world"`,
			want: []Token{
				tok(TOKEN_STRING, "hello world"),
				tok(TOKEN_EOF, ""),
			},
		},
		{
			name:  "string with escaped quotes",
			input: `"with \"escaped\" quotes"`,
			want: []Token{
				tok(TOKEN_STRING, `with \"escaped\" quotes`),
				tok(TOKEN_EOF, ""),
			},
		},
		{
			name:  "empty string",
			input: `""`,
			want: []Token{
				tok(TOKEN_STRING, ""),
				tok(TOKEN_EOF, ""),
			},
		},
		{
			name:  "string with escaped backslash",
			input: `"path\\to\\file"`,
			want: []Token{
				tok(TOKEN_STRING, `path\\to\\file`),
				tok(TOKEN_EOF, ""),
			},
		},
		{
			name:  "string with newline escape",
			input: `"line1\nline2"`,
			want: []Token{
				tok(TOKEN_STRING, `line1\nline2`),
				tok(TOKEN_EOF, ""),
			},
		},

		// ── Number literals ────────────────────────────────────────
		{
			name:  "integer",
			input: "42",
			want: []Token{
				tok(TOKEN_NUMBER, "42"),
				tok(TOKEN_EOF, ""),
			},
		},
		{
			name:  "float",
			input: "3.14",
			want: []Token{
				tok(TOKEN_NUMBER, "3.14"),
				tok(TOKEN_EOF, ""),
			},
		},
		{
			name:  "zero",
			input: "0",
			want: []Token{
				tok(TOKEN_NUMBER, "0"),
				tok(TOKEN_EOF, ""),
			},
		},
		{
			name:  "negative number",
			input: "-7",
			want: []Token{
				tok(TOKEN_MINUS, "-"),
				tok(TOKEN_NUMBER, "7"),
				tok(TOKEN_EOF, ""),
			},
		},
		{
			name:  "large number",
			input: "1000000",
			want: []Token{
				tok(TOKEN_NUMBER, "1000000"),
				tok(TOKEN_EOF, ""),
			},
		},
		{
			name:  "float without leading digit",
			input: ".5",
			want: []Token{
				// Ambiguous: could be DOT_PATH or number. Lexer should
				// treat bare .5 as a number since no alpha follows.
				tok(TOKEN_NUMBER, ".5"),
				tok(TOKEN_EOF, ""),
			},
		},

		// ── Regex literals ─────────────────────────────────────────
		{
			name:  "simple regex",
			input: "/pattern/",
			want: []Token{
				tok(TOKEN_REGEX, "/pattern/"),
				tok(TOKEN_EOF, ""),
			},
		},
		{
			name:  "regex with flags",
			input: "/time.*out/i",
			want: []Token{
				tok(TOKEN_REGEX, "/time.*out/i"),
				tok(TOKEN_EOF, ""),
			},
		},
		{
			name:  "regex with multiple flags",
			input: "/error/gi",
			want: []Token{
				tok(TOKEN_REGEX, "/error/gi"),
				tok(TOKEN_EOF, ""),
			},
		},
		{
			name:  "regex in matches clause",
			input: "matches /time.*out/i",
			want: []Token{
				tok(TOKEN_MATCHES, "matches"),
				tok(TOKEN_REGEX, "/time.*out/i"),
				tok(TOKEN_EOF, ""),
			},
		},

		// ── Operators ──────────────────────────────────────────────
		{
			name:  "equality",
			input: "==",
			want: []Token{
				tok(TOKEN_EQ, "=="),
				tok(TOKEN_EOF, ""),
			},
		},
		{
			name:  "not equal",
			input: "!=",
			want: []Token{
				tok(TOKEN_NEQ, "!="),
				tok(TOKEN_EOF, ""),
			},
		},
		{
			name:  "greater than",
			input: ">",
			want: []Token{
				tok(TOKEN_GT, ">"),
				tok(TOKEN_EOF, ""),
			},
		},
		{
			name:  "less than",
			input: "<",
			want: []Token{
				tok(TOKEN_LT, "<"),
				tok(TOKEN_EOF, ""),
			},
		},
		{
			name:  "greater than or equal",
			input: ">=",
			want: []Token{
				tok(TOKEN_GTE, ">="),
				tok(TOKEN_EOF, ""),
			},
		},
		{
			name:  "less than or equal",
			input: "<=",
			want: []Token{
				tok(TOKEN_LTE, "<="),
				tok(TOKEN_EOF, ""),
			},
		},
		{
			name:  "plus",
			input: "+",
			want: []Token{
				tok(TOKEN_PLUS, "+"),
				tok(TOKEN_EOF, ""),
			},
		},
		{
			name:  "minus",
			input: "-",
			want: []Token{
				tok(TOKEN_MINUS, "-"),
				tok(TOKEN_EOF, ""),
			},
		},
		{
			name:  "star",
			input: "*",
			want: []Token{
				tok(TOKEN_STAR, "*"),
				tok(TOKEN_EOF, ""),
			},
		},
		{
			name:  "slash outside regex context",
			input: ".a / .b",
			want: []Token{
				tok(TOKEN_DOT_PATH, ".a"),
				tok(TOKEN_SLASH, "/"),
				tok(TOKEN_DOT_PATH, ".b"),
				tok(TOKEN_EOF, ""),
			},
		},
		{
			name:  "percent",
			input: "%",
			want: []Token{
				tok(TOKEN_PERCENT, "%"),
				tok(TOKEN_EOF, ""),
			},
		},

		// ── Punctuation ────────────────────────────────────────────
		{
			name:  "comma",
			input: ",",
			want: []Token{
				tok(TOKEN_COMMA, ","),
				tok(TOKEN_EOF, ""),
			},
		},
		{
			name:  "parentheses",
			input: "()",
			want: []Token{
				tok(TOKEN_LPAREN, "("),
				tok(TOKEN_RPAREN, ")"),
				tok(TOKEN_EOF, ""),
			},
		},
		{
			name:  "brackets",
			input: "[]",
			want: []Token{
				tok(TOKEN_LBRACKET, "["),
				tok(TOKEN_RBRACKET, "]"),
				tok(TOKEN_EOF, ""),
			},
		},
		{
			name:  "colon",
			input: ":",
			want: []Token{
				tok(TOKEN_COLON, ":"),
				tok(TOKEN_EOF, ""),
			},
		},
		{
			name:  "pipe",
			input: "|",
			want: []Token{
				tok(TOKEN_PIPE, "|"),
				tok(TOKEN_EOF, ""),
			},
		},

		// ── Identifiers ────────────────────────────────────────────
		{
			name:  "simple identifier",
			input: "avg",
			want: []Token{
				tok(TOKEN_IDENT, "avg"),
				tok(TOKEN_EOF, ""),
			},
		},
		{
			name:  "identifier with underscore",
			input: "my_func",
			want: []Token{
				tok(TOKEN_IDENT, "my_func"),
				tok(TOKEN_EOF, ""),
			},
		},
		{
			name:  "function call pattern",
			input: "avg(.latency_ms)",
			want: []Token{
				tok(TOKEN_IDENT, "avg"),
				tok(TOKEN_LPAREN, "("),
				tok(TOKEN_DOT_PATH, ".latency_ms"),
				tok(TOKEN_RPAREN, ")"),
				tok(TOKEN_EOF, ""),
			},
		},

		// ── String templates ───────────────────────────────────────
		{
			name:  "string template with interpolation",
			input: `"\(.first) \(.last)"`,
			want: []Token{
				tok(TOKEN_STRING_TEMPLATE, `\(.first) \(.last)`),
				tok(TOKEN_EOF, ""),
			},
		},
		{
			name:  "string template as expression",
			input: `select "\(.first) \(.last)" as full_name`,
			want: []Token{
				tok(TOKEN_SELECT, "select"),
				tok(TOKEN_STRING_TEMPLATE, `\(.first) \(.last)`),
				tok(TOKEN_AS, "as"),
				tok(TOKEN_IDENT, "full_name"),
				tok(TOKEN_EOF, ""),
			},
		},
		{
			name:  "string template with single interpolation",
			input: `"\(.name)"`,
			want: []Token{
				tok(TOKEN_STRING_TEMPLATE, `\(.name)`),
				tok(TOKEN_EOF, ""),
			},
		},
		{
			name:  "string template with text around interpolation",
			input: `"hello \(.name)!"`,
			want: []Token{
				tok(TOKEN_STRING_TEMPLATE, `hello \(.name)!`),
				tok(TOKEN_EOF, ""),
			},
		},

		// ── Complex queries ────────────────────────────────────────
		{
			name:  "select with where sort first",
			input: `select .id, .name where .status == "failed" sort by .latency_ms desc first 10`,
			want: []Token{
				tok(TOKEN_SELECT, "select"),
				tok(TOKEN_DOT_PATH, ".id"),
				tok(TOKEN_COMMA, ","),
				tok(TOKEN_DOT_PATH, ".name"),
				tok(TOKEN_WHERE, "where"),
				tok(TOKEN_DOT_PATH, ".status"),
				tok(TOKEN_EQ, "=="),
				tok(TOKEN_STRING, "failed"),
				tok(TOKEN_SORT, "sort"),
				tok(TOKEN_BY, "by"),
				tok(TOKEN_DOT_PATH, ".latency_ms"),
				tok(TOKEN_DESC, "desc"),
				tok(TOKEN_FIRST, "first"),
				tok(TOKEN_NUMBER, "10"),
				tok(TOKEN_EOF, ""),
			},
		},
		{
			name:  "count by field",
			input: "count by .model",
			want: []Token{
				tok(TOKEN_COUNT, "count"),
				tok(TOKEN_BY, "by"),
				tok(TOKEN_DOT_PATH, ".model"),
				tok(TOKEN_EOF, ""),
			},
		},
		{
			name:  "where exists and comparison",
			input: "where .error exists and .latency_ms > 1000",
			want: []Token{
				tok(TOKEN_WHERE, "where"),
				tok(TOKEN_DOT_PATH, ".error"),
				tok(TOKEN_EXISTS, "exists"),
				tok(TOKEN_AND, "and"),
				tok(TOKEN_DOT_PATH, ".latency_ms"),
				tok(TOKEN_GT, ">"),
				tok(TOKEN_NUMBER, "1000"),
				tok(TOKEN_EOF, ""),
			},
		},
		{
			name:  "where with matches regex",
			input: "where .msg matches /time.*out/i",
			want: []Token{
				tok(TOKEN_WHERE, "where"),
				tok(TOKEN_DOT_PATH, ".msg"),
				tok(TOKEN_MATCHES, "matches"),
				tok(TOKEN_REGEX, "/time.*out/i"),
				tok(TOKEN_EOF, ""),
			},
		},
		{
			name:  "select with arithmetic and alias",
			input: "select .id, .end - .start as duration",
			want: []Token{
				tok(TOKEN_SELECT, "select"),
				tok(TOKEN_DOT_PATH, ".id"),
				tok(TOKEN_COMMA, ","),
				tok(TOKEN_DOT_PATH, ".end"),
				tok(TOKEN_MINUS, "-"),
				tok(TOKEN_DOT_PATH, ".start"),
				tok(TOKEN_AS, "as"),
				tok(TOKEN_IDENT, "duration"),
				tok(TOKEN_EOF, ""),
			},
		},
		{
			name:  "where with recursive descent and contains",
			input: `where ..error contains "timeout"`,
			want: []Token{
				tok(TOKEN_WHERE, "where"),
				tok(TOKEN_DOTDOT, ".."),
				tok(TOKEN_IDENT, "error"),
				tok(TOKEN_CONTAINS, "contains"),
				tok(TOKEN_STRING, "timeout"),
				tok(TOKEN_EOF, ""),
			},
		},
		{
			name:  "where with or condition",
			input: `where .status == "error" or .status == "failed"`,
			want: []Token{
				tok(TOKEN_WHERE, "where"),
				tok(TOKEN_DOT_PATH, ".status"),
				tok(TOKEN_EQ, "=="),
				tok(TOKEN_STRING, "error"),
				tok(TOKEN_OR, "or"),
				tok(TOKEN_DOT_PATH, ".status"),
				tok(TOKEN_EQ, "=="),
				tok(TOKEN_STRING, "failed"),
				tok(TOKEN_EOF, ""),
			},
		},
		{
			name:  "where with not",
			input: "where not .deleted",
			want: []Token{
				tok(TOKEN_WHERE, "where"),
				tok(TOKEN_NOT, "not"),
				tok(TOKEN_DOT_PATH, ".deleted"),
				tok(TOKEN_EOF, ""),
			},
		},
		{
			name:  "where is null",
			input: "where .error is null",
			want: []Token{
				tok(TOKEN_WHERE, "where"),
				tok(TOKEN_DOT_PATH, ".error"),
				tok(TOKEN_IS, "is"),
				tok(TOKEN_NULL, "null"),
				tok(TOKEN_EOF, ""),
			},
		},
		{
			name:  "where in list",
			input: `where .model in ("gpt-4", "claude-3")`,
			want: []Token{
				tok(TOKEN_WHERE, "where"),
				tok(TOKEN_DOT_PATH, ".model"),
				tok(TOKEN_IN, "in"),
				tok(TOKEN_LPAREN, "("),
				tok(TOKEN_STRING, "gpt-4"),
				tok(TOKEN_COMMA, ","),
				tok(TOKEN_STRING, "claude-3"),
				tok(TOKEN_RPAREN, ")"),
				tok(TOKEN_EOF, ""),
			},
		},
		{
			name:  "where starts with",
			input: `where .path starts with "/api"`,
			want: []Token{
				tok(TOKEN_WHERE, "where"),
				tok(TOKEN_DOT_PATH, ".path"),
				tok(TOKEN_STARTS, "starts"),
				tok(TOKEN_WITH, "with"),
				tok(TOKEN_STRING, "/api"),
				tok(TOKEN_EOF, ""),
			},
		},
		{
			name:  "where ends with",
			input: `where .name ends with ".json"`,
			want: []Token{
				tok(TOKEN_WHERE, "where"),
				tok(TOKEN_DOT_PATH, ".name"),
				tok(TOKEN_ENDS, "ends"),
				tok(TOKEN_WITH, "with"),
				tok(TOKEN_STRING, ".json"),
				tok(TOKEN_EOF, ""),
			},
		},
		{
			name:  "pipe chained query",
			input: "where .status == 200 | count by .model | sort by .count desc",
			want: []Token{
				tok(TOKEN_WHERE, "where"),
				tok(TOKEN_DOT_PATH, ".status"),
				tok(TOKEN_EQ, "=="),
				tok(TOKEN_NUMBER, "200"),
				tok(TOKEN_PIPE, "|"),
				tok(TOKEN_COUNT, "count"),
				tok(TOKEN_BY, "by"),
				tok(TOKEN_DOT_PATH, ".model"),
				tok(TOKEN_PIPE, "|"),
				tok(TOKEN_SORT, "sort"),
				tok(TOKEN_BY, "by"),
				tok(TOKEN_DOT_PATH, ".count"),
				tok(TOKEN_DESC, "desc"),
				tok(TOKEN_EOF, ""),
			},
		},
		{
			name:  "select with multiplication",
			input: "select .price * .quantity as total",
			want: []Token{
				tok(TOKEN_SELECT, "select"),
				tok(TOKEN_DOT_PATH, ".price"),
				tok(TOKEN_STAR, "*"),
				tok(TOKEN_DOT_PATH, ".quantity"),
				tok(TOKEN_AS, "as"),
				tok(TOKEN_IDENT, "total"),
				tok(TOKEN_EOF, ""),
			},
		},
		{
			name:  "comparison with boolean",
			input: "where .active == true",
			want: []Token{
				tok(TOKEN_WHERE, "where"),
				tok(TOKEN_DOT_PATH, ".active"),
				tok(TOKEN_EQ, "=="),
				tok(TOKEN_TRUE, "true"),
				tok(TOKEN_EOF, ""),
			},
		},
		{
			name:  "nested path with array access and slice",
			input: ".items[0].tags[1:3]",
			want: []Token{
				tok(TOKEN_DOT_PATH, ".items"),
				tok(TOKEN_LBRACKET, "["),
				tok(TOKEN_NUMBER, "0"),
				tok(TOKEN_RBRACKET, "]"),
				tok(TOKEN_DOT_PATH, ".tags"),
				tok(TOKEN_LBRACKET, "["),
				tok(TOKEN_NUMBER, "1"),
				tok(TOKEN_COLON, ":"),
				tok(TOKEN_NUMBER, "3"),
				tok(TOKEN_RBRACKET, "]"),
				tok(TOKEN_EOF, ""),
			},
		},

		// ── Edge cases ─────────────────────────────────────────────
		{
			name:  "empty input",
			input: "",
			want: []Token{
				tok(TOKEN_EOF, ""),
			},
		},
		{
			name:  "whitespace only",
			input: "   \t  \n  ",
			want: []Token{
				tok(TOKEN_EOF, ""),
			},
		},
		{
			name:  "unterminated string",
			input: `"hello`,
			want: []Token{
				tok(TOKEN_ILLEGAL, `"hello`),
				tok(TOKEN_EOF, ""),
			},
		},
		{
			name:  "unterminated regex",
			input: `/pattern`,
			want: []Token{
				tok(TOKEN_ILLEGAL, `/pattern`),
				tok(TOKEN_EOF, ""),
			},
		},
		{
			name:  "multiple spaces between tokens",
			input: "select   .id",
			want: []Token{
				tok(TOKEN_SELECT, "select"),
				tok(TOKEN_DOT_PATH, ".id"),
				tok(TOKEN_EOF, ""),
			},
		},
		{
			name:  "tabs and newlines as whitespace",
			input: "select\t.id\n.name",
			want: []Token{
				tok(TOKEN_SELECT, "select"),
				tok(TOKEN_DOT_PATH, ".id"),
				tok(TOKEN_DOT_PATH, ".name"),
				tok(TOKEN_EOF, ""),
			},
		},
		{
			name:  "lone dot",
			input: ".",
			want: []Token{
				tok(TOKEN_DOT, "."),
				tok(TOKEN_EOF, ""),
			},
		},
		{
			name:  "unknown single character",
			input: "~",
			want: []Token{
				tok(TOKEN_ILLEGAL, "~"),
				tok(TOKEN_EOF, ""),
			},
		},
		{
			name:  "adjacent operators without spaces",
			input: ".a>=.b",
			want: []Token{
				tok(TOKEN_DOT_PATH, ".a"),
				tok(TOKEN_GTE, ">="),
				tok(TOKEN_DOT_PATH, ".b"),
				tok(TOKEN_EOF, ""),
			},
		},
		{
			name:  "string immediately after keyword",
			input: `where"hello"`,
			want: []Token{
				tok(TOKEN_WHERE, "where"),
				tok(TOKEN_STRING, "hello"),
				tok(TOKEN_EOF, ""),
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := Lex(tt.input)
			if len(got) != len(tt.want) {
				t.Fatalf("Lex(%q): got %d tokens, want %d\ngot:  %v\nwant: %v",
					tt.input, len(got), len(tt.want), got, tt.want)
			}
			for i := range tt.want {
				if got[i].Type != tt.want[i].Type || got[i].Literal != tt.want[i].Literal {
					t.Errorf("Lex(%q): token[%d] = {%d %q}, want {%d %q}",
						tt.input, i, got[i].Type, got[i].Literal, tt.want[i].Type, tt.want[i].Literal)
				}
			}
		})
	}
}

func TestLexerTokenTypeString(t *testing.T) {
	// Verify that key token types have distinct values (no accidental collisions).
	seen := make(map[TokenType]string)
	types := map[string]TokenType{
		"EOF":             TOKEN_EOF,
		"ILLEGAL":         TOKEN_ILLEGAL,
		"SELECT":          TOKEN_SELECT,
		"WHERE":           TOKEN_WHERE,
		"SORT":            TOKEN_SORT,
		"BY":              TOKEN_BY,
		"ASC":             TOKEN_ASC,
		"DESC":            TOKEN_DESC,
		"FIRST":           TOKEN_FIRST,
		"LAST":            TOKEN_LAST,
		"LIMIT":           TOKEN_LIMIT,
		"OFFSET":          TOKEN_OFFSET,
		"COUNT":           TOKEN_COUNT,
		"GROUP":           TOKEN_GROUP,
		"DISTINCT":        TOKEN_DISTINCT,
		"AS":              TOKEN_AS,
		"AND":             TOKEN_AND,
		"OR":              TOKEN_OR,
		"NOT":             TOKEN_NOT,
		"IN":              TOKEN_IN,
		"EXISTS":          TOKEN_EXISTS,
		"IS":              TOKEN_IS,
		"NULL":            TOKEN_NULL,
		"CONTAINS":        TOKEN_CONTAINS,
		"STARTS":          TOKEN_STARTS,
		"ENDS":            TOKEN_ENDS,
		"WITH":            TOKEN_WITH,
		"MATCHES":         TOKEN_MATCHES,
		"TRUE":            TOKEN_TRUE,
		"FALSE":           TOKEN_FALSE,
		"STRING":          TOKEN_STRING,
		"NUMBER":          TOKEN_NUMBER,
		"REGEX":           TOKEN_REGEX,
		"STRING_TEMPLATE": TOKEN_STRING_TEMPLATE,
		"DOT_PATH":        TOKEN_DOT_PATH,
		"DOTDOT":          TOKEN_DOTDOT,
		"EQ":              TOKEN_EQ,
		"NEQ":             TOKEN_NEQ,
		"GT":              TOKEN_GT,
		"LT":              TOKEN_LT,
		"GTE":             TOKEN_GTE,
		"LTE":             TOKEN_LTE,
		"PLUS":            TOKEN_PLUS,
		"MINUS":           TOKEN_MINUS,
		"STAR":            TOKEN_STAR,
		"SLASH":           TOKEN_SLASH,
		"PERCENT":         TOKEN_PERCENT,
		"DOT":             TOKEN_DOT,
		"COMMA":           TOKEN_COMMA,
		"LPAREN":          TOKEN_LPAREN,
		"RPAREN":          TOKEN_RPAREN,
		"LBRACKET":        TOKEN_LBRACKET,
		"RBRACKET":        TOKEN_RBRACKET,
		"COLON":           TOKEN_COLON,
		"PIPE":            TOKEN_PIPE,
		"IDENT":           TOKEN_IDENT,
	}

	for name, typ := range types {
		if prev, exists := seen[typ]; exists {
			t.Errorf("token type collision: %s and %s both have value %d", name, prev, typ)
		}
		seen[typ] = name
	}
}
