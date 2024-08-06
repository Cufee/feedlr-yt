package auth

import (
	"context"
	"net/http"

	"github.com/cufee/feedlr-yt/internal/database"
	"github.com/cufee/feedlr-yt/internal/server/handler"
	"github.com/cufee/tpot/brewed"
	"github.com/volatiletech/null/v8"
	"google.golang.org/api/idtoken"
)

var GoogleAuthRedirect brewed.Endpoint[*handler.Context] = func(c *handler.Context) error {
	cookieToken := c.Cookies("g_csrf_token")
	bodyToken, err := c.FormValue("g_csrf_token")
	if err != nil {
		return c.Redirect("/error?message=Failed to log in with Google&context=missing credential", http.StatusTemporaryRedirect)
	}
	if bodyToken == "" || cookieToken == "" {
		return c.Redirect("/error?message=Failed to log in with Google&context=missing credential", http.StatusTemporaryRedirect)
	}
	if bodyToken != cookieToken {
		return c.Redirect("/error?message=Failed to log in with Google&context=missing credential", http.StatusTemporaryRedirect)
	}

	credential, err := c.FormValue("credential")
	if credential == "" || err != nil {
		return c.Redirect("/error?message=Failed to log in with Google&context=missing credential", http.StatusTemporaryRedirect)
	}

	payload, err := idtoken.Validate(context.Background(), credential, GoogleAuthClientID)
	if err != nil {
		return c.Redirect("/error?message=Failed to log in with Google&context=missing credential", http.StatusTemporaryRedirect)
	}

	googleUser, err := GoogleTokenInfo(credential)
	if err != nil {
		return c.Redirect("/error?message=Failed to log in with Google&context=missing credential", http.StatusTemporaryRedirect)
	}
	if payload.Audience != googleUser.Aud || payload.Issuer != googleUser.Issuer || payload.Subject != googleUser.Subject {
		return c.Redirect("/error?message=Failed to log in with Google&context=bad user info received", http.StatusTemporaryRedirect)
	}

	if googleUser.EmailVerified != "true" {
		return c.Redirect("/error?message=You need to verify your Google Account before using it to log in", http.StatusTemporaryRedirect)
	}
	if googleUser.Name == "" || googleUser.Email == "" {
		return c.Redirect("/error?message=Your Google Account is incomplete&context=missing name or email", http.StatusTemporaryRedirect)
	}

	connection, err := c.Database().GetConnection(c.Context(), googleUser.Subject)
	if err != nil && !database.IsErrNotFound(err) {
		return c.Redirect("/error?message=Failed to log in with Google&context=failed to get a connection", http.StatusTemporaryRedirect)
	}
	if database.IsErrNotFound(err) {
		user, err := c.Database().CreateUser(c.Context())
		if err != nil {
			return c.Redirect("/error?message=Failed to log in with Google&context=failed to create a user account", http.StatusTemporaryRedirect)
		}
		connection, err = c.Database().CreateConnection(c.Context(), user.ID, googleUser.Subject, database.ConnectionTypeGoogle)
		if err != nil {
			return c.Redirect("/error?message=Failed to log in with Google&context=failed to create a connection", http.StatusTemporaryRedirect)
		}
	}

	session, err := c.SessionClient().New(c.Context())
	if err != nil {
		return c.Redirect("/error?message=Failed to log in with Google&context=failed to create a session", http.StatusTemporaryRedirect)
	}

	session, err = session.UpdateUser(c.Context(), null.StringFrom(connection.UserID), null.StringFrom(connection.ID))
	if err != nil {
		return c.Redirect("/error?message=Failed to log in with Google&context=failed to update session", http.StatusTemporaryRedirect)
	}

	c.SetSession(session)

	sessionCookie, err := session.Cookie()
	if err != nil {
		return c.Redirect("/error?message=Failed to log in with Google&context=failed to create a cookie", http.StatusTemporaryRedirect)
	}
	c.Cookie(sessionCookie)

	return c.Redirect("/app", http.StatusTemporaryRedirect)
}
