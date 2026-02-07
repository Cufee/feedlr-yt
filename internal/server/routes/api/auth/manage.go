package auth

import (
	"encoding/json"
	"log"
	"net/http"

	"github.com/a-h/templ"
	"github.com/cufee/feedlr-yt/internal/auth"
	"github.com/cufee/feedlr-yt/internal/metrics"
	"github.com/cufee/feedlr-yt/internal/server/handler"
	"github.com/cufee/feedlr-yt/internal/templates/components/shared"
	"github.com/cufee/tpot/brewed"
	"github.com/go-webauthn/webauthn/webauthn"
)

var DeletePasskey brewed.Partial[*handler.Context] = func(ctx *handler.Context) (templ.Component, error) {
	outcome := "error"
	defer func() {
		metrics.IncUserAction("passkey_delete", outcome)
		metrics.IncUserEvent("passkey_delete", outcome)
	}()

	session, ok := ctx.Session()
	if !ok {
		outcome = "unauthorized"
		return nil, ctx.Redirect("/login", http.StatusTemporaryRedirect)
	}
	userID, ok := session.UserID()
	if !ok {
		outcome = "unauthorized"
		return nil, ctx.Redirect("/login", http.StatusTemporaryRedirect)
	}

	keyID := ctx.Sanitize(ctx.Params("passkeyId"))
	if keyID == "" {
		outcome = "invalid_request"
		return nil, ctx.Error("missing passkey id")
	}

	err := ctx.Database().DeleteUserPasskey(ctx.Context(), userID, keyID)
	if err != nil {
		outcome = "delete_error"
		return nil, ctx.Err(err)
	}

	outcome = "success"
	return shared.DeletedElement("pk-" + keyID), nil
}

var AdditionalPasskeyBegin brewed.Endpoint[*handler.Context] = func(ctx *handler.Context) error {
	outcome := "error"
	defer func() {
		metrics.IncUserAction("passkey_begin", outcome)
		metrics.IncUserEvent("passkey_begin", outcome)
	}()

	session, ok := ctx.Session()
	if !ok {
		outcome = "unauthorized"
		return ctx.Redirect("/login", http.StatusTemporaryRedirect)
	}
	userID, ok := session.UserID()
	if !ok {
		outcome = "unauthorized"
		return ctx.Redirect("/login", http.StatusTemporaryRedirect)
	}

	userStore := auth.NewStore(ctx.Database())
	user, err := userStore.GetUser(ctx.Context(), userID)
	if err != nil {
		outcome = "user_load_error"
		return ctx.Redirect("/login", http.StatusTemporaryRedirect)
	}

	waoptions, wasession, err := ctx.WebAuthn().BeginRegistration(user)
	if err != nil {
		outcome = "webauthn_begin_error"
		return ctx.Status(http.StatusInternalServerError).SendString("Failed to register")
	}

	encodedSes, err := json.Marshal(wasession)
	if err != nil {
		outcome = "session_encode_error"
		return ctx.Status(http.StatusInternalServerError).SendString("Failed to register")
	}

	session, err = session.UpdateMeta(ctx.Context(), map[string]string{"type": "passkey", "data": string(encodedSes)})
	if err != nil {
		outcome = "session_update_error"
		return ctx.Status(http.StatusInternalServerError).SendString("Failed to register")
	}

	cookie, err := session.Cookie()
	if err != nil {
		outcome = "cookie_error"
		return ctx.Status(http.StatusInternalServerError).SendString("Failed to register")
	}
	ctx.Cookie(cookie)
	outcome = "success"
	return ctx.JSON(waoptions)
}

var AdditionalPasskeyFinish brewed.Endpoint[*handler.Context] = func(ctx *handler.Context) error {
	outcome := "error"
	defer func() {
		metrics.IncUserAction("passkey_finish", outcome)
		metrics.IncUserEvent("passkey_finish", outcome)
	}()

	session, ok := ctx.Session()
	if !ok {
		outcome = "unauthorized"
		return ctx.Redirect("/login", http.StatusTemporaryRedirect)
	}
	userID, ok := session.UserID()
	if !ok {
		outcome = "unauthorized"
		return ctx.Redirect("/login", http.StatusTemporaryRedirect)
	}
	if !ok || session.Meta["type"] != "passkey" || session.Meta["data"] == "" {
		outcome = "missing_credentials"
		return ctx.Status(http.StatusBadRequest).SendString("Missing credentials")
	}

	var wasession webauthn.SessionData
	err := json.Unmarshal([]byte(session.Meta["data"]), &wasession)
	if err != nil {
		log.Println("json#Unmarshal failed", err.Error())
		outcome = "invalid_credentials"
		return ctx.Status(http.StatusInternalServerError).SendString("Invalid credentials")
	}

	userStore := auth.NewStore(ctx.Database())
	user, err := userStore.GetUser(ctx.Context(), userID)
	if err != nil {
		log.Println("userStore#GetUser failed", err.Error())
		outcome = "user_load_error"
		return ctx.Status(http.StatusInternalServerError).SendString("Invalid credentials")
	}

	credential, err := ctx.WebAuthn().FinishRegistration(user, wasession, ctx.Request())
	if err != nil {
		log.Println("WebAuthn#AdditionalPasskeyFinish failed", err.Error())
		outcome = "webauthn_finish_error"
		return ctx.Status(http.StatusInternalServerError).SendString("Invalid credentials")
	}

	// Handle credential.Authenticator.CloneWarning
	if credential.Authenticator.CloneWarning {
		log.Printf("the authenticator may be cloned\n")
	}

	err = user.AddCredential(*credential)
	if err != nil {
		log.Println("user#UpdateCredential failed", err.Error())
		outcome = "credential_error"
		return ctx.Status(http.StatusInternalServerError).SendString("Invalid credentials")
	}

	err = userStore.SaveUser(ctx.Context(), &user)
	if err != nil {
		log.Println("userStore#SaveUser failed", err.Error())
		outcome = "user_save_error"
		return ctx.Status(http.StatusInternalServerError).SendString("Invalid credentials")
	}

	outcome = "success"
	return ctx.SendStatus(http.StatusOK)
}
