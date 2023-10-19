package templates

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path"
	"path/filepath"
	"reflect"
	"strings"
)

type Import struct {
	path string
	name string
}

type importedFunction struct {
	name string
}

func generateComponentsTree() {
	basePath, err := filepath.Abs(".")
	if err != nil {
		panic(err)
	}

	files, err := os.ReadDir(basePath)
	if err != nil {
		panic(err)
	}

	packageFunctions := make(map[string][]ast.FuncDecl)
	for _, file := range files {
		if !file.IsDir() {
			continue
		}

		fns, err := parseDirectoryFunctions(basePath, file.Name())
		if err != nil {
			panic(err)
		}

		for path, fn := range fns {
			packageFunctions[path] = append(packageFunctions[path], fn...)
		}
	}

	var imports []Import
	var components []Component

	packagePath := reflect.TypeOf(Import{}).PkgPath()

	for absPath, _ := range packageFunctions {
		shortPath := strings.ReplaceAll(absPath, basePath+"/", "")
		importPath := path.Join(packagePath, shortPath)
		imports = append(imports, Import{path: importPath, name: strings.ReplaceAll(shortPath, "/", "_")})

		fmt.Println(importPath)
	}

}

func parseDirectoryFunctions(base, name string) (map[string][]ast.FuncDecl, error) {
	files, err := os.ReadDir(path.Join(base, name))
	if err != nil {
		return nil, err
	}

	fns := make(map[string][]ast.FuncDecl)
	for _, file := range files {
		filePath := path.Join(base, name)
		if file.IsDir() {
			return parseDirectoryFunctions(filePath, file.Name())
		}
		if !strings.HasSuffix(file.Name(), "_templ.go") {
			continue
		}

		fset := token.NewFileSet() // positions are relative to fset
		parsed, err := parser.ParseFile(fset, path.Join(filePath, file.Name()), nil, 0)
		if err != nil {
			return nil, err
		}

		functions, err := parseFunctions(parsed)
		if err != nil {
			return nil, err
		}
		for _, fn := range functions {
			fns[filePath] = append(fns[filePath], fn)
		}
	}

	return fns, nil
}

func parseFunctions(nodes ast.Node) ([]ast.FuncDecl, error) {
	var fns []ast.FuncDecl
	ast.Inspect(nodes, func(n ast.Node) bool {
		switch x := n.(type) {
		case *ast.FuncDecl:
			fns = append(fns, *x)
		}
		return true
	})

	return fns, nil
}
