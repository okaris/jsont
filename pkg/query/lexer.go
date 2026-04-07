package query

import (
	"strings"
)

// TokenType represents the type of a lexer token.
type TokenType int

const (
	// Special
	TOKEN_EOF TokenType = iota
	TOKEN_ILLEGAL

	// Keywords
	TOKEN_SELECT
	TOKEN_WHERE
	TOKEN_SORT
	TOKEN_BY
	TOKEN_ASC
	TOKEN_DESC
	TOKEN_FIRST
	TOKEN_LAST
	TOKEN_LIMIT
	TOKEN_OFFSET
	TOKEN_COUNT
	TOKEN_GROUP
	TOKEN_DISTINCT
	TOKEN_AS
	TOKEN_AND
	TOKEN_OR
	TOKEN_NOT
	TOKEN_IN
	TOKEN_EXISTS
	TOKEN_IS
	TOKEN_NULL
	TOKEN_CONTAINS
	TOKEN_STARTS
	TOKEN_ENDS
	TOKEN_WITH
	TOKEN_MATCHES
	TOKEN_TRUE
	TOKEN_FALSE

	// Literals
	TOKEN_STRING          // "hello"
	TOKEN_NUMBER          // 42, 3.14
	TOKEN_REGEX           // /pattern/flags
	TOKEN_STRING_TEMPLATE // "\(.x) text"

	// Paths
	TOKEN_DOT_PATH // .field, .field.nested
	TOKEN_DOTDOT   // ..

	// Operators
	TOKEN_EQ      // ==
	TOKEN_NEQ     // !=
	TOKEN_GT      // >
	TOKEN_LT      // <
	TOKEN_GTE     // >=
	TOKEN_LTE     // <=
	TOKEN_PLUS    // +
	TOKEN_MINUS   // -
	TOKEN_STAR    // *
	TOKEN_SLASH   // /
	TOKEN_PERCENT // %

	// Punctuation
	TOKEN_DOT      // .
	TOKEN_COMMA    // ,
	TOKEN_LPAREN   // (
	TOKEN_RPAREN   // )
	TOKEN_LBRACKET // [
	TOKEN_RBRACKET // ]
	TOKEN_COLON    // :
	TOKEN_PIPE     // |

	// Identifiers
	TOKEN_IDENT // function names, unquoted field names
)

// Token represents a single lexer token.
type Token struct {
	Type    TokenType
	Literal string
}

var keywords = map[string]TokenType{
	"select":   TOKEN_SELECT,
	"where":    TOKEN_WHERE,
	"sort":     TOKEN_SORT,
	"by":       TOKEN_BY,
	"asc":      TOKEN_ASC,
	"desc":     TOKEN_DESC,
	"first":    TOKEN_FIRST,
	"last":     TOKEN_LAST,
	"limit":    TOKEN_LIMIT,
	"offset":   TOKEN_OFFSET,
	"count":    TOKEN_COUNT,
	"group":    TOKEN_GROUP,
	"distinct": TOKEN_DISTINCT,
	"as":       TOKEN_AS,
	"and":      TOKEN_AND,
	"or":       TOKEN_OR,
	"not":      TOKEN_NOT,
	"in":       TOKEN_IN,
	"exists":   TOKEN_EXISTS,
	"is":       TOKEN_IS,
	"null":     TOKEN_NULL,
	"contains": TOKEN_CONTAINS,
	"starts":   TOKEN_STARTS,
	"ends":     TOKEN_ENDS,
	"with":     TOKEN_WITH,
	"matches":  TOKEN_MATCHES,
	"true":     TOKEN_TRUE,
	"false":    TOKEN_FALSE,
}

// Lex tokenizes the input string into a slice of Tokens.
func Lex(input string) []Token {
	l := &lexer{
		input:  input,
		tokens: make([]Token, 0),
	}
	l.run()
	return l.tokens
}

type lexer struct {
	input  string
	pos    int
	tokens []Token
}

func (l *lexer) peek() byte {
	if l.pos >= len(l.input) {
		return 0
	}
	return l.input[l.pos]
}

func (l *lexer) advance() byte {
	ch := l.input[l.pos]
	l.pos++
	return ch
}

func (l *lexer) emit(typ TokenType, lit string) {
	l.tokens = append(l.tokens, Token{Type: typ, Literal: lit})
}

func (l *lexer) skipWhitespace() {
	for l.pos < len(l.input) {
		ch := l.input[l.pos]
		if ch == ' ' || ch == '\t' || ch == '\n' || ch == '\r' {
			l.pos++
		} else {
			break
		}
	}
}

func (l *lexer) lastNonEOFType() TokenType {
	if len(l.tokens) == 0 {
		return TOKEN_EOF // start of input
	}
	return l.tokens[len(l.tokens)-1].Type
}

// slashIsRegex returns true if a '/' at the current position should start a regex.
func (l *lexer) slashIsRegex() bool {
	prev := l.lastNonEOFType()
	switch prev {
	case TOKEN_EOF: // start of input
		return true
	case TOKEN_EQ, TOKEN_NEQ, TOKEN_GT, TOKEN_LT, TOKEN_GTE, TOKEN_LTE,
		TOKEN_PLUS, TOKEN_MINUS, TOKEN_STAR, TOKEN_SLASH, TOKEN_PERCENT:
		return true
	case TOKEN_COMMA, TOKEN_LPAREN, TOKEN_LBRACKET, TOKEN_COLON, TOKEN_PIPE:
		return true
	case TOKEN_SELECT, TOKEN_WHERE, TOKEN_SORT, TOKEN_BY, TOKEN_ASC, TOKEN_DESC,
		TOKEN_FIRST, TOKEN_LAST, TOKEN_LIMIT, TOKEN_OFFSET, TOKEN_COUNT,
		TOKEN_GROUP, TOKEN_DISTINCT, TOKEN_AS, TOKEN_AND, TOKEN_OR, TOKEN_NOT,
		TOKEN_IN, TOKEN_EXISTS, TOKEN_IS, TOKEN_NULL, TOKEN_CONTAINS,
		TOKEN_STARTS, TOKEN_ENDS, TOKEN_WITH, TOKEN_MATCHES,
		TOKEN_TRUE, TOKEN_FALSE:
		return true
	}
	return false
}

func isAlpha(ch byte) bool {
	return (ch >= 'a' && ch <= 'z') || (ch >= 'A' && ch <= 'Z')
}

func isDigit(ch byte) bool {
	return ch >= '0' && ch <= '9'
}

func isIdentChar(ch byte) bool {
	return isAlpha(ch) || isDigit(ch) || ch == '_'
}

func (l *lexer) readString() {
	start := l.pos
	l.pos++ // skip opening quote
	hasTemplate := false
	var buf strings.Builder
	for l.pos < len(l.input) {
		ch := l.input[l.pos]
		if ch == '\\' {
			if l.pos+1 < len(l.input) {
				next := l.input[l.pos+1]
				if next == '(' {
					hasTemplate = true
				}
				buf.WriteByte(ch)
				buf.WriteByte(next)
				l.pos += 2
				continue
			}
		}
		if ch == '"' {
			lit := buf.String()
			l.pos++ // skip closing quote
			if hasTemplate {
				l.emit(TOKEN_STRING_TEMPLATE, lit)
			} else {
				l.emit(TOKEN_STRING, lit)
			}
			return
		}
		buf.WriteByte(ch)
		l.pos++
	}
	// Unterminated string
	l.emit(TOKEN_ILLEGAL, l.input[start:l.pos])
}

func (l *lexer) readRegex() {
	start := l.pos
	l.pos++ // skip opening /
	for l.pos < len(l.input) {
		ch := l.input[l.pos]
		if ch == '\\' && l.pos+1 < len(l.input) {
			l.pos += 2 // skip escaped char
			continue
		}
		if ch == '/' {
			l.pos++ // skip closing /
			// Read flags
			for l.pos < len(l.input) && isAlpha(l.input[l.pos]) {
				l.pos++
			}
			l.emit(TOKEN_REGEX, l.input[start:l.pos])
			return
		}
		l.pos++
	}
	// Unterminated regex
	l.emit(TOKEN_ILLEGAL, l.input[start:l.pos])
}

func (l *lexer) readNumber() {
	start := l.pos
	for l.pos < len(l.input) && isDigit(l.input[l.pos]) {
		l.pos++
	}
	if l.pos < len(l.input) && l.input[l.pos] == '.' {
		// Check if next char is a digit (decimal point)
		if l.pos+1 < len(l.input) && isDigit(l.input[l.pos+1]) {
			l.pos++ // consume dot
			for l.pos < len(l.input) && isDigit(l.input[l.pos]) {
				l.pos++
			}
		}
	}
	l.emit(TOKEN_NUMBER, l.input[start:l.pos])
}

func (l *lexer) readIdentOrKeyword() {
	start := l.pos
	for l.pos < len(l.input) && isIdentChar(l.input[l.pos]) {
		l.pos++
	}
	word := l.input[start:l.pos]
	if typ, ok := keywords[strings.ToLower(word)]; ok {
		l.emit(typ, word)
	} else {
		l.emit(TOKEN_IDENT, word)
	}
}

func (l *lexer) readDot() {
	// We're at a '.', figure out what it is.
	// Check for '..'
	if l.pos+1 < len(l.input) && l.input[l.pos+1] == '.' {
		l.emit(TOKEN_DOTDOT, "..")
		l.pos += 2
		return
	}

	// Check for dot path: '.' followed by an alpha or underscore
	if l.pos+1 < len(l.input) && (isAlpha(l.input[l.pos+1]) || l.input[l.pos+1] == '_') {
		start := l.pos
		l.pos++ // skip initial dot
		// read first segment
		for l.pos < len(l.input) && isIdentChar(l.input[l.pos]) {
			l.pos++
		}
		// Continue reading .segment parts
		for l.pos < len(l.input) && l.input[l.pos] == '.' {
			// peek ahead: next must be alpha/underscore to continue path
			if l.pos+1 < len(l.input) && (isAlpha(l.input[l.pos+1]) || l.input[l.pos+1] == '_') {
				l.pos++ // skip dot
				for l.pos < len(l.input) && isIdentChar(l.input[l.pos]) {
					l.pos++
				}
			} else {
				break
			}
		}
		l.emit(TOKEN_DOT_PATH, l.input[start:l.pos])
		return
	}

	// Check for number like .5
	if l.pos+1 < len(l.input) && isDigit(l.input[l.pos+1]) {
		start := l.pos
		l.pos++ // skip dot
		for l.pos < len(l.input) && isDigit(l.input[l.pos]) {
			l.pos++
		}
		l.emit(TOKEN_NUMBER, l.input[start:l.pos])
		return
	}

	// Lone dot
	l.emit(TOKEN_DOT, ".")
	l.pos++
}

func (l *lexer) run() {
	for {
		l.skipWhitespace()
		if l.pos >= len(l.input) {
			l.emit(TOKEN_EOF, "")
			return
		}

		ch := l.peek()

		switch {
		case ch == '"':
			l.readString()
		case ch == '.':
			l.readDot()
		case isDigit(ch):
			l.readNumber()
		case isAlpha(ch) || ch == '_':
			l.readIdentOrKeyword()
		case ch == '/':
			if l.slashIsRegex() {
				l.readRegex()
			} else {
				l.emit(TOKEN_SLASH, "/")
				l.pos++
			}
		case ch == '=':
			l.pos++
			if l.pos < len(l.input) && l.input[l.pos] == '=' {
				l.pos++
				l.emit(TOKEN_EQ, "==")
			} else {
				l.emit(TOKEN_ILLEGAL, "=")
			}
		case ch == '!':
			l.pos++
			if l.pos < len(l.input) && l.input[l.pos] == '=' {
				l.pos++
				l.emit(TOKEN_NEQ, "!=")
			} else {
				l.emit(TOKEN_ILLEGAL, "!")
			}
		case ch == '>':
			l.pos++
			if l.pos < len(l.input) && l.input[l.pos] == '=' {
				l.pos++
				l.emit(TOKEN_GTE, ">=")
			} else {
				l.emit(TOKEN_GT, ">")
			}
		case ch == '<':
			l.pos++
			if l.pos < len(l.input) && l.input[l.pos] == '=' {
				l.pos++
				l.emit(TOKEN_LTE, "<=")
			} else {
				l.emit(TOKEN_LT, "<")
			}
		case ch == '+':
			l.emit(TOKEN_PLUS, "+")
			l.pos++
		case ch == '-':
			l.emit(TOKEN_MINUS, "-")
			l.pos++
		case ch == '*':
			l.emit(TOKEN_STAR, "*")
			l.pos++
		case ch == '%':
			l.emit(TOKEN_PERCENT, "%")
			l.pos++
		case ch == ',':
			l.emit(TOKEN_COMMA, ",")
			l.pos++
		case ch == '(':
			l.emit(TOKEN_LPAREN, "(")
			l.pos++
		case ch == ')':
			l.emit(TOKEN_RPAREN, ")")
			l.pos++
		case ch == '[':
			l.emit(TOKEN_LBRACKET, "[")
			l.pos++
		case ch == ']':
			l.emit(TOKEN_RBRACKET, "]")
			l.pos++
		case ch == ':':
			l.emit(TOKEN_COLON, ":")
			l.pos++
		case ch == '|':
			l.emit(TOKEN_PIPE, "|")
			l.pos++
		default:
			l.emit(TOKEN_ILLEGAL, string(ch))
			l.pos++
		}
	}
}

