package main

import (
	"flag"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/steebchen/prisma-client-go/binaries"
)

func main() {
	cwd, _ := os.Getwd()

	engineName := flag.String("engine", "query-engine", "engine name")
	binaryName := flag.String("binary", "debian-openssl-1.1.x", "binary name")
	path := flag.String("path", filepath.Join(cwd, "prisma", "bin"), "path to store binary")

	err := binaries.FetchEngine(*path, *engineName, *binaryName)
	if err != nil {
		panic(err)
	}

	enginePath := binaries.GetEnginePath(*path, *engineName, *binaryName)
	err = exec.Command("mv", enginePath, filepath.Join(*path, "engine")).Run()
	if err != nil {
		panic(err)
	}
}
