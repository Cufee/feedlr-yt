package auth

import (
	"crypto/sha256"
	"fmt"
	"net/http"
	"slices"

	"github.com/cufee/feedlr-yt/internal/logic"
	"github.com/cufee/feedlr-yt/internal/server/handler"
	"github.com/cufee/feedlr-yt/internal/templates/components/settings"
	"github.com/cufee/tpot/brewed"
)

func usernameToID(username string) string {
	hash := sha256.New()
	hash.Write([]byte(username))
	sum := hash.Sum(nil)
	return fmt.Sprintf("%x", sum)
}

type authForm struct {
	Username string `json:"username"`
}

var LoginBegin brewed.Endpoint[*handler.Context] = func(ctx *handler.Context) error {
	var form authForm
	err := ctx.BodyParser(&form)
	if err != nil {
		ctx.Status(http.StatusBadRequest)
		return ctx.SendString("Invalid username")
	}

	username := ctx.Sanitize(form.Username)
	if username == "" || username != form.Username {
		ctx.Status(http.StatusBadRequest)
		return ctx.SendString("Invalid username")
	}

	userID := usernameToID(username)
	user, err := ctx.Database().GetUser(ctx.Context(), userID)
	if err != nil {
		ctx.Status(http.StatusInternalServerError)
		return ctx.SendString("Invalid username")
	}

	session, err := ctx.SessionClient().New(ctx.Context())
	if err != nil {
		ctx.Status(http.StatusInternalServerError)
		return ctx.SendString("Failed to log in")
	}

	session, err = session.UpdateMeta(ctx.Context(), map[string]string{"pending_user_id": user.ID, "type": "passkey"})
	if err != nil {
		ctx.Status(http.StatusInternalServerError)
		return ctx.SendString("Failed to log in")
	}

	options, session, err := ctx.WebAuthn().BeginLogin()
	if err != nil {
		msg := fmt.Sprintf("can't begin login: %s", err.Error())
		p.l.Errorf(msg)
		JSONResponse(w, msg, http.StatusBadRequest)
		p.deleteSessionCookie(w)

		return
	}

	cookie, err := session.Cookie()
	if err != nil {
		ctx.Status(http.StatusInternalServerError)
		return ctx.SendString("Failed to log in")
	}
	ctx.Cookie(cookie)

	return nil
}

var LoginFinish brewed.Endpoint[*handler.Context] = func(ctx *handler.Context) error {
	userID, ok := ctx.UserID()
	if !ok {
		return nil, ctx.SendStatus(http.StatusUnauthorized)
	}

	category := ctx.Query("category")

	updated, err := logic.ToggleSponsorBlockCategory(ctx.Context(), ctx.Database(), userID, category)
	if err != nil {
		return nil, err
	}

	enabled := slices.Contains(updated.SponsorBlock.SelectedSponsorBlockCategories, category)
	return settings.CategoryToggleButton(category, enabled, false), nil
}
