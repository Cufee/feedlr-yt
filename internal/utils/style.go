package utils

import (
	"os"
	"path/filepath"
	"strings"
)

func FindStylePath() string {
	cwd, _ := os.Getwd()
	files, _ := filepath.Glob(filepath.Join(cwd, "assets", "css", "style.*.css"))
	if len(files) == 0 {
		panic("missing style file")
	}
	return strings.ReplaceAll(files[0], cwd, "")
}
