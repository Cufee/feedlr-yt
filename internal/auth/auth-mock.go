//go:build dev

package auth

import (
	"context"

	"github.com/aarondl/null/v8"
	"github.com/cufee/feedlr-yt/internal/database"
	"github.com/cufee/feedlr-yt/internal/sessions"
	"github.com/gofiber/fiber/v2"
)

const (
	mockUserID   = "dev-mock-user"
	mockUsername = "dev-user"
)

// MockMiddleware creates a development middleware that bypasses passkey auth
// and automatically authenticates as a hardcoded dev user.
func MockMiddleware(db database.Client) func(c *fiber.Ctx) error {
	sc, err := sessions.New(db)
	if err != nil {
		panic(err)
	}

	// Ensure mock user exists at startup
	ensureMockUser(db)

	return func(c *fiber.Ctx) error {
		// Try to get existing session
		session, err := sc.Get(c.Context(), c.Cookies("session_id"))

		// If no session or session invalid, create a new one
		if err != nil || !session.Valid() {
			session, err = sc.New(c.Context())
			if err != nil {
				return c.Redirect("/error?message=failed+to+create+session")
			}
		}

		// Check if session already has the mock user
		userID, hasUser := session.UserID()
		if !hasUser || userID != mockUserID {
			// Associate session with mock user
			session, err = session.UpdateUser(c.Context(), null.StringFrom(mockUserID), null.StringFrom("mock-connection"))
			if err != nil {
				return c.Redirect("/error?message=failed+to+update+session")
			}
		}

		// Refresh session and set cookie
		if err := session.Refresh(c); err != nil {
			return c.Redirect("/error?message=failed+to+refresh+session")
		}

		c.Locals("session", session)
		return c.Next()
	}
}

// ensureMockUser creates the mock user if it doesn't exist
func ensureMockUser(db database.Client) {
	ctx := context.Background()

	// Check if user exists
	_, err := db.GetUser(ctx, mockUserID)
	if err == nil {
		return // User exists
	}

	// Create the mock user
	_, err = db.CreateUser(ctx, mockUserID, mockUsername)
	if err != nil {
		panic("failed to create mock user: " + err.Error())
	}
}
