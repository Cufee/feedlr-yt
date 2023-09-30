package middleware

import "github.com/gofiber/fiber/v2"

func AuthMiddleware(c *fiber.Ctx) error {
	// TODO: implement auth middleware
	c.Locals("userId", "cln6e1m390000itjq0gtnz0ht")
	return c.Next()
}
