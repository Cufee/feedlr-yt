package auth

import (
	"encoding/json"
	"log"
	"net/http"

	"github.com/aarondl/null/v8"
	iauth "github.com/cufee/feedlr-yt/internal/auth"
	"github.com/cufee/feedlr-yt/internal/metrics"
	"github.com/cufee/feedlr-yt/internal/server/handler"
	"github.com/cufee/tpot/brewed"
	"github.com/go-webauthn/webauthn/webauthn"
)

var DiscoverableLoginBegin brewed.Endpoint[*handler.Context] = func(ctx *handler.Context) error {
	outcome := "error"
	defer func() {
		metrics.IncUserAction("discoverable_login_begin", outcome)
		metrics.IncUserEvent("discoverable_login_begin", outcome)
	}()

	_, ok := ctx.UserID()
	if ok {
		outcome = "already_authenticated"
		return ctx.Redirect("/app", http.StatusTemporaryRedirect)
	}

	session, err := ctx.SessionClient().New(ctx.Context())
	if err != nil {
		ctx.ClearCookie("session_id")
		log.Print("ctx#SessionClient#New error", err)
		outcome = "session_error"
		return ctx.Status(http.StatusInternalServerError).SendString("Failed to log in")
	}

	waoptions, wasession, err := ctx.WebAuthn().BeginDiscoverableLogin()
	if err != nil {
		ctx.ClearCookie("session_id")
		log.Print("ctx#WebAuthn#BeginDiscoverableLogin error", err)
		outcome = "webauthn_begin_error"
		return ctx.Status(http.StatusInternalServerError).SendString("Failed to log in")
	}

	encodedSes, err := json.Marshal(wasession)
	if err != nil {
		ctx.ClearCookie("session_id")
		log.Print("json#Marshal error", err)
		outcome = "session_encode_error"
		return ctx.Status(http.StatusInternalServerError).SendString("Failed to log in")
	}

	session, err = session.UpdateMeta(ctx.Context(), map[string]string{"type": "passkey-discoverable", "data": string(encodedSes)})
	if err != nil {
		ctx.ClearCookie("session_id")
		log.Print("session#UpdateMeta error", err)
		outcome = "session_update_error"
		return ctx.Status(http.StatusInternalServerError).SendString("Failed to log in")
	}

	cookie, err := session.Cookie()
	if err != nil {
		ctx.ClearCookie("session_id")
		log.Print("session#Cookie error", err)
		outcome = "cookie_error"
		return ctx.Status(http.StatusInternalServerError).SendString("Failed to log in")
	}
	ctx.Cookie(cookie)
	outcome = "success"
	return ctx.JSON(waoptions)
}

var DiscoverableLoginFinish brewed.Endpoint[*handler.Context] = func(ctx *handler.Context) error {
	outcome := "error"
	defer func() {
		metrics.IncUserAction("discoverable_login_finish", outcome)
		metrics.IncUserEvent("discoverable_login_finish", outcome)
	}()

	session, ok := ctx.Session()
	if !ok || session.Meta["type"] != "passkey-discoverable" || session.Meta["data"] == "" {
		outcome = "missing_credentials"
		return ctx.Status(http.StatusBadRequest).SendString("Missing credentials")
	}

	var wasession webauthn.SessionData
	err := json.Unmarshal([]byte(session.Meta["data"]), &wasession)
	if err != nil {
		ctx.ClearCookie("session_id")
		log.Println("json#Unmarshal failed", err.Error())
		outcome = "invalid_credentials"
		return ctx.Status(http.StatusInternalServerError).SendString("Invalid credentials")
	}

	userStore := iauth.NewStore(ctx.Database())

	// Capture the resolved user from the handler callback
	var resolvedUser iauth.User
	userHandler := func(rawID, userHandle []byte) (webauthn.User, error) {
		user, err := userStore.GetUser(ctx.Context(), string(userHandle))
		if err != nil {
			return nil, err
		}
		resolvedUser = user
		return user, nil
	}

	credential, err := ctx.WebAuthn().FinishDiscoverableLogin(userHandler, wasession, ctx.Request())
	if err != nil {
		ctx.ClearCookie("session_id")
		log.Println("WebAuthn#FinishDiscoverableLogin failed", err.Error())
		outcome = "webauthn_finish_error"
		return ctx.Status(http.StatusInternalServerError).SendString("Invalid credentials")
	}

	if credential.Authenticator.CloneWarning {
		log.Printf("the authenticator may be cloned\n")
	}

	err = resolvedUser.UpdateCredential(*credential)
	if err != nil {
		ctx.ClearCookie("session_id")
		log.Println("user#UpdateCredential failed", err.Error())
		outcome = "credential_update_error"
		return ctx.Status(http.StatusInternalServerError).SendString("Invalid credentials")
	}

	err = userStore.SaveUser(ctx.Context(), &resolvedUser)
	if err != nil {
		ctx.ClearCookie("session_id")
		log.Println("userStore#SaveUser failed", err.Error())
		outcome = "user_save_error"
		return ctx.Status(http.StatusInternalServerError).SendString("Invalid credentials")
	}

	session, err = session.UpdateUser(ctx.Context(), null.StringFrom(resolvedUser.ID), null.String{})
	if err != nil {
		ctx.ClearCookie("session_id")
		log.Println("session#UpdateUser failed", err.Error())
		outcome = "session_update_error"
		return ctx.Status(http.StatusInternalServerError).SendString("Invalid credentials")
	}

	cookie, err := session.Cookie()
	if err != nil {
		ctx.ClearCookie("session_id")
		log.Println("session#Cookie failed", err.Error())
		outcome = "cookie_error"
		return ctx.Status(http.StatusInternalServerError).SendString("Invalid credentials")
	}
	ctx.Cookie(cookie)
	outcome = "success"
	return ctx.SendStatus(http.StatusOK)
}
