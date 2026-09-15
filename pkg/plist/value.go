package plist

import (
	"encoding/base64"
	"fmt"
	"strconv"
	"strings"
	"time"
)

// Coerce turns a textual value, such as a command-line argument, into a
// property list value of the same type as existing. Preserving the type matters
// most for booleans: CFBoolean reads any non-empty string as true, so writing
// the string "false" over a <false/> would invert the setting.
//
// A nil existing means the key is new and has no type to keep, so Infer picks
// one. The types recognised are those Parse produces.
func Coerce(existing any, literal string) (any, error) {
	switch existing.(type) {
	case nil:
		return Infer(literal), nil
	case string:
		return literal, nil
	case bool:
		return ParseBool(literal)
	case int, int8, int16, int32, int64:
		n, err := strconv.ParseInt(literal, 10, 64)
		if err != nil {
			return nil, fmt.Errorf("%q is not an integer", literal)
		}
		return n, nil
	case uint, uint8, uint16, uint32, uint64:
		n, err := strconv.ParseUint(literal, 10, 64)
		if err != nil {
			return nil, fmt.Errorf("%q is not an integer", literal)
		}
		return n, nil
	case float32, float64:
		f, err := strconv.ParseFloat(literal, 64)
		if err != nil {
			return nil, fmt.Errorf("%q is not a real number", literal)
		}
		return f, nil
	case time.Time:
		t, err := time.Parse(time.RFC3339, literal)
		if err != nil {
			return nil, fmt.Errorf("%q is not an RFC 3339 date", literal)
		}
		return t, nil
	case []byte:
		b, err := base64.StdEncoding.DecodeString(literal)
		if err != nil {
			return nil, fmt.Errorf("%q is not base64 data", literal)
		}
		return b, nil
	default:
		return nil, fmt.Errorf("cannot replace a %s with a single value", Kind(existing))
	}
}

// Infer types a value for a key that does not exist yet. Only booleans and
// whole numbers are recognised: a version such as 1.0 or 1.0.0 has to stay a
// string, so real numbers are never guessed at.
func Infer(literal string) any {
	if b, err := ParseBool(literal); err == nil {
		return b
	}
	if n, err := strconv.ParseInt(literal, 10, 64); err == nil {
		return n
	}
	return literal
}

// ParseBool reads the spellings a property list uses for a boolean.
func ParseBool(literal string) (bool, error) {
	switch strings.ToLower(literal) {
	case "true", "yes", "1":
		return true, nil
	case "false", "no", "0":
		return false, nil
	}
	return false, fmt.Errorf("%q is not a boolean", literal)
}

// Format renders a value as text. A scalar prints bare, so a caller can use it
// directly; ok is false for a dictionary or array, which has no single-line
// form and should be marshalled instead.
func Format(value any) (string, bool) {
	switch v := value.(type) {
	case string:
		return v, true
	case bool:
		return strconv.FormatBool(v), true
	case int:
		return strconv.FormatInt(int64(v), 10), true
	case int8:
		return strconv.FormatInt(int64(v), 10), true
	case int16:
		return strconv.FormatInt(int64(v), 10), true
	case int32:
		return strconv.FormatInt(int64(v), 10), true
	case int64:
		return strconv.FormatInt(v, 10), true
	case uint:
		return strconv.FormatUint(uint64(v), 10), true
	case uint8:
		return strconv.FormatUint(uint64(v), 10), true
	case uint16:
		return strconv.FormatUint(uint64(v), 10), true
	case uint32:
		return strconv.FormatUint(uint64(v), 10), true
	case uint64:
		return strconv.FormatUint(v, 10), true
	case float32:
		return strconv.FormatFloat(float64(v), 'g', -1, 32), true
	case float64:
		return strconv.FormatFloat(v, 'g', -1, 64), true
	case time.Time:
		return v.Format(time.RFC3339), true
	case []byte:
		return base64.StdEncoding.EncodeToString(v), true
	}
	return "", false
}
