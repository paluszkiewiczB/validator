//go:generate go run -trimpath . -in=generated_test.go -outpkg=main_test -out=generated_validations_test.go -debug=true
package main_test

import "testing"

type Required struct {
	String        string              `validate:"required"`
	StringPointer *string             `validate:"required"`
	Slice         []struct{}          `validate:"required"`
	Map           map[string]struct{} `validate:"required"`
}

func NewValidRequired() Required {
	return Required{
		String:        "string",
		StringPointer: new(string),
		Slice:         []struct{}{{}},
		Map:           map[string]struct{}{"key": {}},
	}
}

func Test_Required(t *testing.T) {
	t.Run("Valid", func(t *testing.T) {
		v := NewValidRequired()
		if err := v.Validate(); err != nil {
			t.Errorf("expected no error, got %v", err)
		}
	})

	t.Run("Invalid", func(t *testing.T) {
		cases := map[string]struct {
			mut func(r *Required)
			err string
		}{
			"empty string": {mut: func(r *Required) { r.String = "" }, err: "field \"String\" is required"},
			"nil pointer":  {mut: func(r *Required) { r.StringPointer = nil }, err: "field \"StringPointer\" is required"},
			"empty slice":  {mut: func(r *Required) { r.Slice = make([]struct{}, 0) }, err: "field \"Slice\" is required"},
			"nil slice":    {mut: func(r *Required) { r.Slice = nil }, err: "field \"Slice\" is required"},
			"empty map":    {mut: func(r *Required) { r.Map = make(map[string]struct{}) }, err: "field \"Map\" is required"},
			"nil map":      {mut: func(r *Required) { r.Map = nil }, err: "field \"Map\" is required"},
		}

		for name, c := range cases {
			t.Run(name, func(t *testing.T) {
				valid := NewValidRequired()
				c.mut(&valid)
				if err := valid.Validate(); err == nil || err.Error() != c.err {
					t.Errorf("expected error %q, got %v", c.err, err)
				}
			})
		}
	})
}

type Eqfield struct {
	Field1 string
	Field2 string `validate:"eqfield=Field1"`
}

func Test_Eqfield(t *testing.T) {
	t.Run("valid", func(t *testing.T) {
		v := Eqfield{Field1: "foo", Field2: "foo"}
		if err := v.Validate(); err != nil {
			t.Errorf("expected no error, got %v", err)
		}
	})

	t.Run("invalid", func(t *testing.T) {
		v := Eqfield{Field1: "foo", Field2: "bar"}
		if err := v.Validate(); err == nil {
			t.Errorf("expected error, got nil")
		}
	})
}

type Gte struct {
	One int
	Two float64 `validate:"gte=One"`
}

func Test_Gte(t *testing.T) {
	t.Run("valid", func(t *testing.T) {
		v := Gte{One: 1, Two: 1.2}
		if err := v.Validate(); err != nil {
			t.Errorf("expected no error, got %v", err)
		}
	})

	t.Run("invalid", func(t *testing.T) {
		v := Gte{One: 2, Two: -0.3}
		if err := v.Validate(); err == nil {
			t.Errorf("expected error, got nil")
		}
	})
}
