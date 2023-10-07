package auth

import (
	"context"
	"log"
	"strconv"
	"time"

	"github.com/auth0/go-auth0/authentication/oauth"
	"github.com/auth0/go-auth0/authentication/passwordless"
	"github.com/byvko-dev/youtube-app/internal/database"
	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/session"
)

func NewLoginStartHandler(store *session.Store) func(*fiber.Ctx) error {
	return func(c *fiber.Ctx) error {
		session, err := store.Get(c)
		if err != nil {
			log.Printf("session.Get: %v\n", err)
			return c.SendStatus(fiber.StatusInternalServerError)
		}

		var body struct {
			Email string `form:"email"`
		}
		err = c.BodyParser(&body)
		if err != nil {
			log.Printf("c.BodyParser: %v\n", err)
			return c.SendStatus(fiber.StatusBadRequest)
		}

		if body.Email == "" {
			return c.SendStatus(fiber.StatusBadRequest)
		}

		res, err := client.Passwordless.SendEmail(context.Background(), passwordless.SendEmailRequest{Email: body.Email})
		if err != nil {
			log.Printf("Passwordless.SendEmail: %v\n", err)
			return c.SendStatus(fiber.StatusInternalServerError)
		}

		session.Set("auth0RequestId", res.ID)
		session.SetExpiry(time.Minute * 5)
		session.Save()

		return c.SendString("Awesome! You can close this tab, and check your email for a link to log in.")
	}
}

func NewLoginVerifyHandler(store *session.Store) func(c *fiber.Ctx) error {
	return func(c *fiber.Ctx) error {
		session, err := store.Get(c)
		if err != nil {
			log.Printf("session.Get: %v\n", err)
			return c.SendStatus(fiber.StatusInternalServerError)
		}

		var accessToken string = c.Query("access_token")
		var tokenExpiration int64

		code := c.Query("code")
		if code != "" {
			res, err := client.Passwordless.LoginWithEmail(context.Background(), passwordless.LoginWithEmailRequest{Code: code, GrantType: "http://auth0.com/oauth/grant-type/passwordless/otp", Scope: "openid profile"}, oauth.IDTokenValidationOptions{})
			if err != nil {
				session.Destroy()
				log.Printf("Passwordless.LoginWithEmail: %v\n", err)
				return c.SendStatus(fiber.StatusInternalServerError)
			}

			tokenExpiration = res.ExpiresIn
			accessToken = res.AccessToken
		} else {
			expiration, err := strconv.Atoi(c.Query("expires_in"))
			if err != nil {
				log.Printf("strconv.Atoi: %v\n", err)
				return c.SendStatus(fiber.StatusBadRequest)
			}
			tokenExpiration = int64(expiration)
		}

		if accessToken != "" && tokenExpiration > 0 {
			info, err := client.UserInfo(context.Background(), accessToken)
			if err != nil {
				session.Destroy()
				log.Printf("UserInfo: %v\n", err)
				return c.SendStatus(fiber.StatusInternalServerError)
			}

			user, err := database.C.EnsureUserExists(info.Sub)
			if err != nil {
				session.Destroy()
				log.Printf("EnsureUserExists: %v\n", err)
				return c.SendStatus(fiber.StatusInternalServerError)
			}
			session.SetExpiry(time.Duration(tokenExpiration) * time.Second)
			session.Set("accessToken", accessToken)
			session.Set("authId", info.Sub)
			session.Set("userId", user.ID)
			session.Save()
			return c.Redirect("/app")
		}

		return c.SendStatus(fiber.StatusBadRequest)
	}
}
