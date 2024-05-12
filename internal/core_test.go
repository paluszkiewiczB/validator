package internal_test

import (
	"fmt"
	"strings"
	"testing"

	"github.com/paluszkiewiczB/validator/internal"
)

type testLog struct {
	args []any
	t    *testing.T
}

func (t *testLog) Debug(msg string, keyvals ...interface{}) {
	t.t.Helper()
	args := make([]any, 0, len(keyvals)+len(t.args))
	args = append(args, t.args...)
	args = append(args, keyvals...)
	t.t.Log(msg + formatArgs(args))
}

func formatArgs(args []any) string {
	sb := strings.Builder{}
	for i := range len(args) / 2 {
		sb.WriteRune(' ')
		k, v := args[i*2], args[i*2+1]
		sb.WriteString(toString(k))
		sb.WriteRune('=')
		sb.WriteString(toString(v))
	}
	return sb.String()
}

func toString(a any) string {
	switch s := a.(type) {
	case string:
		return s
	case fmt.Stringer:
		return s.String()
	default:
		return fmt.Sprintf("%v", a)
	}
}

func (t *testLog) With(args ...any) internal.Logger {
	t.t.Helper()
	newArgs := make([]any, 0, len(args)+len(t.args))
	newArgs = append(newArgs, t.args...)
	newArgs = append(newArgs, args...)
	return &testLog{args: newArgs, t: t.t}
}

func newTestLog(t *testing.T) internal.Logger {
	t.Helper()
	return &testLog{t: t}
}

func Test_ParseValidations(t *testing.T) {
	internal.Log = newTestLog(t)

	// FIXME: write an actual parser for this, or find something ready to use
	cases := map[string]map[string][]string{
		raw(`json:"foo"`):                                           nil,
		raw(`validate:"required"`):                                  {"required": {}},
		raw(`validate:"required" json:foo"`):                        {"required": {}},
		raw(`validate:"required,oneof=red green blue,oneof=r g b"`): {"required": {}, "oneof": {"red green blue", "r g b"}},
	}

	for in, expected := range cases {
		t.Run(in, func(t *testing.T) {
			out, err := internal.ParseValidations(in)
			if err != nil {
				t.Errorf("unexpected error: %v", err)
			}
			sameValidations(t, expected, out)
		})
	}
}

func sameValidations(t *testing.T, a, b map[string][]string) {
	t.Helper()
	if len(a) != len(b) {
		t.Errorf("expected length %d, got %d for: %v vs %v", len(a), len(b), a, b)
		return
	}

	for k, v := range a {
		if bv, ok := b[k]; !ok {
			t.Errorf("missing key in second map: %s", k)
		} else {
			if len(v) != len(bv) {
				t.Errorf("expected length %v, got %v", v, bv)
				return
			}

			for i, vv := range v {
				if vv != bv[i] {
					t.Errorf("at position: %d for key: %s expected %s, got %s", i, k, vv, bv[i])
				}
			}
		}
	}
}

func raw(s string) string {
	return fmt.Sprintf("`%s`", s)
}
