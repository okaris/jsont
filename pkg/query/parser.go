package query

import (
	"fmt"
	"math"
	"strconv"
	"strings"
)

// Parse parses a jt query string into an AST.
func Parse(input string) (*Query, error) {
	tokens := Lex(input)
	p := &parser{tokens: tokens}

	// Check for empty query.
	if p.peek().Type == TOKEN_EOF {
		return nil, fmt.Errorf("empty query")
	}

	// Check for illegal tokens (unterminated strings etc).
	for _, tok := range tokens {
		if tok.Type == TOKEN_ILLEGAL {
			if strings.HasPrefix(tok.Literal, "\"") {
				return nil, fmt.Errorf("unterminated string")
			}
		}
	}

	q, err := p.parseQuery()
	if err != nil {
		return nil, err
	}

	if p.peek().Type != TOKEN_EOF {
		return nil, fmt.Errorf("unexpected token %q", p.peek().Literal)
	}

	return q, nil
}

type parser struct {
	tokens []Token
	pos    int
}

func (p *parser) peek() Token {
	if p.pos >= len(p.tokens) {
		return Token{Type: TOKEN_EOF}
	}
	return p.tokens[p.pos]
}

func (p *parser) advance() Token {
	tok := p.peek()
	if p.pos < len(p.tokens) {
		p.pos++
	}
	return tok
}

func (p *parser) expect(typ TokenType) (Token, error) {
	tok := p.peek()
	if tok.Type != typ {
		return tok, fmt.Errorf("expected %d, got %q", typ, tok.Literal)
	}
	return p.advance(), nil
}

// isKeywordToken returns true if the token type is a keyword that could be used as an identifier.
func isKeywordToken(typ TokenType) bool {
	switch typ {
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

// isClauseKeyword returns true if tok starts a new clause (used to stop field parsing).
func isClauseKeyword(typ TokenType) bool {
	switch typ {
	case TOKEN_WHERE, TOKEN_SORT, TOKEN_GROUP, TOKEN_COUNT,
		TOKEN_FIRST, TOKEN_LAST, TOKEN_LIMIT, TOKEN_DISTINCT, TOKEN_EOF:
		return true
	}
	return false
}

func (p *parser) parseQuery() (*Query, error) {
	q := &Query{}

	for p.peek().Type != TOKEN_EOF {
		switch p.peek().Type {
		case TOKEN_SELECT:
			p.advance()
			sel, err := p.parseSelectFields()
			if err != nil {
				return nil, err
			}
			q.Select = sel

		case TOKEN_WHERE:
			p.advance()
			expr, err := p.parseExpression()
			if err != nil {
				return nil, err
			}
			q.Where = &WhereClause{Condition: expr}

		case TOKEN_SORT:
			p.advance()
			if _, err := p.expect(TOKEN_BY); err != nil {
				return nil, fmt.Errorf("expected 'by' after 'sort'")
			}
			sb, err := p.parseSortBy()
			if err != nil {
				return nil, err
			}
			q.SortBy = sb

		case TOKEN_GROUP:
			p.advance()
			if _, err := p.expect(TOKEN_BY); err != nil {
				return nil, fmt.Errorf("expected 'by' after 'group'")
			}
			expr, err := p.parsePrimaryExpr()
			if err != nil {
				return nil, fmt.Errorf("expected expression after 'group by'")
			}
			q.GroupBy = &GroupByClause{Expr: expr}

		case TOKEN_COUNT:
			p.advance()
			if p.peek().Type == TOKEN_BY {
				p.advance()
				expr, err := p.parsePrimaryExpr()
				if err != nil {
					return nil, fmt.Errorf("expected expression after 'count by'")
				}
				q.Count = &CountClause{By: expr}
			} else {
				q.Count = &CountClause{}
			}

		case TOKEN_FIRST:
			p.advance()
			n, err := p.parsePositiveInt()
			if err != nil {
				return nil, err
			}
			q.Limit = &LimitClause{N: n, IsLast: false}

		case TOKEN_LAST:
			p.advance()
			n, err := p.parsePositiveInt()
			if err != nil {
				return nil, err
			}
			q.Limit = &LimitClause{N: n, IsLast: true}

		case TOKEN_LIMIT:
			p.advance()
			n, err := p.parseLimitInt()
			if err != nil {
				return nil, err
			}
			lc := &LimitClause{N: n}
			if p.peek().Type == TOKEN_OFFSET {
				p.advance()
				off, err := p.parseLimitInt()
				if err != nil {
					return nil, err
				}
				lc.Offset = off
			}
			q.Limit = lc

		case TOKEN_DISTINCT:
			p.advance()
			expr, err := p.parsePrimaryExpr()
			if err != nil {
				return nil, fmt.Errorf("expected expression after 'distinct'")
			}
			q.Distinct = &DistinctClause{Expr: expr}

		case TOKEN_DOT_PATH, TOKEN_DOTDOT, TOKEN_STAR, TOKEN_IDENT,
			TOKEN_STRING, TOKEN_STRING_TEMPLATE, TOKEN_NUMBER,
			TOKEN_LPAREN, TOKEN_MINUS:
			// Implicit select
			sel, err := p.parseSelectFields()
			if err != nil {
				return nil, err
			}
			q.Select = sel

		default:
			return nil, fmt.Errorf("unexpected token %q", p.peek().Literal)
		}
	}

	return q, nil
}

func (p *parser) parseSelectFields() (*SelectClause, error) {
	var fields []SelectField
	for {
		field, err := p.parseSelectField()
		if err != nil {
			return nil, err
		}
		fields = append(fields, field)
		if p.peek().Type != TOKEN_COMMA {
			break
		}
		p.advance() // consume comma
	}
	return &SelectClause{Fields: fields}, nil
}

func (p *parser) parseSelectField() (SelectField, error) {
	// Check for wildcard: bare * or .path.*
	if p.peek().Type == TOKEN_STAR {
		p.advance()
		return SelectField{Expr: WildcardExpr{Prefix: ""}}, nil
	}
	if p.peek().Type == TOKEN_DOT_PATH {
		// Check if followed by .* (dot star)
		path := p.peek().Literal
		saved := p.pos
		p.advance()
		if p.peek().Type == TOKEN_DOT {
			p.advance()
			if p.peek().Type == TOKEN_STAR {
				p.advance()
				return SelectField{Expr: WildcardExpr{Prefix: path}}, nil
			}
			// Not a wildcard, restore
			p.pos = saved
			p.advance() // re-consume the dot path
		}
		// Parse as expression from the dot-path we already consumed
		p.pos = saved
	}

	expr, err := p.parseSelectExpr()
	if err != nil {
		return SelectField{}, err
	}
	var alias string
	if p.peek().Type == TOKEN_AS {
		p.advance()
		tok := p.peek()
		if tok.Type == TOKEN_IDENT || isKeywordToken(tok.Type) {
			p.advance()
			alias = tok.Literal
		} else {
			return SelectField{}, fmt.Errorf("expected alias name after 'as'")
		}
	}
	return SelectField{Expr: expr, Alias: alias}, nil
}

// parseSelectExpr parses a pipe-level expression for select fields.
func (p *parser) parseSelectExpr() (Expr, error) {
	return p.parsePipeExpr()
}

func (p *parser) parsePipeExpr() (Expr, error) {
	left, err := p.parseAdditiveExpr()
	if err != nil {
		return nil, err
	}
	for p.peek().Type == TOKEN_PIPE {
		p.advance()
		// After pipe, expect an ident (function name)
		tok, err := p.expect(TOKEN_IDENT)
		if err != nil {
			return nil, fmt.Errorf("expected function name after '|'")
		}
		left = PipeExpr{Left: left, Right: FuncCall{Name: tok.Literal}}
	}
	return left, nil
}

func (p *parser) parseSortBy() (*SortByClause, error) {
	var fields []SortField
	for {
		expr, err := p.parsePrimaryExpr()
		if err != nil {
			return nil, fmt.Errorf("expected expression in sort by")
		}
		desc := false
		if p.peek().Type == TOKEN_DESC {
			p.advance()
			desc = true
		} else if p.peek().Type == TOKEN_ASC {
			p.advance()
		}
		fields = append(fields, SortField{Expr: expr, Desc: desc})
		if p.peek().Type != TOKEN_COMMA {
			break
		}
		p.advance()
	}
	return &SortByClause{Fields: fields}, nil
}

func (p *parser) parsePositiveInt() (int, error) {
	if p.peek().Type == TOKEN_MINUS {
		return 0, fmt.Errorf("expected positive number")
	}
	tok := p.peek()
	if tok.Type != TOKEN_NUMBER {
		return 0, fmt.Errorf("expected number")
	}
	p.advance()
	n, err := strconv.ParseFloat(tok.Literal, 64)
	if err != nil {
		return 0, fmt.Errorf("invalid number %q", tok.Literal)
	}
	if n != math.Trunc(n) {
		return 0, fmt.Errorf("expected integer, got %s", tok.Literal)
	}
	if n <= 0 {
		return 0, fmt.Errorf("expected positive number")
	}
	return int(n), nil
}

func (p *parser) parseLimitInt() (int, error) {
	tok := p.peek()
	if tok.Type != TOKEN_NUMBER {
		return 0, fmt.Errorf("expected number")
	}
	p.advance()
	n, err := strconv.ParseFloat(tok.Literal, 64)
	if err != nil {
		return 0, fmt.Errorf("invalid number %q", tok.Literal)
	}
	if n != math.Trunc(n) {
		return 0, fmt.Errorf("expected integer, got %s", tok.Literal)
	}
	return int(n), nil
}

// ── Expression parsing with precedence ───────────────────────────────

// Precedence (lowest to highest):
// or
// and
// not (unary)
// comparison (==, !=, >, <, >=, <=, contains, starts with, ends with, matches, in, exists, is)
// additive (+, -)
// multiplicative (*, /, %)
// unary (-)
// postfix ([], pipe)
// primary

func (p *parser) parseExpression() (Expr, error) {
	return p.parseOrExpr()
}

func (p *parser) parseOrExpr() (Expr, error) {
	left, err := p.parseAndExpr()
	if err != nil {
		return nil, err
	}
	for p.peek().Type == TOKEN_OR {
		p.advance()
		right, err := p.parseAndExpr()
		if err != nil {
			return nil, err
		}
		left = BinaryOp{Left: left, Op: "or", Right: right}
	}
	return left, nil
}

func (p *parser) parseAndExpr() (Expr, error) {
	left, err := p.parseNotExpr()
	if err != nil {
		return nil, err
	}
	for p.peek().Type == TOKEN_AND {
		p.advance()
		right, err := p.parseNotExpr()
		if err != nil {
			return nil, err
		}
		left = BinaryOp{Left: left, Op: "and", Right: right}
	}
	return left, nil
}

func (p *parser) parseNotExpr() (Expr, error) {
	if p.peek().Type == TOKEN_NOT {
		p.advance()
		expr, err := p.parseNotExpr()
		if err != nil {
			return nil, err
		}
		return UnaryOp{Op: "not", Expr: expr}, nil
	}
	return p.parseComparisonExpr()
}

func (p *parser) parseComparisonExpr() (Expr, error) {
	left, err := p.parseAdditiveExpr()
	if err != nil {
		return nil, err
	}

	switch p.peek().Type {
	case TOKEN_EQ:
		p.advance()
		right, err := p.parseAdditiveExpr()
		if err != nil {
			return nil, fmt.Errorf("expected expression after '=='")
		}
		return BinaryOp{Left: left, Op: "==", Right: right}, nil
	case TOKEN_NEQ:
		p.advance()
		right, err := p.parseAdditiveExpr()
		if err != nil {
			return nil, fmt.Errorf("expected expression after '!='")
		}
		return BinaryOp{Left: left, Op: "!=", Right: right}, nil
	case TOKEN_GT:
		p.advance()
		right, err := p.parseAdditiveExpr()
		if err != nil {
			return nil, fmt.Errorf("expected expression after '>'")
		}
		return BinaryOp{Left: left, Op: ">", Right: right}, nil
	case TOKEN_LT:
		p.advance()
		right, err := p.parseAdditiveExpr()
		if err != nil {
			return nil, fmt.Errorf("expected expression after '<'")
		}
		return BinaryOp{Left: left, Op: "<", Right: right}, nil
	case TOKEN_GTE:
		p.advance()
		right, err := p.parseAdditiveExpr()
		if err != nil {
			return nil, fmt.Errorf("expected expression after '>='")
		}
		return BinaryOp{Left: left, Op: ">=", Right: right}, nil
	case TOKEN_LTE:
		p.advance()
		right, err := p.parseAdditiveExpr()
		if err != nil {
			return nil, fmt.Errorf("expected expression after '<='")
		}
		return BinaryOp{Left: left, Op: "<=", Right: right}, nil
	case TOKEN_CONTAINS:
		p.advance()
		right, err := p.parseAdditiveExpr()
		if err != nil {
			return nil, fmt.Errorf("expected expression after 'contains'")
		}
		return ContainsExpr{Haystack: left, Needle: right}, nil
	case TOKEN_STARTS:
		p.advance()
		if p.peek().Type != TOKEN_WITH {
			return nil, fmt.Errorf("expected 'with' after 'starts'")
		}
		p.advance()
		right, err := p.parseAdditiveExpr()
		if err != nil {
			return nil, fmt.Errorf("expected expression after 'starts with'")
		}
		return StartsWithExpr{Expr: left, Prefix: right}, nil
	case TOKEN_ENDS:
		p.advance()
		if p.peek().Type != TOKEN_WITH {
			return nil, fmt.Errorf("expected 'with' after 'ends'")
		}
		p.advance()
		right, err := p.parseAdditiveExpr()
		if err != nil {
			return nil, fmt.Errorf("expected expression after 'ends with'")
		}
		return EndsWithExpr{Expr: left, Suffix: right}, nil
	case TOKEN_MATCHES:
		p.advance()
		right, err := p.parsePrimaryExpr()
		if err != nil {
			return nil, fmt.Errorf("expected regex after 'matches'")
		}
		return MatchesExpr{Expr: left, Regex: right}, nil
	case TOKEN_IN:
		p.advance()
		if p.peek().Type != TOKEN_LPAREN {
			return nil, fmt.Errorf("expected '(' after 'in'")
		}
		p.advance()
		var values []Expr
		for p.peek().Type != TOKEN_RPAREN {
			if len(values) > 0 {
				if _, err := p.expect(TOKEN_COMMA); err != nil {
					return nil, fmt.Errorf("expected ',' or ')' in 'in' list")
				}
			}
			val, err := p.parsePrimaryExpr()
			if err != nil {
				return nil, err
			}
			values = append(values, val)
		}
		p.advance() // consume )
		return InExpr{Expr: left, Values: values}, nil
	case TOKEN_EXISTS:
		p.advance()
		return ExistsExpr{Expr: left}, nil
	case TOKEN_IS:
		p.advance()
		if p.peek().Type == TOKEN_NULL {
			p.advance()
			return IsNullExpr{Expr: left}, nil
		}
		// is type
		tok := p.advance()
		return IsTypeExpr{Expr: left, TypeName: tok.Literal}, nil
	}

	return left, nil
}

func (p *parser) parseAdditiveExpr() (Expr, error) {
	left, err := p.parseMultiplicativeExpr()
	if err != nil {
		return nil, err
	}
	for p.peek().Type == TOKEN_PLUS || p.peek().Type == TOKEN_MINUS {
		op := p.advance().Literal
		right, err := p.parseMultiplicativeExpr()
		if err != nil {
			return nil, err
		}
		left = BinaryOp{Left: left, Op: op, Right: right}
	}
	return left, nil
}

func (p *parser) parseMultiplicativeExpr() (Expr, error) {
	left, err := p.parseUnaryExpr()
	if err != nil {
		return nil, err
	}
	for p.peek().Type == TOKEN_STAR || p.peek().Type == TOKEN_SLASH || p.peek().Type == TOKEN_PERCENT {
		op := p.advance().Literal
		right, err := p.parseUnaryExpr()
		if err != nil {
			return nil, err
		}
		left = BinaryOp{Left: left, Op: op, Right: right}
	}
	return left, nil
}

func (p *parser) parseUnaryExpr() (Expr, error) {
	if p.peek().Type == TOKEN_MINUS {
		p.advance()
		expr, err := p.parseUnaryExpr()
		if err != nil {
			return nil, err
		}
		return UnaryOp{Op: "-", Expr: expr}, nil
	}
	return p.parsePostfixExpr()
}

func (p *parser) parsePostfixExpr() (Expr, error) {
	expr, err := p.parsePrimaryExpr()
	if err != nil {
		return nil, err
	}

	for p.peek().Type == TOKEN_LBRACKET {
		p.advance() // consume [

		// Empty brackets: array iterator
		if p.peek().Type == TOKEN_RBRACKET {
			p.advance()
			expr = ArrayIterator{Expr: expr}
			continue
		}

		// Try to parse index or slice
		// Could be: [N], [N:M], [N:], [:M], [-N:]
		first, hasFirst, err := p.parseBracketNumber()
		if err != nil {
			return nil, err
		}

		if p.peek().Type == TOKEN_COLON {
			// Slice
			p.advance()
			second, hasSecond, err := p.parseBracketNumber()
			if err != nil {
				return nil, err
			}
			if _, err := p.expect(TOKEN_RBRACKET); err != nil {
				return nil, fmt.Errorf("expected ']'")
			}
			slice := ArraySlice{Expr: expr}
			if hasFirst {
				v := first
				slice.Start = &v
			}
			if hasSecond {
				v := second
				slice.End = &v
			}
			expr = slice
		} else {
			// Index
			if !hasFirst {
				return nil, fmt.Errorf("expected number in array index")
			}
			if _, err := p.expect(TOKEN_RBRACKET); err != nil {
				return nil, fmt.Errorf("expected ']'")
			}
			expr = ArrayIndex{Expr: expr, Index: first}
		}
	}

	return expr, nil
}

func (p *parser) parseBracketNumber() (int, bool, error) {
	neg := false
	if p.peek().Type == TOKEN_MINUS {
		neg = true
		p.advance()
	}
	if p.peek().Type == TOKEN_NUMBER {
		tok := p.advance()
		n, err := strconv.Atoi(tok.Literal)
		if err != nil {
			return 0, false, fmt.Errorf("invalid number %q", tok.Literal)
		}
		if neg {
			n = -n
		}
		return n, true, nil
	}
	if neg {
		return 0, false, fmt.Errorf("expected number after '-'")
	}
	return 0, false, nil
}

func (p *parser) parsePrimaryExpr() (Expr, error) {
	tok := p.peek()

	switch tok.Type {
	case TOKEN_DOT_PATH:
		p.advance()
		return DotPath{Path: tok.Literal}, nil

	case TOKEN_DOTDOT:
		p.advance()
		ident, err := p.expect(TOKEN_IDENT)
		if err != nil {
			return nil, fmt.Errorf("expected field name after '..'")
		}
		return RecursiveDescent{Field: ident.Literal}, nil

	case TOKEN_STRING:
		p.advance()
		return StringLiteral{Value: tok.Literal}, nil

	case TOKEN_NUMBER:
		p.advance()
		n, err := strconv.ParseFloat(tok.Literal, 64)
		if err != nil {
			return nil, fmt.Errorf("invalid number %q", tok.Literal)
		}
		return NumberLiteral{Value: n}, nil

	case TOKEN_TRUE:
		p.advance()
		return BoolLiteral{Value: true}, nil

	case TOKEN_FALSE:
		p.advance()
		return BoolLiteral{Value: false}, nil

	case TOKEN_NULL:
		p.advance()
		return NullLiteral{}, nil

	case TOKEN_REGEX:
		p.advance()
		pattern, flags := parseRegexLiteral(tok.Literal)
		return RegexLiteral{Pattern: pattern, Flags: flags}, nil

	case TOKEN_STRING_TEMPLATE:
		p.advance()
		parts, err := parseStringTemplate(tok.Literal)
		if err != nil {
			return nil, err
		}
		return StringTemplate{Parts: parts}, nil

	case TOKEN_IDENT:
		// Could be function call
		name := tok.Literal
		p.advance()
		if p.peek().Type == TOKEN_LPAREN {
			return p.parseFuncCall(name)
		}
		// Bare ident used as expression (e.g. type name after "is")
		return DotPath{Path: "." + name}, nil

	case TOKEN_COUNT:
		// count() as function call in select context
		name := tok.Literal
		p.advance()
		if p.peek().Type == TOKEN_LPAREN {
			return p.parseFuncCall(name)
		}
		// Bare "count" used as field reference (e.g. in sort by count)
		return DotPath{Path: "." + name}, nil

	case TOKEN_LPAREN:
		p.advance()
		expr, err := p.parseOrExpr()
		if err != nil {
			return nil, err
		}
		if _, err := p.expect(TOKEN_RPAREN); err != nil {
			return nil, fmt.Errorf("expected ')'")
		}
		return expr, nil

	default:
		return nil, fmt.Errorf("unexpected token %q", tok.Literal)
	}
}

func (p *parser) parseFuncCall(name string) (Expr, error) {
	p.advance() // consume (
	var args []Expr
	for p.peek().Type != TOKEN_RPAREN {
		if len(args) > 0 {
			if _, err := p.expect(TOKEN_COMMA); err != nil {
				return nil, fmt.Errorf("expected ',' or ')' in function arguments")
			}
		}
		arg, err := p.parseSelectExpr()
		if err != nil {
			return nil, err
		}
		args = append(args, arg)
	}
	p.advance() // consume )
	return FuncCall{Name: name, Args: args}, nil
}

// parseRegexLiteral parses "/pattern/flags" into pattern and flags.
func parseRegexLiteral(lit string) (string, string) {
	// lit is like "/pattern/flags" — strip the leading /
	lit = lit[1:] // remove leading /
	lastSlash := strings.LastIndex(lit, "/")
	pattern := lit[:lastSlash]
	flags := lit[lastSlash+1:]
	return pattern, flags
}

// parseStringTemplate parses the raw string template content.
// The raw literal has \( ... ) interpolation sequences.
func parseStringTemplate(raw string) ([]Expr, error) {
	var parts []Expr
	i := 0
	var textBuf strings.Builder

	for i < len(raw) {
		if i+1 < len(raw) && raw[i] == '\\' && raw[i+1] == '(' {
			// Flush text buffer
			if textBuf.Len() > 0 {
				parts = append(parts, StringLiteral{Value: textBuf.String()})
				textBuf.Reset()
			}
			// Find matching )
			i += 2 // skip \(
			depth := 1
			start := i
			for i < len(raw) && depth > 0 {
				if raw[i] == '(' {
					depth++
				} else if raw[i] == ')' {
					depth--
				}
				if depth > 0 {
					i++
				}
			}
			exprStr := raw[start:i]
			i++ // skip )

			// Parse the interpolated expression
			tokens := Lex(exprStr)
			subParser := &parser{tokens: tokens}
			expr, err := subParser.parsePrimaryExpr()
			if err != nil {
				return nil, fmt.Errorf("error in template interpolation: %v", err)
			}
			parts = append(parts, expr)
		} else {
			textBuf.WriteByte(raw[i])
			i++
		}
	}
	if textBuf.Len() > 0 {
		parts = append(parts, StringLiteral{Value: textBuf.String()})
	}
	return parts, nil
}
