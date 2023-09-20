package handlers

import (
	"github.com/gofiber/fiber/v2"
)

var handlers map[string]func(c *fiber.Ctx) error = map[string](func(c *fiber.Ctx) error){}

func newHandler(path string, fn func(c *fiber.Ctx) error) {
	handlers[path] = fn
}
