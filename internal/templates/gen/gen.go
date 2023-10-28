package gen

import (
	"errors"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path"
	"path/filepath"
	"reflect"
	"runtime"
	"strings"
)

type Import struct {
	path string
	name string
}

func generateLayoutsTree() {
	fmt.Println("Started generating layouts_gen.go...")

	// Get tha path to a directory where this file is located.
	_, filename, _, ok := runtime.Caller(1)
	if !ok {
		panic("No caller information")
	}
	// Get the path to the package and base path to the layouts directory.
	packagePath := strings.ReplaceAll(reflect.TypeOf(Import{}).PkgPath(), "/gen", "")
	basePath, err := filepath.Abs(filepath.Join(filepath.Dir(filename), "../"))
	if err != nil {
		panic(err)
	}

	// Parse all functions from the layouts directory.
	fns, err := parseDirectoryFunctions(basePath, "layouts")
	if err != nil {
		panic(err)
	}

	// Generate a slice of import statements.
	imports := []Import{
		{path: "github.com/a-h/templ"},
	}
	for absPath := range fns {
		shortPath := strings.ReplaceAll(absPath, basePath+"/", "")
		importPath := path.Join(packagePath, shortPath)
		importName := "im_" + strings.ReplaceAll(shortPath, "/", "_")
		imports = append(imports, Import{path: importPath, name: importName})
	}

	// Generate the contents of layouts_gen.go.
	var generatedFile string
	generatedFile += "package templates\n\n"
	generatedFile += "import (\n"
	for _, imp := range imports {
		if imp.name == "" {
			generatedFile += fmt.Sprintf("\t\"%s\"\n", imp.path)
		} else {
			generatedFile += fmt.Sprintf("\t%s \"%s\"\n", imp.name, imp.path)
		}
	}
	generatedFile += ")\n\n"
	generatedFile += "var layouts = make(map[string]func(...templ.Component) templ.Component)\n\n"
	generatedFile += "func init() {\n"
	for absPath := range fns {
		shortPath := strings.ReplaceAll(absPath, basePath+"/", "")
		for _, fn := range fns[absPath] {
			if !fn.Name.IsExported() {
				// Skip local functions
				continue
			}
			mapKey := filepath.Join(shortPath, fn.Name.Name)
			mapKeyLower := strings.ToLower(mapKey)
			fnName := strings.ReplaceAll(shortPath, "/", "_") + "." + fn.Name.Name
			generatedFile += fmt.Sprintf("\tlayouts[\"%s\"] = im_%s\n", mapKey, fnName)
			generatedFile += fmt.Sprintf("\tlayouts[\"%s\"] = layouts[\"%s\"]\n", mapKeyLower, mapKey)
		}
	}
	generatedFile += "}\n"

	// Write the contents to layouts_gen.go.
	generatedFilePath := filepath.Join(basePath, "layouts_gen.go")
	f, err := os.OpenFile(generatedFilePath, os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0755)
	if err != nil {
		panic(err)
	}
	defer f.Close()

	_, err = f.WriteString(generatedFile)
	if err != nil {
		panic(err)
	}

	fmt.Println("Done generating layouts_gen.go")
}

func parseDirectoryFunctions(base, name string) (map[string][]ast.FuncDecl, error) {
	files, err := os.ReadDir(filepath.Join(base, name))
	if err != nil {
		return nil, errors.Join(errors.New("parseDirectoryFunctions.os.ReadDir failed to read directory"), err)
	}

	fns := make(map[string][]ast.FuncDecl)
	for _, file := range files {
		filePath := filepath.Join(base, name)
		if file.IsDir() {
			functions, err := parseDirectoryFunctions(filePath, file.Name())
			if err != nil {
				return nil, errors.Join(errors.New("parseDirectoryFunctions.parseDirectoryFunctions failed to parse directory"), err)
			}
			for k, v := range functions {
				fns[k] = append(fns[k], v...)
			}
			continue
		}
		if !strings.HasSuffix(file.Name(), "_templ.go") {
			continue
		}

		fset := token.NewFileSet() // positions are relative to fset
		parsed, err := parser.ParseFile(fset, filepath.Join(filePath, file.Name()), nil, 0)
		if err != nil {
			return nil, errors.Join(errors.New("parseDirectoryFunctions.parser.ParseFile failed to parse file"), err)
		}

		functions, err := parseFunctions(parsed)
		if err != nil {
			return nil, errors.Join(errors.New("parseDirectoryFunctions.parseFunctions failed to parse functions"), err)
		}

		fns[filePath] = append(fns[filePath], functions...)
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
