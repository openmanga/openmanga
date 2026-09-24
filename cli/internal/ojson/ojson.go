// Package ojson is an order-preserving JSON tree that writes the same bytes
// as JavaScript's JSON.stringify, so files round-trip with the original app.
//
// Values are: nil, bool, json.Number, float64, int, string, []any, *Object.
package ojson

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"strconv"
	"strings"
	"unicode/utf8"
)

// Object is a JSON object that remembers key insertion order.
type Object struct {
	keys []string
	m    map[string]any
}

func New() *Object { return &Object{m: map[string]any{}} }

// Obj builds an object from alternating key, value pairs.
func Obj(kv ...any) *Object {
	o := New()
	for i := 0; i+1 < len(kv); i += 2 {
		o.Set(kv[i].(string), kv[i+1])
	}
	return o
}

func (o *Object) Len() int       { return len(o.keys) }
func (o *Object) Keys() []string { return append([]string(nil), o.keys...) }

func (o *Object) Has(k string) bool { _, ok := o.m[k]; return ok }

func (o *Object) Get(k string) any { return o.m[k] }

// Set keeps the key's position if it exists, otherwise appends it.
func (o *Object) Set(k string, v any) {
	if _, ok := o.m[k]; !ok {
		o.keys = append(o.keys, k)
	}
	o.m[k] = v
}

func (o *Object) Delete(k string) {
	if _, ok := o.m[k]; !ok {
		return
	}
	delete(o.m, k)
	for i, kk := range o.keys {
		if kk == k {
			o.keys = append(o.keys[:i], o.keys[i+1:]...)
			break
		}
	}
}

func (o *Object) Obj(k string) *Object { v, _ := o.m[k].(*Object); return v }
func (o *Object) Arr(k string) []any   { v, _ := o.m[k].([]any); return v }

func (o *Object) Str(k string) string {
	switch v := o.m[k].(type) {
	case string:
		return v
	case nil:
		return ""
	default:
		return fmt.Sprint(v)
	}
}

func (o *Object) Bool(k string) bool { b, _ := o.m[k].(bool); return b }

// Num reads a number, accepting numeric strings like "2001" (old projects).
func (o *Object) Num(k string) (float64, bool) { return Num(o.m[k]) }

// NumOr returns def when the key is missing or not numeric.
func (o *Object) NumOr(k string, def float64) float64 {
	if f, ok := Num(o.m[k]); ok {
		return f
	}
	return def
}

// Clone deep-copies an object.
func (o *Object) Clone() *Object { return Clone(o).(*Object) }

// ObjPath walks nested objects, creating missing ones when create is true.
func (o *Object) ObjPath(create bool, path ...string) *Object {
	cur := o
	for _, p := range path {
		next := cur.Obj(p)
		if next == nil {
			if !create {
				return nil
			}
			next = New()
			cur.Set(p, next)
		}
		cur = next
	}
	return cur
}

// Num converts a JSON value to float64 the way JS Number() would for our data.
func Num(v any) (float64, bool) {
	switch n := v.(type) {
	case json.Number:
		f, err := n.Float64()
		return f, err == nil
	case float64:
		return n, !math.IsNaN(n)
	case int:
		return float64(n), true
	case int64:
		return float64(n), true
	case string:
		f, err := strconv.ParseFloat(strings.TrimSpace(n), 64)
		return f, err == nil
	}
	return 0, false
}

// Clone deep-copies any value of the tree.
func Clone(v any) any {
	switch t := v.(type) {
	case *Object:
		if t == nil {
			return (*Object)(nil)
		}
		c := &Object{keys: append([]string(nil), t.keys...), m: make(map[string]any, len(t.m))}
		for k, vv := range t.m {
			c.m[k] = Clone(vv)
		}
		return c
	case []any:
		c := make([]any, len(t))
		for i, vv := range t {
			c[i] = Clone(vv)
		}
		return c
	}
	return v
}

// Parse decodes JSON into the ordered tree.
func Parse(data []byte) (any, error) {
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.UseNumber()
	v, err := parseValue(dec)
	if err != nil {
		return nil, err
	}
	if _, err := dec.Token(); err != io.EOF {
		return nil, fmt.Errorf("unexpected data after JSON value")
	}
	return v, nil
}

// ParseObject decodes JSON that must be an object.
func ParseObject(data []byte) (*Object, error) {
	v, err := Parse(data)
	if err != nil {
		return nil, err
	}
	o, ok := v.(*Object)
	if !ok {
		return nil, fmt.Errorf("expected a JSON object")
	}
	return o, nil
}

func parseValue(dec *json.Decoder) (any, error) {
	tok, err := dec.Token()
	if err != nil {
		return nil, err
	}
	switch t := tok.(type) {
	case json.Delim:
		switch t {
		case '{':
			o := New()
			for dec.More() {
				kt, err := dec.Token()
				if err != nil {
					return nil, err
				}
				v, err := parseValue(dec)
				if err != nil {
					return nil, err
				}
				// JSON.parse keeps the last duplicate but the first position
				o.Set(kt.(string), v)
			}
			if _, err := dec.Token(); err != nil {
				return nil, err
			}
			return o, nil
		case '[':
			arr := []any{}
			for dec.More() {
				v, err := parseValue(dec)
				if err != nil {
					return nil, err
				}
				arr = append(arr, v)
			}
			if _, err := dec.Token(); err != nil {
				return nil, err
			}
			return arr, nil
		}
		return nil, fmt.Errorf("unexpected delimiter %v", t)
	default:
		return tok, nil
	}
}

// From converts any Go value (structs, maps) into the tree via encoding/json.
func From(v any) any {
	switch v.(type) {
	case nil, bool, string, json.Number, float64, int, *Object, []any:
		return v
	}
	b, err := json.Marshal(v)
	if err != nil {
		panic(err)
	}
	out, err := Parse(b)
	if err != nil {
		panic(err)
	}
	return out
}

// Stringify mirrors JSON.stringify(v, null, indent). indent "" means compact.
// Object keys with a nil-interface "undefined" are not modelled: nil writes null.
func Stringify(v any, indent string) []byte {
	var b bytes.Buffer
	write(&b, v, indent, "")
	return b.Bytes()
}

// MarshalJSON lets encoding/json embed ordered objects.
func (o *Object) MarshalJSON() ([]byte, error) { return Stringify(o, ""), nil }

func write(b *bytes.Buffer, v any, indent, cur string) {
	switch t := v.(type) {
	case nil:
		b.WriteString("null")
	case bool:
		if t {
			b.WriteString("true")
		} else {
			b.WriteString("false")
		}
	case json.Number:
		b.WriteString(string(t))
	case float64:
		b.WriteString(FormatNumber(t))
	case float32:
		b.WriteString(FormatNumber(float64(t)))
	case int:
		b.WriteString(strconv.Itoa(t))
	case int64:
		b.WriteString(strconv.FormatInt(t, 10))
	case string:
		writeString(b, t)
	case []string:
		arr := make([]any, len(t))
		for i, s := range t {
			arr[i] = s
		}
		write(b, arr, indent, cur)
	case []any:
		if len(t) == 0 {
			b.WriteString("[]")
			return
		}
		inner := cur + indent
		b.WriteByte('[')
		for i, e := range t {
			if i > 0 {
				b.WriteByte(',')
			}
			if indent != "" {
				b.WriteByte('\n')
				b.WriteString(inner)
			}
			write(b, e, indent, inner)
		}
		if indent != "" {
			b.WriteByte('\n')
			b.WriteString(cur)
		}
		b.WriteByte(']')
	case *Object:
		if t == nil {
			b.WriteString("null")
			return
		}
		if len(t.keys) == 0 {
			b.WriteString("{}")
			return
		}
		inner := cur + indent
		b.WriteByte('{')
		for i, k := range t.keys {
			if i > 0 {
				b.WriteByte(',')
			}
			if indent != "" {
				b.WriteByte('\n')
				b.WriteString(inner)
			}
			writeString(b, k)
			b.WriteByte(':')
			if indent != "" {
				b.WriteByte(' ')
			}
			write(b, t.m[k], indent, inner)
		}
		if indent != "" {
			b.WriteByte('\n')
			b.WriteString(cur)
		}
		b.WriteByte('}')
	default:
		write(b, From(v), indent, cur)
	}
}

// FormatNumber formats like JavaScript's Number#toString (NaN/Infinity become null, as JSON.stringify does).
func FormatNumber(f float64) string {
	if math.IsNaN(f) || math.IsInf(f, 0) {
		return "null"
	}
	if f == 0 {
		return "0"
	}
	abs := math.Abs(f)
	if abs >= 1e21 || abs < 1e-6 {
		s := strconv.FormatFloat(f, 'e', -1, 64)
		// Go: 1e-07, JS: 1e-7 ; Go: 1e+21, JS: 1e+21
		mant, exp, _ := strings.Cut(s, "e")
		sign := exp[0]
		exp = strings.TrimLeft(exp[1:], "0")
		return mant + "e" + string(sign) + exp
	}
	return strconv.FormatFloat(f, 'f', -1, 64)
}

func writeString(b *bytes.Buffer, s string) {
	b.WriteByte('"')
	for i := 0; i < len(s); {
		c := s[i]
		if c < utf8.RuneSelf {
			switch c {
			case '"':
				b.WriteString(`\"`)
			case '\\':
				b.WriteString(`\\`)
			case '\b':
				b.WriteString(`\b`)
			case '\f':
				b.WriteString(`\f`)
			case '\n':
				b.WriteString(`\n`)
			case '\r':
				b.WriteString(`\r`)
			case '\t':
				b.WriteString(`\t`)
			default:
				if c < 0x20 {
					fmt.Fprintf(b, `\u%04x`, c)
				} else {
					b.WriteByte(c)
				}
			}
			i++
			continue
		}
		r, size := utf8.DecodeRuneInString(s[i:])
		if r == utf8.RuneError && size == 1 {
			b.WriteString(`�`)
		} else {
			b.WriteString(s[i : i+size])
		}
		i += size
	}
	b.WriteByte('"')
}
