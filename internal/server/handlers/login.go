package root

// func LoginHandler(c *fiber.Ctx) error {
// 	sessionId := c.Cookies("session_id")
// 	if sessionId != "" {
// 		session, _ := sessions.FromID(sessionId)
// 		if session.Valid() {
// 			return c.Redirect("/app")
// 		}
// 		session.Delete()
// 		c.ClearCookie("session_id")
// 	}
// 	return c.Render("login", nil, withLayout(c))
// }

// func LoginRedirectHandler(c *fiber.Ctx) error {
// 	return c.Render("login/redirect", nil, withLayout(c))
// }
