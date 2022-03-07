// Copyright 2022 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package checks

import (
	"strings"

	"golang.org/x/tools/go/analysis"
)

var (
	deniedImports   = []string{} // "io/ioutil", "encoding/json"
	DenylistImports = &analysis.Analyzer{
		Name: "denylist_imports",
		Doc:  "check for denied imports",
		Run:  runDenylistImports,
	}
)

func runDenylistImports(pass *analysis.Pass) (interface{}, error) {
	for _, file := range pass.Files {
		for _, im := range file.Imports {
			val := im.Path.Value
			val = strings.TrimPrefix(val, `"`)
			val = strings.TrimSuffix(val, `"`)
			for _, deniedImport := range deniedImports {
				if deniedImport == val {
					// Allow a exemption when there is a comment 'Allow "package_name" import'
					allowed := false
					for _, comment := range file.Comments {
						if strings.Contains(comment.Text(), "Allow \""+val+"\" import") {
							allowed = true
							break
						}
					}

					if !allowed {
						pass.Reportf(im.Path.Pos(), `"`+deniedImport+"\" is not allowed to be imported")
					}
				}
			}
		}
	}
	return nil, nil
}
