package app

import (
	"log"

	"github.com/cufee/feedlr-yt/internal/logic"
	"github.com/cufee/feedlr-yt/internal/templates/pages/app"
	"github.com/gofiber/fiber/v2"
)

func GetOrPostApp(c *fiber.Ctx) error {
	userId, _ := c.Locals("userId").(string)

	layout := "layouts/app"
	if c.Method() == "POST" || c.Get("HX-Request") != "" {
		layout = "layouts/blank"
	}

	props, err := logic.GetUserVideosProps(userId)
	if err != nil {
		log.Printf("GetUserVideosProps: %v", err)
		return c.Redirect("/error?message=Something went wrong")
	}
	if len(props.Videos) == 0 {
		return c.Redirect("/app/onboarding")
	}

	return c.Render(layout, app.VideosFeed(*props))
}
