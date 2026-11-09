// Command single-caller-ratio reports the share of unexported functions in a
// Go tree that have exactly one caller in production code.
//
// Run it from the repository root:
//
//	go run ./scripts/single-caller-ratio src/server
//
// Counting is by function name. Two methods of the same name on different types
// count as one name with two declarations, and a name is counted as
// single-caller only when its call count equals its declaration count. Test
// files are excluded from both sides: a helper called only by a test is not a
// production caller.
package main

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

func main() {
	root := "src/server"
	if len(os.Args) > 1 {
		root = os.Args[1]
	}
	files, err := parseProductionFiles(root)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	single, total := countSingleCallers(files)
	if total == 0 {
		fmt.Fprintf(os.Stderr, "no Go files under %s\n", root)
		os.Exit(1)
	}
	fmt.Printf("single-caller: %d / %d = %.1f%%\n", single, total, 100*float64(single)/float64(total))
}

func parseProductionFiles(root string) ([]*ast.File, error) {
	fset := token.NewFileSet()
	var files []*ast.File
	err := filepath.WalkDir(root, func(path string, entry fs.DirEntry, err error) error {
		if err != nil || entry.IsDir() || !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
			return err
		}
		parsed, parseErr := parser.ParseFile(fset, path, nil, 0)
		if parseErr != nil {
			return fmt.Errorf("parse %s: %w", path, parseErr)
		}
		files = append(files, parsed)
		return nil
	})
	return files, err
}

func countSingleCallers(files []*ast.File) (single, total int) {
	calls := map[string]int{}
	decls := map[string]int{}
	for _, file := range files {
		countCalls(file, calls)
		countUnexportedDecls(file, decls)
	}
	for name, declared := range decls {
		total += declared
		if calls[name] == declared {
			single += declared
		}
	}
	return single, total
}

func countCalls(file *ast.File, calls map[string]int) {
	ast.Inspect(file, func(node ast.Node) bool {
		call, ok := node.(*ast.CallExpr)
		if !ok {
			return true
		}
		switch target := call.Fun.(type) {
		case *ast.Ident:
			calls[target.Name]++
		case *ast.SelectorExpr:
			calls[target.Sel.Name]++
		}
		return true
	})
}

func countUnexportedDecls(file *ast.File, decls map[string]int) {
	for _, decl := range file.Decls {
		fn, ok := decl.(*ast.FuncDecl)
		if ok && fn.Body != nil && !ast.IsExported(fn.Name.Name) {
			decls[fn.Name.Name]++
		}
	}
}
