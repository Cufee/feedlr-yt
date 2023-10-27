package main

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

func main() {
	generateStyleFile()
}

func generateStyleFile() {
	fmt.Println("Started generating style_gen.go...")

	// Get the path to a directory where this file is located.
	_, filename, _, ok := runtime.Caller(1)
	if !ok {
		panic("No caller information")
	}
	basePath, err := filepath.Abs(filepath.Join(filepath.Dir(filename), "../"))
	if err != nil {
		panic(err)
	}

	rootPath := filepath.Join(basePath, "../", "../")
	assetsPath := filepath.Join(rootPath, "assets")

	files, _ := filepath.Glob(filepath.Join(assetsPath, "css", "style.*.css"))
	if len(files) == 0 {
		panic("missing style file")
	}

	stylePath := filepath.Join("/", (strings.Replace(files[0], rootPath, "", 1)))

	var generatedFile string = fmt.Sprintf("package assets\n\nvar StylePath = \"%s\"", stylePath)
	generatedFilePath := filepath.Join(basePath, "style_gen.go")
	f, err := os.OpenFile(generatedFilePath, os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0755)
	if err != nil {
		panic(err)
	}
	defer f.Close()

	_, err = f.WriteString(generatedFile)
	if err != nil {
		panic(err)
	}

	fmt.Println("Done generating style_gen.go")
}
