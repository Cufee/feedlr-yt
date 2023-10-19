package templates

import (
	"context"

	"github.com/a-h/templ"
	"github.com/gofiber/fiber/v2"
)

//go:generate templ generate

func Render(c *fiber.Ctx, component templ.Component) error {
	templBuffer := templ.GetBuffer()
	defer templ.ReleaseBuffer(templBuffer)
	ctx := templ.InitializeContext(context.Background())
	err := component.Render(ctx, templBuffer)
	if err != nil {
		return err
	}
	c.Set("Content-Type", "text/html; charset=utf-8")
	return c.Send(templBuffer.Bytes())
}
