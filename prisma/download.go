package main

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/steebchen/prisma-client-go/binaries"
)

func main() {
	cwd, err := os.Getwd()
	if err != nil {
		panic(err)
	}
	err = binaries.FetchEngine(filepath.Join(cwd, "prisma", "bin"), "query-engine", "linux-static-x64")
	if err != nil {
		panic(err)
	}

	fmt.Print(binaries.GetEnginePath(filepath.Join(cwd, "prisma", "bin"), "query-engine", "linux-static-x64"))
}
