// Package segment compiles a JSON segment definition into a parameterised
// SQL condition over the contacts table (alias "c").
//
// Safety model: user input never becomes SQL text. Field names and operators
// are looked up in fixed tables; every value is bound as a positional
// parameter. Depth and size are bounded so a definition cannot produce an
// unbounded query.
package segment

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"
)

// Limits.
const (
	MaxDepth   = 5
	MaxRules   = 50
	MaxValues  = 500 // for "in" lists
	MaxTextLen = 200
)

// Node is a group ({"op":"and","rules":[...]}) or a rule
// ({"field":"tag","operator":"has","value":"vip"}).
type Node struct {
	Op       string          `json:"op,omitempty"` // and | or (groups)
	Not      bool            `json:"not,omitempty"`
	Rules    []Node          `json:"rules,omitempty"`
	Field    string          `json:"field,omitempty"`
	Operator string          `json:"operator,omitempty"`
	Value    json.RawMessage `json:"value,omitempty"`
}

// Error describes an invalid definition; Path points at the offending node.
type Error struct {
	Path    string
	Message string
}

func (e *Error) Error() string { return e.Path + ": " + e.Message }

type kind int

const (
	kText kind = iota
	kTime
	kBool
	kTag
	kGroup
)

// fields maps public field names to SQL expressions over contacts c.
var fields = map[string]struct {
	sql  string
	kind kind
}{
	"name":            {"c.name", kText},
	"profile_name":    {"c.profile_name", kText},
	"phone":           {"c.phone_e164", kText},
	"email":           {"c.email::text", kText},
	"var1":            {"c.var1", kText},
	"var2":            {"c.var2", kText},
	"var3":            {"c.var3", kText},
	"var4":            {"c.var4", kText},
	"var5":            {"c.var5", kText},
	"created_at":      {"c.created_at", kTime},
	"last_inbound_at": {"c.last_inbound_at", kTime},
	"opted_out":       {"(c.opted_out_at IS NOT NULL)", kBool},
	"tag":             {"", kTag},
	"group":           {"", kGroup},
}

var operators = map[kind][]string{
	kText:  {"eq", "neq", "contains", "not_contains", "starts_with", "in", "not_in", "is_set", "is_not_set"},
	kTime:  {"before", "after", "within_days", "older_than_days", "is_set", "is_not_set"},
	kBool:  {"is_true", "is_false"},
	kTag:   {"has", "not_has", "has_any", "has_none"},
	kGroup: {"in_group", "not_in_group"},
}

// Compiled is a WHERE condition and its arguments. Args are numbered from
// the offset given to Compile, so the condition can be combined with other
// parameters.
type Compiled struct {
	SQL  string
	Args []any
}

// Parse decodes and validates a definition.
func Parse(raw []byte) (*Node, error) {
	var n Node
	dec := json.NewDecoder(strings.NewReader(string(raw)))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&n); err != nil {
		return nil, &Error{Path: "$", Message: "invalid JSON: " + err.Error()}
	}
	if _, err := Compile(&n, 1); err != nil {
		return nil, err
	}
	return &n, nil
}

// Compile turns n into SQL; placeholders start at $offset.
func Compile(n *Node, offset int) (Compiled, error) {
	if n == nil {
		return Compiled{}, &Error{Path: "$", Message: "definition is empty"}
	}
	c := &compiler{next: offset}
	sql, err := c.node(n, "$", 1)
	if err != nil {
		return Compiled{}, err
	}
	return Compiled{SQL: sql, Args: c.args}, nil
}

type compiler struct {
	args  []any
	next  int
	rules int
}

func (c *compiler) bind(v any) string {
	c.args = append(c.args, v)
	p := fmt.Sprintf("$%d", c.next)
	c.next++
	return p
}

func (c *compiler) node(n *Node, path string, depth int) (string, error) {
	if depth > MaxDepth {
		return "", &Error{Path: path, Message: fmt.Sprintf("nesting deeper than %d levels", MaxDepth)}
	}
	isGroup := n.Op != "" || n.Rules != nil
	isRule := n.Field != "" || n.Operator != "" || n.Value != nil
	switch {
	case isGroup && isRule:
		return "", &Error{Path: path, Message: "a node is either a group (op, rules) or a rule (field, operator, value)"}
	case isGroup:
		if n.Op != "and" && n.Op != "or" {
			return "", &Error{Path: path, Message: `op must be "and" or "or"`}
		}
		if len(n.Rules) == 0 {
			return "", &Error{Path: path, Message: "a group needs at least one rule"}
		}
		parts := make([]string, 0, len(n.Rules))
		for i := range n.Rules {
			s, err := c.node(&n.Rules[i], fmt.Sprintf("%s.rules[%d]", path, i), depth+1)
			if err != nil {
				return "", err
			}
			parts = append(parts, s)
		}
		out := "(" + strings.Join(parts, " "+strings.ToUpper(n.Op)+" ") + ")"
		if n.Not {
			out = "(NOT " + out + ")"
		}
		return out, nil
	case isRule:
		c.rules++
		if c.rules > MaxRules {
			return "", &Error{Path: path, Message: fmt.Sprintf("more than %d rules", MaxRules)}
		}
		s, err := c.rule(n, path)
		if err != nil {
			return "", err
		}
		if n.Not {
			s = "(NOT " + s + ")"
		}
		return s, nil
	}
	return "", &Error{Path: path, Message: "empty node"}
}

func (c *compiler) rule(n *Node, path string) (string, error) {
	f, ok := fields[n.Field]
	if !ok {
		return "", &Error{Path: path + ".field", Message: fmt.Sprintf("unknown field %q", truncate(n.Field))}
	}
	allowed := false
	for _, op := range operators[f.kind] {
		allowed = allowed || op == n.Operator
	}
	if !allowed {
		return "", &Error{Path: path + ".operator", Message: fmt.Sprintf("operator %q is not valid for %s (use one of %s)",
			truncate(n.Operator), n.Field, strings.Join(operators[f.kind], ", "))}
	}
	vp := path + ".value"
	switch n.Operator {
	case "is_set":
		if f.kind == kTime {
			return f.sql + " IS NOT NULL", noValue(n, vp)
		}
		return "(" + f.sql + " IS NOT NULL AND " + f.sql + " <> '')", noValue(n, vp)
	case "is_not_set":
		if f.kind == kTime {
			return f.sql + " IS NULL", noValue(n, vp)
		}
		return "(" + f.sql + " IS NULL OR " + f.sql + " = '')", noValue(n, vp)
	case "is_true":
		return f.sql, noValue(n, vp)
	case "is_false":
		return "(NOT " + f.sql + ")", noValue(n, vp)
	}

	switch f.kind {
	case kText:
		if n.Operator == "in" || n.Operator == "not_in" {
			list, err := stringList(n.Value, vp)
			if err != nil {
				return "", err
			}
			s := f.sql + " = ANY(" + c.bind(list) + "::text[])"
			if n.Operator == "not_in" {
				s = "(" + f.sql + " IS NULL OR NOT (" + s + "))"
			}
			return s, nil
		}
		v, err := text(n.Value, vp)
		if err != nil {
			return "", err
		}
		switch n.Operator {
		case "eq":
			return "lower(" + f.sql + ") = lower(" + c.bind(v) + ")", nil
		case "neq":
			return "(" + f.sql + " IS NULL OR lower(" + f.sql + ") <> lower(" + c.bind(v) + "))", nil
		case "contains":
			return f.sql + " ILIKE " + c.bind("%"+escapeLike(v)+"%") + " ESCAPE '\\'", nil
		case "not_contains":
			return "(" + f.sql + " IS NULL OR " + f.sql + " NOT ILIKE " + c.bind("%"+escapeLike(v)+"%") + " ESCAPE '\\')", nil
		case "starts_with":
			return f.sql + " ILIKE " + c.bind(escapeLike(v)+"%") + " ESCAPE '\\'", nil
		}
	case kTime:
		switch n.Operator {
		case "within_days", "older_than_days":
			d, err := days(n.Value, vp)
			if err != nil {
				return "", err
			}
			cmp := ">="
			if n.Operator == "older_than_days" {
				cmp = "<"
			}
			return f.sql + " " + cmp + " now() - make_interval(days => " + c.bind(d) + "::int)", nil
		default:
			t, err := timestamp(n.Value, vp)
			if err != nil {
				return "", err
			}
			cmp := "<"
			if n.Operator == "after" {
				cmp = ">"
			}
			return f.sql + " " + cmp + " " + c.bind(t), nil
		}
	case kTag:
		if n.Operator == "has" || n.Operator == "not_has" {
			v, err := text(n.Value, vp)
			if err != nil {
				return "", err
			}
			s := "EXISTS (SELECT 1 FROM contact_tags ct JOIN tags t ON t.id = ct.tag_id WHERE ct.contact_id = c.id AND lower(t.name) = lower(" + c.bind(v) + "))"
			if n.Operator == "not_has" {
				s = "NOT " + s
			}
			return s, nil
		}
		list, err := stringList(n.Value, vp)
		if err != nil {
			return "", err
		}
		s := "EXISTS (SELECT 1 FROM contact_tags ct JOIN tags t ON t.id = ct.tag_id WHERE ct.contact_id = c.id AND lower(t.name) = ANY(" + c.bind(lowerAll(list)) + "::text[]))"
		if n.Operator == "has_none" {
			s = "NOT " + s
		}
		return s, nil
	case kGroup:
		v, err := text(n.Value, vp)
		if err != nil {
			return "", err
		}
		if !uuidLike(v) {
			return "", &Error{Path: vp, Message: "group must be a group id"}
		}
		s := "EXISTS (SELECT 1 FROM contact_group_members gm WHERE gm.contact_id = c.id AND gm.group_id = " + c.bind(v) + "::uuid)"
		if n.Operator == "not_in_group" {
			s = "NOT " + s
		}
		return s, nil
	}
	return "", &Error{Path: path, Message: "unsupported rule"}
}

func noValue(n *Node, path string) error {
	if n.Value != nil && string(n.Value) != "null" {
		return &Error{Path: path, Message: "this operator takes no value"}
	}
	return nil
}

func text(raw json.RawMessage, path string) (string, error) {
	var s string
	if err := json.Unmarshal(raw, &s); err != nil {
		return "", &Error{Path: path, Message: "must be a string"}
	}
	if s == "" || len([]rune(s)) > MaxTextLen {
		return "", &Error{Path: path, Message: fmt.Sprintf("must be 1–%d characters", MaxTextLen)}
	}
	return s, nil
}

func stringList(raw json.RawMessage, path string) ([]string, error) {
	var l []string
	if err := json.Unmarshal(raw, &l); err != nil {
		return nil, &Error{Path: path, Message: "must be a list of strings"}
	}
	if len(l) == 0 || len(l) > MaxValues {
		return nil, &Error{Path: path, Message: fmt.Sprintf("must have 1–%d entries", MaxValues)}
	}
	for _, s := range l {
		if s == "" || len([]rune(s)) > MaxTextLen {
			return nil, &Error{Path: path, Message: fmt.Sprintf("entries must be 1–%d characters", MaxTextLen)}
		}
	}
	return l, nil
}

func days(raw json.RawMessage, path string) (int, error) {
	var d int
	if err := json.Unmarshal(raw, &d); err != nil || d < 1 || d > 3650 {
		return 0, &Error{Path: path, Message: "must be a whole number of days between 1 and 3650"}
	}
	return d, nil
}

func timestamp(raw json.RawMessage, path string) (time.Time, error) {
	var s string
	if err := json.Unmarshal(raw, &s); err == nil {
		for _, layout := range []string{time.RFC3339, "2006-01-02"} {
			if t, err := time.Parse(layout, s); err == nil {
				return t.UTC(), nil
			}
		}
	}
	return time.Time{}, &Error{Path: path, Message: "must be a date (2026-01-31) or RFC 3339 time"}
}

func escapeLike(s string) string {
	return strings.NewReplacer(`\`, `\\`, `%`, `\%`, `_`, `\_`).Replace(s)
}

func lowerAll(l []string) []string {
	out := make([]string, len(l))
	for i, s := range l {
		out[i] = strings.ToLower(s)
	}
	return out
}

func uuidLike(s string) bool {
	if len(s) != 36 {
		return false
	}
	for i, r := range s {
		switch i {
		case 8, 13, 18, 23:
			if r != '-' {
				return false
			}
		default:
			if !strings.ContainsRune("0123456789abcdefABCDEF", r) {
				return false
			}
		}
	}
	return true
}

func truncate(s string) string {
	if len(s) > 40 {
		return s[:40] + "…"
	}
	return s
}

// IsInvalid reports whether err is a definition error.
func IsInvalid(err error) bool {
	var e *Error
	return errors.As(err, &e)
}
