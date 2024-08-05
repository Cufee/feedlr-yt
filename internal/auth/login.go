package auth

import (
	"github.com/gofiber/fiber/v2"
)

func LoginStartHandler(c *fiber.Ctx) error {
	// sessionId := c.Cookies("session_id")
	// if sessionId != "" {
	// 	session, _ := sessions.FromID(sessionId)
	// 	if session.Valid() {
	// 		return c.Redirect("/app")
	// 	}
	// 	session.Delete()
	// 	c.ClearCookie("session_id")
	// }

	// id, err := ksuid.NewRandomWithTime(time.Now())
	// if err != nil {
	// 	log.Printf("ksuid.NewRandomWithTime: %v\n", err)
	// 	return c.Redirect("/error?message=Something went wrong while logging in&context=creating session ID")
	// }

	// nonce, err := database.DefaultClient.NewAuthNonce(time.Now().Add(time.Minute*5), id.String())
	// if err != nil {
	// 	log.Printf("database.DefaultClient.NewAuthNonce: %v\n", err)
	// 	return c.Redirect("/error?message=Something went wrong while logging in&context=creating auth nonce")
	// }

	// meta := make(map[string]any)
	// meta["nonce"] = nonce.Value
	// meta["expires_at"] = nonce.ExpiresAt

	// url := defaultAuthenticator.AuthCodeURL(nonce.Value)
	// return c.Redirect(url, fiber.StatusTemporaryRedirect)
	return c.Redirect("/", fiber.StatusTemporaryRedirect)
}

func LoginCallbackHandler(c *fiber.Ctx) error {
	// existingSession := c.Cookies("session_id")
	// if existingSession != "" {
	// 	err := sessions.DeleteSession(existingSession)
	// 	if err != nil {
	// 		log.Printf("sessions.DeleteSession: %v\n", err)
	// 	}
	// }

	// token, err := defaultAuthenticator.Exchange(c.Context(), c.Query("code"))
	// if err != nil {
	// 	log.Printf("defaultAuthenticator.Exchange: %v\n", err)
	// 	return c.Redirect("/error?message=Something went wrong while logging in&context=invalid auth code")
	// }

	// idToken, err := defaultAuthenticator.VerifyIDToken(c.Context(), token)
	// if err != nil {
	// 	log.Printf("defaultAuthenticator.VerifyIDToken: %v\n", err)
	// 	return c.Redirect("/error?message=Something went wrong while logging in&context=invalid id token")
	// }

	// if idToken.Subject != "" && idToken.Expiry.After(time.Now()) && token.Expiry.After(time.Now()) {
	// 	user, err := database.DefaultClient.EnsureUserExists(idToken.Subject)
	// 	if err != nil {
	// 		log.Printf("EnsureUserExists: %v\n", err)
	// 		return c.Redirect("/error?message=Something went wrong while logging in&context=invalid auth token")
	// 	}

	// 	session, err := sessions.New()
	// 	if err != nil {
	// 		session.Delete()
	// 		c.ClearCookie("session_id")
	// 		log.Printf("sessions.Delete: %v\n", err)
	// 		return c.Redirect("/error?message=Something went wrong while logging in&context=creating session")
	// 	}

	// 	err = session.AddUserID(user.ID.Hex(), idToken.Subject)
	// 	if err != nil {
	// 		session.Delete()
	// 		c.ClearCookie("session_id")
	// 		log.Printf("session.Update: %v\n", err)
	// 		return c.Redirect("/error?message=Something went wrong while logging in&context=updating session")
	// 	}

	// 	cookie, err := session.Cookie()
	// 	if err != nil {
	// 		session.Delete()
	// 		c.ClearCookie("session_id")
	// 		log.Printf("session.Cookie: %v\n", err)
	// 		return c.Redirect("/error?message=Something went wrong while logging in&context=missing session cookie")
	// 	}

	// 	c.Cookie(cookie)
	// 	return c.Redirect("/app")
	// }

	return c.Redirect("/error?message=Something went wrong while logging in&context=missing auth token")
}
