package utils

import (
	"os"
	"path/filepath"
	"strings"
)

var CurrentStylePath = FindStylePath()

func FindStylePath() string {
	cwd, _ := os.Getwd()
	files, _ := filepath.Glob(filepath.Join(cwd, "assets", "css", "style.*.css"))
	if len(files) == 0 {
		panic("missing style file")
	}
	if cwd == "/" {
		return files[0]
	}
	return strings.Replace(files[0], cwd, "", 1)
}
