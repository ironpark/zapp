package plist

import (
	"testing"
	"time"
)

// A boolean set to "false" must not become the string "false": CFBoolean reads
// any non-empty string as true, so coercing to a string inverts the setting.
func TestCoerceKeepsTheTypeAValueAlreadyHas(t *testing.T) {
	for _, c := range []struct {
		existing any
		literal  string
		want     any
	}{
		{true, "false", false},
		{false, "yes", true},
		{int64(42), "99", int64(99)},
		{uint64(1), "2", uint64(2)},
		{"1.0", "2.0", "2.0"},
		{1.5, "2.5", 2.5},
	} {
		got, err := Coerce(c.existing, c.literal)
		if err != nil || got != c.want {
			t.Fatalf("Coerce(%#v,%q) = %#v,%v want %#v", c.existing, c.literal, got, err, c.want)
		}
	}
}

func TestCoerceRejectsAValueTheTypeCannotHold(t *testing.T) {
	for _, c := range []struct {
		existing any
		literal  string
	}{
		{true, "perhaps"},
		{int64(1), "many"},
		{1.5, "wide"},
		{time.Now(), "someday"},
		{map[string]any{}, "scalar"},
		{[]any{}, "scalar"},
	} {
		if got, err := Coerce(c.existing, c.literal); err == nil {
			t.Fatalf("Coerce(%#v,%q) = %#v, want error", c.existing, c.literal, got)
		}
	}
}

// A new key has no type to preserve. Only booleans and whole numbers are
// guessed at, so a version such as 1.0 stays a string.
func TestInferRecognisesOnlyBooleansAndIntegers(t *testing.T) {
	for _, c := range []struct {
		literal string
		want    any
	}{
		{"true", true},
		{"YES", true},
		{"false", false},
		{"no", false},
		{"7", int64(7)},
		{"1.0", "1.0"},
		{"1.0.0", "1.0.0"},
		{"hello", "hello"},
	} {
		if got := Infer(c.literal); got != c.want {
			t.Fatalf("Infer(%q) = %#v, want %#v", c.literal, got, c.want)
		}
	}
}

func TestFormatRendersEveryScalarBare(t *testing.T) {
	when := time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC)
	for _, c := range []struct {
		value any
		want  string
	}{
		{"text", "text"},
		{true, "true"},
		{7, "7"},
		{int64(7), "7"},
		{uint32(7), "7"},
		{1.5, "1.5"},
		{when, "2026-01-02T03:04:05Z"},
		{[]byte{0xde, 0xad}, "3q0="},
	} {
		got, ok := Format(c.value)
		if !ok || got != c.want {
			t.Fatalf("Format(%#v) = %q,%v want %q", c.value, got, ok, c.want)
		}
	}
	for _, value := range []any{map[string]any{}, []any{}} {
		if _, ok := Format(value); ok {
			t.Fatalf("Format(%#v) claimed a scalar form", value)
		}
	}
}
