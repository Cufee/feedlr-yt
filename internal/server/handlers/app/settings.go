package app

// func AppSettingsHandler(c *fiber.Ctx) error {
// 	userId, _ := c.Locals("userId").(string)
// 	_ = userId

// 	return c.Render("app/settings", withNavbarProps(c), withLayout(c))
// }

// func ManageChannelsAddHandler(c *fiber.Ctx) error {
// 	userId, _ := c.Locals("userId").(string)

// 	subscriptions, err := logic.GetUserSubscriptionsProps(userId)
// 	if err != nil {
// 		log.Printf("GetUserSubscriptionsProps: %v", err)
// 		return c.Redirect("/error?message=Something went wrong")
// 	}

// 	return c.Render("app/channels/manage", withNavbarProps(c, fiber.Map{
// 		"Subscriptions": subscriptions,
// 	}), withLayout(c))
// }

// func OnboardingHandler(c *fiber.Ctx) error {
// 	return c.Render("app/onboarding", withNavbarProps(c), withLayout(c))
// }
