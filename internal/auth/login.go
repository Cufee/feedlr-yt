package auth

import (
	"context"
	"log"
	"strconv"

	"github.com/auth0/go-auth0/authentication/passwordless"
	"github.com/byvko-dev/youtube-app/internal/database"
	"github.com/byvko-dev/youtube-app/internal/sessions"
	"github.com/gofiber/fiber/v2"
)

// TODO: This should return some HTML or a proper redirect for HTMX
func LoginStartHandler(c *fiber.Ctx) error {
	existingSession := c.Cookies("session_id")
	if existingSession != "" {
		err := sessions.DeleteSession(existingSession)
		if err != nil {
			log.Printf("sessions.DeleteSession: %v\n", err)
		}
	}

	var body struct {
		Email string `form:"email"`
	}
	err := c.BodyParser(&body)
	if err != nil {
		log.Printf("c.BodyParser: %v\n", err)
		return c.SendStatus(fiber.StatusBadRequest)
	}

	if body.Email == "" {
		return c.SendStatus(fiber.StatusBadRequest)
	}

	_, err = client.Passwordless.SendEmail(context.Background(), passwordless.SendEmailRequest{Email: body.Email})
	if err != nil {
		log.Printf("Passwordless.SendEmail: %v\n", err)
		return c.SendStatus(fiber.StatusInternalServerError)
	}

	session, err := sessions.New()
	if err != nil {
		log.Printf("sessions.NewSession: %v\n", err)
		return c.SendStatus(fiber.StatusInternalServerError)
	}

	cookie, err := session.Cookie()
	if err != nil {
		log.Printf("session.Cookie: %v\n", err)
		return c.SendStatus(fiber.StatusInternalServerError)
	}

	c.Cookie(cookie)
	return c.SendString("Awesome! You can close this tab, and check your email for a link to log in.")
}

func LoginVerifyHandler(c *fiber.Ctx) error {
	sessionId := c.Cookies("session_id")
	if sessionId == "" {
		return c.SendStatus(fiber.StatusBadRequest)
	}

	var tokenExpiration int64
	var accessToken string = c.Query("access_token")

	expiration, err := strconv.Atoi(c.Query("expires_in"))
	if err != nil {
		log.Printf("strconv.Atoi: %v\n", err)
		return c.SendStatus(fiber.StatusBadRequest)
	}
	tokenExpiration = int64(expiration)

	if accessToken != "" && tokenExpiration > 0 {
		info, err := client.UserInfo(context.Background(), accessToken)
		if err != nil {
			sessions.DeleteSession(sessionId)
			log.Printf("UserInfo: %v\n", err)
			return c.SendStatus(fiber.StatusInternalServerError)
		}

		user, err := database.C.EnsureUserExists(info.Sub)
		if err != nil {
			sessions.DeleteSession(sessionId)
			log.Printf("EnsureUserExists: %v\n", err)
			return c.SendStatus(fiber.StatusInternalServerError)
		}

		session, err := sessions.New()
		if err != nil {
			session.Delete()
			c.ClearCookie("session_id")
			log.Printf("sessions.FromID: %v\n", err)
			return c.SendStatus(fiber.StatusInternalServerError)
		}

		err = session.Update(sessions.Options{UserID: user.ID, AuthID: info.Sub, AccessToken: accessToken})
		if err != nil {
			session.Delete()
			c.ClearCookie("session_id")
			log.Printf("session.Update: %v\n", err)
			return c.SendStatus(fiber.StatusInternalServerError)
		}

		cookie, err := session.Cookie()
		if err != nil {
			session.Delete()
			c.ClearCookie("session_id")
			log.Printf("session.Cookie: %v\n", err)
			return c.SendStatus(fiber.StatusInternalServerError)
		}

		c.Cookie(cookie)
		return c.Redirect("/app")
	}

	return c.SendStatus(fiber.StatusBadRequest)
}
