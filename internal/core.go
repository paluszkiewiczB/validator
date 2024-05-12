package internal

import (
	"errors"
	"fmt"
	"go/ast"
	"go/token"
	"go/types"
	"slices"
	"strings"

	"golang.org/x/exp/maps"
)

type Struct struct {
	Name   string
	Fields []Field
	Ast    *ast.StructType
}

type Field struct {
	Name        string
	Type        Type
	Ast         *ast.Field
	Validations Validations
}

func NewField(f *ast.Field, v Validations) Field {
	return Field{
		Name:        f.Names[0].Name,
		Type:        Type(types.ExprString(f.Type)),
		Ast:         f,
		Validations: v,
	}
}

func (f Field) IsSlice() string {
	return fmt.Sprintf("%s %s", f.Name, f.Type)
}

// Type is a parsed struct field type.
// TODO: figure out how to handle alias types `type MyString string` and `type MyString = string`.
type Type string

func (t Type) IsString() bool {
	return t == "string"
}

func (t Type) IsSlice() bool {
	return strings.HasPrefix(string(t), "[]")
}

func (t Type) IsMap() bool {
	return strings.HasPrefix(string(t), "map[")
}

func (t Type) IsPtr() bool {
	return t[0] == '*'
}

// Validations is a parsed struct tag 'required'.
// For field:
//
//	Name string `validate:"required,oneof=red green blue,oneof=r g b"`
//
// the value would be: map[string][]string{"required":{}, "oneof":{"red green blue","r g b"}}
type Validations map[string][]string

func ParseValidations(tag string) (Validations, error) {
	state := newParseState()
	for i, c := range tag {
		err := state.accept(c)
		if err != nil {
			return nil, fmt.Errorf("parsing tag: %q[%d] - rune %q,  %w", tag, i, string(c), err)
		}
	}

	return state.build()
}

func FindStructs(f *ast.File) ([]Struct, error) {
	structs := make(map[string]Struct)
	var currentType *ast.TypeSpec
	l := Log
	ast.Inspect(f, func(n ast.Node) bool {
		if t, ok := n.(*ast.TypeSpec); ok {
			l = l.With("type", t.Name)
			l.Debug("current type")
			currentType = t
			return true
		}

		s, ok := n.(*ast.StructType)
		if !ok {
			return true
		}

		for _, field := range s.Fields.List {
			l = l.With("field", field.Names[0].Name)
			l.Debug("checking field")
			if field.Tag == nil {
				l.Debug("no tag found, skipping")
				continue
			}

			l.Debug("finding validations")
			structField, err := buildField(field)
			if errors.Is(err, notFound) {
				continue
			}

			if err != nil {
				panic(err)
			}

			name := currentType.Name.Name
			thisField := Struct{Name: name, Fields: []Field{structField}, Ast: s}
			structs[name] = mergeStructs(structs[name], thisField)
			l = Log
		}

		return false
	})

	l.Debug("finished finding structs", "map", structs)
	return maps.Values(structs), nil
}

func mergeStructs(a, b Struct) Struct {
	if a.Name == "" {
		a.Name = b.Name
	} else if a.Name != b.Name {
		panic("struct names do not match")
	}

	aNames := mapSlice(a.Fields, func(f Field) string { return f.Name })
	bNames := mapSlice(b.Fields, func(f Field) string { return f.Name })

	for _, name := range bNames {
		if slices.Contains(aNames, name) {
			panic("field already exists: " + name)
		}
	}

	a.Fields = append(a.Fields, b.Fields...)

	if a.Ast == nil {
		a.Ast = b.Ast
	}

	return a
}

var notFound = errors.New("validation not found")

func buildField(f *ast.Field) (Field, error) {
	l := Log
	tag := f.Tag.Value
	if tag == "" {
		return Field{}, notFound
	}

	vals, err := ParseValidations(tag)
	if err != nil {
		return Field{}, fmt.Errorf("parsing validations: %q, %w", tag, err)
	}

	if len(vals) == 0 {
		l.Debug("no validations found")
		return Field{}, notFound
	}

	l.Debug("found", "validations", vals)
	return NewField(f, vals), nil
}

const (
	Required = "required"
	Eqfield  = "eqfield"
	Gte      = "gte"
)

// deprecated: use Generator instead.
type ValidatorFunc func(key string, str Struct, field Field) (ast.Stmt, error)

func (f ValidatorFunc) AsGenerator() Generator {
	return GeneratorFunc(func(key string, str Struct, field Field) (Generated, error) {
		stmt, err := f(key, str, field)
		return Generated{Stmts: []ast.Stmt{stmt}}, err
	})
}

type Generator interface {
	Generate(key string, str Struct, field Field) (Generated, error)
}

type Generated struct {
	// Stmts are the pieces of generated code
	Stmts []ast.Stmt
	// Imports are the import paths required by the generated Stmts
	Imports []string
}

type GeneratorFunc func(key string, str Struct, field Field) (Generated, error)

func (f GeneratorFunc) Generate(key string, str Struct, field Field) (Generated, error) {
	return f(key, str, field)
}

func GeneratorFor(validation string) Generator {
	v, ok := validators[validation]
	if !ok {
		return nil
	}

	return v
}

var validators = map[string]Generator{
	Required: forKey(Required, hasOptions(0, required)).AsGenerator(),
	Eqfield:  forKey(Eqfield, hasOptions(1, eqfield)).AsGenerator(),
	Gte:      forKey(Gte, hasOptions(1, gte)).AsGenerator(),
}

// TODO: always converts the field to float64, should be able to:
// 1. detect that field already is float64
// 2. compare fields of the same type without conversion (e.g. uint8 to uint8)
func gte(key string, str Struct, field Field) (ast.Stmt, error) {
	than := field.Validations[key][0]
	return &ast.IfStmt{
		Init: &ast.AssignStmt{
			Lhs: []ast.Expr{&ast.Ident{Name: "val"}, &ast.Ident{Name: "than"}},
			Rhs: []ast.Expr{&ast.Ident{Name: cast("float64", FieldAccess(str, field))}, &ast.Ident{Name: cast("float64", FieldNameAccess(str, than))}},
			Tok: token.DEFINE,
		},
		Cond: &ast.BinaryExpr{
			X:  &ast.Ident{Name: "val"},
			Op: token.LSS,
			Y:  &ast.Ident{Name: "than"},
		},
		Body: errorBlock(`errors.New("field \"%s\" must greater or equal than \"%s\"")`, field.Name, than),
		Else: nil,
	}, nil
}

func forKey(supported string, fun ValidatorFunc) ValidatorFunc {
	return func(key string, str Struct, field Field) (ast.Stmt, error) {
		if key != supported {
			return nil, fmt.Errorf("unsupported validation key: %q, supported: %q", key, supported)
		}

		return fun(key, str, field)
	}
}

func hasOptions(count int, fun ValidatorFunc) ValidatorFunc {
	return func(key string, str Struct, field Field) (ast.Stmt, error) {
		got := len(field.Validations[key])
		if got != count {
			return nil, fmt.Errorf("validation %q expects exactly %d option, but got: %d - %v", key, count, got, field.Validations[key])
		}

		return fun(key, str, field)
	}
}

func eqfield(key string, str Struct, field Field) (ast.Stmt, error) {
	eqTo := field.Validations[key][0]
	return &ast.IfStmt{
		Cond: &ast.BinaryExpr{
			X:  &ast.Ident{Name: FieldAccess(str, field)},
			Op: token.NEQ,
			Y:  &ast.Ident{Name: FieldNameAccess(str, eqTo)},
		},
		Body: errorBlock(`errors.New("field \"%s\" must be equal to \"%s\"")`, field.Name, eqTo),
		Else: nil,
	}, nil
}

func required(key string, str Struct, field Field) (ast.Stmt, error) {
	l := Log
	l.With("key", key)
	l.Debug("validating")

	switch t := field.Type; {
	case t.IsString():
		l.Debug("is string")
		return requireNonZeroLength(str, field)
	case t.IsSlice():
		l.Debug("is slice")
		return requireNonZeroLength(str, field)
	case t.IsMap():
		l.Debug("is map")
		return requireNonZeroLength(str, field)
	case t.IsPtr():
		l.Debug("is ptr")
		return requireNonNil(str, field)
	}

	return nil, fmt.Errorf("unsupported type for validation: %q", Required)
}

func mapSlice[S ~[]T, T, R any](slice S, f func(T) R) []R {
	out := make([]R, len(slice))
	for i, v := range slice {
		out[i] = f(v)
	}
	return out
}

func requireNonZeroLength(str Struct, field Field) (ast.Stmt, error) {
	return &ast.IfStmt{
		Cond: &ast.BinaryExpr{
			X:  &ast.CallExpr{Fun: &ast.Ident{Name: "len"}, Args: []ast.Expr{&ast.Ident{Name: FieldAccess(str, field)}}},
			Op: token.EQL,
			Y:  &ast.Ident{Name: "0"},
		},
		Body: errorBlock(`errors.New("field \"%s\" is required")`, field.Name),
	}, nil
}

func errorBlock(msg string, args ...any) *ast.BlockStmt {
	return &ast.BlockStmt{
		List: []ast.Stmt{
			&ast.ReturnStmt{
				Results: []ast.Expr{
					&ast.BasicLit{Kind: token.STRING, Value: fmt.Sprintf(msg, args...)},
				},
			},
		},
	}
}

func requireNonNil(str Struct, field Field) (ast.Stmt, error) {
	return &ast.IfStmt{
		Cond: &ast.BinaryExpr{
			X:  &ast.Ident{Name: FieldAccess(str, field)},
			Op: token.EQL,
			Y:  &ast.Ident{Name: "nil"},
		},
		Body: &ast.BlockStmt{
			List: []ast.Stmt{
				&ast.ReturnStmt{
					Results: []ast.Expr{
						&ast.BasicLit{Kind: token.STRING, Value: fmt.Sprintf(`errors.New("field \"%s\" is required")`, field.Name)},
					},
				},
			},
		},
	}, nil
}

func Receiver(s Struct) string {
	return ReceiverName(s) + " " + s.Name
}

func ReceiverName(s Struct) string {
	return strings.ToLower(s.Name[:1])
}

func FieldAccess(s Struct, f Field) string {
	return ReceiverName(s) + "." + f.Name
}

func FieldNameAccess(s Struct, f string) string {
	return ReceiverName(s) + "." + f
}

func NoError() ast.Stmt {
	return &ast.ReturnStmt{
		Results: []ast.Expr{&ast.BasicLit{Kind: token.STRING, Value: "nil"}},
	}
}

func cast(as, what string) string {
	return fmt.Sprintf("%s(%s)", as, what)
}
