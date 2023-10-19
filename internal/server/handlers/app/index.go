package app

// func AppHandler(c *fiber.Ctx) error {
// 	userId, _ := c.Locals("userId").(string)

// 	subscriptions, err := logic.GetUserSubscriptionsProps(userId)
// 	if err != nil {
// 		log.Printf("GetUserSubscriptionsProps: %v", err)
// 		return c.Redirect("/error?message=Something went wrong")
// 	}
// 	if len(subscriptions.All) == 0 {
// 		return c.Redirect("/app/onboarding")
// 	}

// 	props, err := subscriptions.ToMap()
// 	if err != nil {
// 		log.Printf("UserSubscriptionsFeedProps.ToMap: %v", err)
// 		return c.Redirect("/error?message=Something went wrong")
// 	}

// 	return c.Render("app/index", withNavbarProps(c, props), withLayout(c))
// }

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
