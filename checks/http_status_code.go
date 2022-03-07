package checks

import (
	"bytes"
	"go/ast"
	"go/printer"
	"go/token"

	"golang.org/x/tools/go/analysis"
)

var HttpStatusCode = &analysis.Analyzer{
	Name: "httpStatusCode",
	Doc:  "check for http status code",
	Run:  run,
}

func isIdent(expr ast.Expr, ident string) bool {
	id, ok := expr.(*ast.Ident)
	return ok && id.Name == ident
}

func isPkgDot(expr ast.Expr, pkg, name string) bool {
	sel, ok := expr.(*ast.SelectorExpr)
	return ok && isIdent(sel.X, pkg) && isIdent(sel.Sel, name)
}

func run(pass *analysis.Pass) (interface{}, error) {
	for _, file := range pass.Files {
		ast.Inspect(file, func(n ast.Node) bool {
			ce, ok := n.(*ast.CallExpr)
			if !ok {
				return true
			}
			isUsingHttpStatusCode := isPkgDot(ce.Fun, "ctx", "Status") || isPkgDot(ce.Fun, "ctx", "JSON")
			if !isUsingHttpStatusCode {
				return true
			}
			if len(ce.Args) == 0 {
				return true
			}
			_, ok = ce.Args[0].(*ast.BasicLit)
			if !ok {
				return true
			}
			pass.Reportf(n.Pos(), "Stop using literal value in ctx.Status")
			return false
		})
	}
	return nil, nil
}

// render returns the pretty-print of the given node
func render(fset *token.FileSet, x interface{}) string {
	var buf bytes.Buffer
	if err := printer.Fprint(&buf, fset, x); err != nil {
		panic(err)
	}
	return buf.String()
}
