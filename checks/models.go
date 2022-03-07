// Copyright 2020 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package checks

import (
	"bytes"
	"errors"
	"go/ast"
	"go/format"
	"go/token"
	"os/exec"
	"strings"

	"golang.org/x/tools/go/analysis"
)

var Models = &analysis.Analyzer{
	Name: "models",
	Doc:  "check models for black-listed packages.",
	Run:  checkModels,
}

var ModelsSession = &analysis.Analyzer{
	Name: "modelssession",
	Doc:  "check models for misuse of session.",
	Run:  checkModelsSession,
}

var (
	modelsImpBlockList = []string{
		"code.gitea.io/gitea/modules/git",
	}
)

func checkModels(pass *analysis.Pass) (interface{}, error) {
	if !strings.EqualFold(pass.Pkg.Path(), "code.gitea.io/gitea/models") {
		return nil, nil
	}

	if _, err := exec.LookPath("go"); err != nil {
		return nil, errors.New("go was not found in the PATH")
	}

	impsCmd := exec.Command("go", "list", "-f", `{{join .Imports "\n"}}`, "code.gitea.io/gitea/models")
	impsOut, err := impsCmd.Output()
	if err != nil {
		return nil, err
	}

	imps := strings.Split(string(impsOut), "\n")
	for _, imp := range imps {
		if stringInSlice(imp, modelsImpBlockList) {
			pass.Reportf(0, "code.gitea.io/gitea/models cannot import the following packages: %s", modelsImpBlockList)
			return nil, nil
		}
	}

	return nil, nil
}

func checkModelsSession(pass *analysis.Pass) (interface{}, error) {
	if !strings.EqualFold(pass.Pkg.Path(), "code.gitea.io/gitea/models") {
		return nil, nil
	}

	for _, file := range pass.Files {
		for _, decl := range file.Decls {
			// We only care about function declarations
			fnDecl, ok := decl.(*ast.FuncDecl)
			if !ok {
				continue
			}

			fnname := formatFunctionName(fnDecl)

			// OK now we to step through each line in the function and ensure that if we open a session we close it or return the session
			w := walker{
				fname:  file.Name.String(),
				fnname: fnname,
				pass:   pass,
			}
			ast.Walk(&w, fnDecl.Body)

			// Finally we may have a named return so we need to check if the session is returned as one of these
			if w.HasUnclosedSession() && w.sessionName != "" {
				w.closesSession = fnDeclHasNamedReturn(fnDecl, w.sessionName)
			}

			if w.HasUnclosedSession() {
				pass.Reportf(fnDecl.Pos(), "%s opens session but does not close it", fnname)
			}
		}
	}

	return nil, nil
}

// fnDeclHasNamedReturn checks if the function declaration has a named return with the provided name
func fnDeclHasNamedReturn(fnDecl *ast.FuncDecl, name string) bool {
	if fnDecl.Type.Results == nil {
		return false
	}
	for _, result := range fnDecl.Type.Results.List {
		if len(result.Names) != 1 {
			continue
		}
		if result.Names[0].Name == name {
			return true
		}
	}
	return false
}

// formatNode is a convenience function for printing a node
func formatNode(node ast.Node) string {
	buf := new(bytes.Buffer)
	_ = format.Node(buf, token.NewFileSet(), node)
	return buf.String()
}

// formatFunctionName returns the function name as called by the source
func formatFunctionName(fnDecl *ast.FuncDecl) string {
	fnname := fnDecl.Name.Name
	if fnDecl.Recv != nil && fnDecl.Recv.List[fnDecl.Recv.NumFields()-1] != nil {
		ns := formatNode(fnDecl.Recv.List[fnDecl.Recv.NumFields()-1].Type)
		if ns[0] == '*' {
			ns = ns[1:]
		}
		fnname = ns + "." + fnname
	}
	return fnname
}

// walker looks for unclosed sessions
type walker struct {
	fname          string
	fnname         string
	pass           *analysis.Pass
	createsSession bool
	closesSession  bool
	sessionName    string
}

func (w *walker) HasUnclosedSession() bool {
	return w.createsSession && !w.closesSession
}

func (w *walker) Visit(node ast.Node) ast.Visitor {
	if node == nil {
		return nil
	}

	switch t := node.(type) {

	case *ast.ExprStmt:
		if isCloseSessionExpr(t.X, w.sessionName) {
			w.closesSession = true
			return nil
		}
	case *ast.AssignStmt:
		if len(t.Lhs) != 1 && len(t.Rhs) != 1 {
			break
		}

		name, ok := t.Lhs[0].(*ast.Ident)
		if !ok {
			break
		}

		if isNewSession(t.Rhs[0]) {
			w.createsSession = true
			w.sessionName = name.Name
			return nil
		}
		if isCloseSessionExpr(t.Rhs[0], w.sessionName) {
			w.closesSession = true
			return nil
		}
	case *ast.DeferStmt:
		if isCloseSessionExpr(t.Call, w.sessionName) {
			w.closesSession = true
			return nil
		}
	case *ast.ReturnStmt:
		for _, expr := range t.Results {
			id, ok := expr.(*ast.Ident)
			if !ok {
				continue
			}
			if w.sessionName != "" && id.Name == w.sessionName {
				w.closesSession = true
			}
		}
	}

	return w
}

// isCloseSessionExpr checks whether a provided expression represents a call to sess.Close
func isCloseSessionExpr(expr ast.Expr, name string) bool {
	if name == "" {
		return false
	}
	call, ok := expr.(*ast.CallExpr)
	if ok {
		expr = call.Fun
	}
	sel, ok := expr.(*ast.SelectorExpr)
	if !ok {
		return false
	}
	id, ok := sel.X.(*ast.Ident)
	if !ok {
		return false
	}

	if id.Name != name || sel.Sel.Name != "Close" {
		return false
	}

	return true
}

// isNewSession checks whether a provided expression represents a call to x.NewSession()
func isNewSession(expr ast.Expr) bool {
	value, ok := expr.(*ast.CallExpr)
	if !ok {
		return false
	}
	sel, ok := value.Fun.(*ast.SelectorExpr)
	if !ok {
		return false
	}
	id, ok := sel.X.(*ast.Ident)
	if !ok {
		return false
	}
	if id.Name != "x" || sel.Sel.Name != "NewSession" {
		return false
	}
	return true
}
