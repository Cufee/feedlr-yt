package auth

import (
	"encoding/json"
	"log"
	"net/http"

	"github.com/aarondl/null/v8"
	"github.com/cufee/feedlr-yt/internal/auth"
	"github.com/cufee/feedlr-yt/internal/database"
	"github.com/cufee/feedlr-yt/internal/metrics"
	"github.com/cufee/feedlr-yt/internal/server/handler"
	"github.com/cufee/tpot/brewed"
	"github.com/go-webauthn/webauthn/webauthn"
	"github.com/lucsky/cuid"
)

var RegistrationBegin brewed.Endpoint[*handler.Context] = func(ctx *handler.Context) error {
	outcome := "error"
	defer func() {
		metrics.IncUserAction("registration_begin", outcome)
		metrics.IncUserEvent("registration_begin", outcome)
	}()

	_, ok := ctx.UserID()
	if ok {
		outcome = "already_authenticated"
		return ctx.Redirect("/app", http.StatusTemporaryRedirect)
	}

	var form authForm
	err := ctx.BodyParser(&form)
	if err != nil {
		log.Print("ctx#BodyParser error", err)
		outcome = "invalid_request"
		return ctx.Status(http.StatusBadRequest).SendString("Invalid username")
	}

	username := ctx.Sanitize(form.Username)
	if username == "" || username != form.Username {
		outcome = "invalid_username"
		return ctx.Status(http.StatusBadRequest).SendString("Invalid username")
	}
	if len(username) < 5 {
		outcome = "invalid_username"
		return ctx.Status(http.StatusBadRequest).SendString("Username should be at least 5 characters long")
	}
	if len(username) > 18 {
		outcome = "invalid_username"
		return ctx.Status(http.StatusBadRequest).SendString("Username should be at most 18 characters long")
	}

	userStore := auth.NewStore(ctx.Database())
	_, err = userStore.FindUser(ctx.Context(), username)
	if !database.IsErrNotFound(err) {
		outcome = "username_taken"
		return ctx.Status(http.StatusBadRequest).SendString("Username already taken")
	}

	user, err := userStore.NewUser(ctx.Context(), cuid.New(), username)
	if err != nil {
		log.Print("userStore#NewUser error", err)
		outcome = "new_user_error"
		return ctx.Status(http.StatusInternalServerError).SendString("Failed to register")
	}

	session, err := ctx.SessionClient().New(ctx.Context())
	if err != nil {
		ctx.ClearCookie("session_id")
		log.Print("ctx#SessionClient#New error", err)
		outcome = "session_error"
		return ctx.Status(http.StatusInternalServerError).SendString("Failed to register")
	}

	waoptions, wasession, err := ctx.WebAuthn().BeginRegistration(user)
	if err != nil {
		ctx.ClearCookie("session_id")
		log.Print("ctx#WebAuth#BeginRegistration error", err)
		outcome = "webauthn_begin_error"
		return ctx.Status(http.StatusInternalServerError).SendString("Failed to register")
	}

	encodedSes, err := json.Marshal(wasession)
	if err != nil {
		ctx.ClearCookie("session_id")
		log.Print("json$Marshal error", err)
		outcome = "session_encode_error"
		return ctx.Status(http.StatusInternalServerError).SendString("Failed to register")
	}

	session, err = session.UpdateMeta(ctx.Context(), map[string]string{"user_id": user.ID, "type": "passkey", "data": string(encodedSes), "username": user.Username})
	if err != nil {
		ctx.ClearCookie("session_id")
		log.Print("session#UpdateMeta error", err)
		outcome = "session_update_error"
		return ctx.Status(http.StatusInternalServerError).SendString("Failed to register")
	}

	cookie, err := session.Cookie()
	if err != nil {
		ctx.ClearCookie("session_id")
		log.Print("session#Cookie error", err)
		outcome = "cookie_error"
		return ctx.Status(http.StatusInternalServerError).SendString("Failed to register")
	}
	ctx.Cookie(cookie)
	outcome = "success"
	return ctx.JSON(waoptions)
}

var RegistrationFinish brewed.Endpoint[*handler.Context] = func(ctx *handler.Context) error {
	outcome := "error"
	defer func() {
		metrics.IncUserAction("registration_finish", outcome)
		metrics.IncUserEvent("registration_finish", outcome)
	}()

	session, ok := ctx.Session()
	if !ok || session.Meta["type"] != "passkey" || session.Meta["user_id"] == "" || session.Meta["username"] == "" || session.Meta["data"] == "" {
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

	userStore := auth.NewStore(ctx.Database())
	user, err := userStore.NewUser(ctx.Context(), session.Meta["user_id"], session.Meta["username"])
	if err != nil {
		ctx.ClearCookie("session_id")
		log.Println("userStore#GetOrCreateUser failed", err.Error())
		outcome = "new_user_error"
		return ctx.Status(http.StatusInternalServerError).SendString("Invalid credentials")
	}

	credential, err := ctx.WebAuthn().FinishRegistration(user, wasession, ctx.Request())
	if err != nil {
		ctx.ClearCookie("session_id")
		log.Println("WebAuthn#FinishLogin failed", err.Error())
		outcome = "webauthn_finish_error"
		return ctx.Status(http.StatusInternalServerError).SendString("Invalid credentials")
	}

	// Handle credential.Authenticator.CloneWarning
	if credential.Authenticator.CloneWarning {
		log.Printf("the authenticator may be cloned\n")
	}

	err = user.AddCredential(*credential)
	if err != nil {
		ctx.ClearCookie("session_id")
		log.Println("user#UpdateCredential failed", err.Error())
		outcome = "credential_error"
		return ctx.Status(http.StatusInternalServerError).SendString("Invalid credentials")
	}

	err = userStore.CreateUser(ctx.Context(), &user)
	if err != nil {
		ctx.ClearCookie("session_id")
		log.Println("userStore#SaveUser failed", err.Error())
		outcome = "user_create_error"
		return ctx.Status(http.StatusInternalServerError).SendString("Invalid credentials")
	}

	session, err = session.UpdateUser(ctx.Context(), null.StringFrom(user.ID), null.String{})
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
