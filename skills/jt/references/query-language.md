# jt Query Language — Full Reference

Read this when you need precise details on operators, precedence, functions, or edge cases beyond what the main skill covers.

## Table of Contents

1. [Operator Precedence](#operator-precedence)
2. [All Operators — Detailed](#all-operators)
3. [All Functions — Detailed](#all-functions)
4. [Dot-Path Edge Cases](#dot-path-edge-cases)
5. [Type Coercion Rules](#type-coercion-rules)

---

## Operator Precedence

Highest to lowest:

| Level | Operators | Associativity |
|---|---|---|
| 1 | `.field`, `[index]`, `[slice]`, `()` | Left |
| 2 | `\|` (pipe) | Left |
| 3 | Unary `-`, `not` | Right |
| 4 | `*`, `/`, `%` | Left |
| 5 | `+`, `-` | Left |
| 6 | `==`, `!=`, `>`, `<`, `>=`, `<=`, `contains`, `starts with`, `ends with`, `matches`, `in`, `exists`, `is null`, `is <type>` | Left |
| 7 | `and` | Left |
| 8 | `or` | Left |

Parentheses `()` override precedence at any level.

---

## All Operators

### Comparison

| Operator | Left type | Right type | Behavior |
|---|---|---|---|
| `==` | any | any | Type-aware equality. `null == null` is true. Different types are never equal. |
| `!=` | any | any | Negation of `==` |
| `>`, `<`, `>=`, `<=` | number | number | Numeric comparison |
| `>`, `<`, `>=`, `<=` | string | string | Lexicographic comparison |
| `>`, `<`, `>=`, `<=` | mixed | mixed | Numbers < strings. null is less than everything. |

### String operators

| Operator | Syntax | Behavior |
|---|---|---|
| `contains` | `.field contains "text"` | Substring match (case-sensitive). Also works for array element membership: `.tags contains "admin"` checks if `"admin"` is an element. |
| `starts with` | `.field starts with "prefix"` | String prefix check |
| `ends with` | `.field ends with "suffix"` | String suffix check |
| `matches` | `.field matches /regex/flags` | Regex match. Flags: `i` (case-insensitive). Uses Go `regexp` syntax. |

### Set and existence

| Operator | Syntax | Behavior |
|---|---|---|
| `in` | `.field in ("a", "b", "c")` | True if value equals any element in the set |
| `exists` | `.field exists` | True if field is present AND not null |
| `is null` | `.field is null` | True if field is null OR missing |
| `is <type>` | `.field is string` | Type check. Valid types: `string`, `number`, `bool`, `boolean`, `array`, `object`, `null` |

### Logical

| Operator | Syntax | Behavior |
|---|---|---|
| `and` | `A and B` | Short-circuit: if A is false, B is not evaluated |
| `or` | `A or B` | Short-circuit: if A is true, B is not evaluated |
| `not` | `not A` | Logical negation. Truthy: non-null, non-false, non-zero, non-empty-string |

### Arithmetic

| Operator | Types | Behavior |
|---|---|---|
| `+` | number, number | Addition |
| `+` | string, string | Concatenation |
| `-` | number, number | Subtraction |
| `*` | number, number | Multiplication |
| `/` | number, number | Division. Division by zero returns error. |
| `%` | number, number | Modulo |
| `-` (unary) | number | Negation |

Arithmetic with `null` propagates: `null + 5` → `null`.

---

## All Functions

### Aggregate functions

Only valid in `select` with `group by`, or standalone over all rows.

| Function | Signature | Returns | Notes |
|---|---|---|---|
| `count()` | `count()` | number | Total count of objects in group |
| `sum(expr)` | `sum(.field)` | number | Sum of numeric values. Nulls skipped. |
| `avg(expr)` | `avg(.field)` | number | Mean of numeric values. Nulls skipped. |
| `min(expr)` | `min(.field)` | any | Minimum value (numbers or strings) |
| `max(expr)` | `max(.field)` | any | Maximum value (numbers or strings) |

### Math functions

| Function | Signature | Returns | Example |
|---|---|---|---|
| `abs(x)` | `abs(.balance)` | number | `abs(-5)` → `5` |
| `floor(x)` | `floor(.price)` | number | `floor(3.7)` → `3` |
| `ceil(x)` | `ceil(.price)` | number | `ceil(3.2)` → `4` |
| `round(x)` | `round(.score)` | number | `round(3.5)` → `4` |
| `round(x, n)` | `round(.score, 2)` | number | `round(3.14159, 2)` → `3.14` |
| `sqrt(x)` | `sqrt(.variance)` | number | `sqrt(16)` → `4` |
| `pow(a, b)` | `pow(2, 10)` | number | `pow(2, 10)` → `1024` |

### String functions

| Function | Signature | Returns | Example |
|---|---|---|---|
| `length(x)` | `length(.name)` | number | String length or array length |
| `lower(x)` | `lower(.email)` | string | `lower("HELLO")` → `"hello"` |
| `upper(x)` | `upper(.code)` | string | `upper("hello")` → `"HELLO"` |
| `trim(x)` | `trim(.input)` | string | `trim("  hi  ")` → `"hi"` |
| `split(x, sep)` | `split(.csv, ",")` | array | `split("a,b,c", ",")` → `["a","b","c"]` |
| `join(x, sep)` | `join(.tags, ", ")` | string | `join(["a","b"], ", ")` → `"a, b"` |
| `replace(x, old, new)` | `replace(.s, "old", "new")` | string | String replacement |
| `substr(x, start, len)` | `substr(.s, 0, 5)` | string | `substr("hello world", 1, 3)` → `"ell"` |

### Type functions

| Function | Signature | Returns | Example |
|---|---|---|---|
| `type(x)` | `type(.field)` | string | `"string"`, `"number"`, `"boolean"`, `"null"`, `"array"`, `"object"` |
| `keys(x)` | `keys(.obj)` | array | Object keys as string array |
| `values(x)` | `values(.obj)` | array | Object values as array |
| `to_number(x)` | `to_number(.s)` | number | Parse string to number |
| `to_string(x)` | `to_string(.n)` | string | Format value as string |

### Conditional functions

| Function | Signature | Returns | Example |
|---|---|---|---|
| `coalesce(a, b, ...)` | `coalesce(.nick, .name, "anon")` | any | First non-null value |
| `if(cond, then, else)` | `if(.active, "yes", "no")` | any | Conditional expression |

### Regex functions

| Function | Signature | Returns | Example |
|---|---|---|---|
| `regex_extract(str, /pat/)` | `regex_extract(.msg, /error: (.+)/)` | string | First match (or first capture group) |
| `regex_extract_all(str, /pat/)` | `regex_extract_all(.s, /\d+/)` | array | All matches as string array |

---

## Dot-Path Edge Cases

### Missing fields return null

```bash
jt data.jsonl 'select .nonexistent'       # → null for every object
jt data.jsonl 'where .a.b.c == "x"'       # → false if .a or .a.b is missing (null != "x")
```

### Array iteration collects results

```bash
# .items[].name on {"items": [{"name":"a"}, {"name":"b"}]}
# → ["a", "b"]

# Nested iteration: .matrix[][].value
# → flattened array of all .value fields from nested arrays
```

### Recursive descent collects all matches

```bash
# ..error on {"a":{"error":"x"}, "b":{"c":{"error":"y"}}}
# → ["x", "y"]
```

### Negative array indices

```bash
.items[-1]     # last element
.items[-2]     # second to last
```

### Array slices

```bash
.items[2:5]    # indices 2, 3, 4
.items[:3]     # first 3 (indices 0, 1, 2)
.items[-3:]    # last 3
.items[::2]    # every other element (step)
```

---

## Type Coercion Rules

jt is strict about types — there's no implicit coercion in comparisons:
- `"42" == 42` → **false** (string vs number)
- `0 == false` → **false** (number vs bool)
- `"" == null` → **false** (string vs null)
- `null == null` → **true**

Use `to_number()` or `to_string()` for explicit conversion:
```bash
jt data.jsonl 'where to_number(.port) > 8000'
```

### Truthiness (for `and`, `or`, `not`, `if`)
- Truthy: non-null, non-false, non-zero numbers, non-empty strings
- Falsy: `null`, `false`, `0`, `""`
