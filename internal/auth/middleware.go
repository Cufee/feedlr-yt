package auth

import (
	"log"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/session"
	_ "github.com/joho/godotenv/autoload"
)

func NewMiddleware(store *session.Store) func(c *fiber.Ctx) error {
	return func(c *fiber.Ctx) error {
		session, err := store.Get(c)
		if err != nil {
			log.Printf("session.Get: %v\n", err)
			return c.Redirect("/login")
		}

		userID, ok := session.Get("userId").(string)
		if !ok {
			log.Println("session.Get userId: !ok")
			return c.Redirect("/login")
		}

		c.Locals("userId", userID)
		return c.Next()
	}
}
