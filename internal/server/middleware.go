package server

// var limiterMiddleware = limiter.New(limiter.Config{
// 	Max:        20,
// 	Expiration: 30 * time.Second,
// 	KeyGenerator: func(c *fiber.Ctx) string {
// 		trace := c.Cookies("trace_id")
// 		if trace == "" {
// 			trace = uuid.NewString()
// 			cookie := fiber.Cookie{
// 				Name:  "trace_id",
// 				Value: trace,
// 			}
// 			c.Cookie(&cookie)
// 		}
// 		return c.Get("X-Forwarded-For", trace)
// 	},
// 	LimitReached: func(c *fiber.Ctx) error {
// 		return c.Render("429", nil, "layouts/with-head")
// 	},
// 	Storage: newRedisStore(),
// })
