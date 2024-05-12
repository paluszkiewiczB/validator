//nolint:unused
package main

import (
	"flag"
	"fmt"
	"go/ast"
	"go/format"
	"go/parser"
	"go/token"
	"golang.org/x/exp/slices"
	"log"
	"os"
	"os/exec"

	"github.com/paluszkiewiczB/validator/internal"

	"golang.org/x/tools/go/ast/astutil"
)

var (
	srcFile = flag.String("in", "main.go", "input file")
	dstFile = flag.String("out", "generated.go", "output file")
	dstPkg  = flag.String("outpkg", "main", "output package")
	debug   = flag.Bool("debug", false, "debug logs enabled")
)

func main() {
	flag.Parse()

	if srcFile == nil || len(*srcFile) == 0 {
		log.Fatal("input file not provided")
	}

	if dstFile == nil || len(*dstFile) == 0 {
		log.Fatal("input file not provided")
	}

	if debug != nil && *debug {
		log.Printf("using slog")
		internal.UseSlog()
	}

	log.Printf("destination package: %s", *dstPkg)

	set := token.NewFileSet()
	f := Must2(os.Open(*srcFile))
	parsed := Must2(parser.ParseFile(set, f.Name(), f, parser.AllErrors))

	structs := Must2(internal.FindStructs(parsed))

	dst := Must2(os.OpenFile(*dstFile, os.O_CREATE|os.O_WRONLY, 0o600))
	Must(dst.Truncate(0))

	methods := make([]ast.Decl, 0)
	log.Printf("validations: %#v", structs)

	var imports []string
	for _, str := range structs {
		var stmts []ast.Stmt

		for _, field := range str.Fields {
			for validation := range field.Validations {
				gen := internal.GeneratorFor(validation)
				if gen == nil {
					panic(fmt.Sprintf("validator not found for struct: %q, field: %q, validation: %q", str.Name, field.Name, validation))
				}

				generated, err := gen.Generate(validation, str, field)
				if err != nil {
					panic(err)
				}

				stmts = append(stmts, generated.Stmts...)
				imports = append(imports, generated.Imports...)
			}
		}

		// TODO: check if there are multiple structs and join them into single error (like errors.Join).
		// Make sure the solution is compatible with go-playground/validator type validator.ValidationErrors,
		// so it can be used with the translator?

		stmts = append(stmts, internal.NoError())

		if len(stmts) != 0 {
			methods = append(methods, &ast.FuncDecl{
				Doc: &ast.CommentGroup{
					List: []*ast.Comment{
						{Text: "// Validate implements Validator."},
					},
				},
				Recv: &ast.FieldList{List: []*ast.Field{{Type: &ast.Ident{Name: internal.Receiver(str)}}}},
				Name: &ast.Ident{Name: "Validate"},
				Type: &ast.FuncType{Results: &ast.FieldList{List: []*ast.Field{{Type: &ast.Ident{Name: "error"}}}}},
				Body: &ast.BlockStmt{List: stmts},
			})
		}
	}

	file := &ast.File{
		Name:  &ast.Ident{Name: *dstPkg},
		Decls: methods,
		Doc: &ast.CommentGroup{
			List: []*ast.Comment{
				// FIXME this comment is
				{Text: "// File generated automatically by validator. DO NOT EDIT."},
			},
		},
	}

	dstFs := token.NewFileSet()
	astutil.AddImport(dstFs, file, "errors")

	slices.Sort(imports)
	imports = slices.Compact(imports)
	for _, imp := range imports {
		astutil.AddImport(dstFs, file, imp)
	}

	err := format.Node(dst, dstFs, file)
	if err != nil {
		panic(err)
	}

	// FIXME this is a hack to format the output file, which should (?) be guaranteed to be properly formatted by format.Node
	err = exec.Command("go", "fmt", dst.Name()).Run()
	if err != nil {
		panic(err)
	}
}

func Must(err error) {
	if err != nil {
		panic(err)
	}
}

func Must2[V any](val V, err error) V {
	Must(err)
	return val
}
