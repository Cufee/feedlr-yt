package handlers

import (
	"fmt"
	"os"
	"path"
	"strings"

	"github.com/gofiber/fiber/v2"
)

var layouts []string

func init() {
	files, err := os.ReadDir(path.Join(rootDir, "layouts"))
	if err != nil {
		panic(err)
	}

	for _, file := range files {
		if !file.IsDir() && strings.HasSuffix(file.Name(), ".html") && file.Name() != "main.html" {
			layouts = append(layouts, file.Name())
		}
	}
}

func addRouteLayout(c *fiber.Ctx) {
	layout := c.App().Config().ViewsLayout
	for _, l := range layouts {
		name := strings.TrimSuffix(l, ".html")
		if strings.HasPrefix(c.Path(), fmt.Sprintf("/%s", name)) {
			layout = "layouts/" + name
			break
		}
	}
	c.Locals("layout", layout)

}
