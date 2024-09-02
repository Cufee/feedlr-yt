package auth

import (
	"encoding/json"
	"log"
	"net/http"

	"github.com/cufee/feedlr-yt/internal/auth"
	"github.com/cufee/feedlr-yt/internal/database"
	"github.com/cufee/feedlr-yt/internal/server/handler"
	"github.com/cufee/tpot/brewed"
	"github.com/go-webauthn/webauthn/webauthn"
	"github.com/lucsky/cuid"
	"github.com/volatiletech/null/v8"
)

var RegistrationBegin brewed.Endpoint[*handler.Context] = func(ctx *handler.Context) error {
	_, ok := ctx.UserID()
	if ok {
		return ctx.Redirect("/app", http.StatusTemporaryRedirect)
	}

	var form authForm
	err := ctx.BodyParser(&form)
	if err != nil {
		log.Print("ctx#BodyParser error", err)
		return ctx.Status(http.StatusBadRequest).SendString("Invalid username")
	}

	username := ctx.Sanitize(form.Username)
	if username == "" || username != form.Username {
		return ctx.Status(http.StatusBadRequest).SendString("Invalid username")
	}
	if len(username) < 5 {
		return ctx.Status(http.StatusBadRequest).SendString("Username should be at least 5 characters long")
	}
	if len(username) > 18 {
		return ctx.Status(http.StatusBadRequest).SendString("Username should be at most 18 characters long")
	}

	userStore := auth.NewStore(ctx.Database())
	_, err = userStore.FindUser(ctx.Context(), username)
	if !database.IsErrNotFound(err) {
		return ctx.Status(http.StatusBadRequest).SendString("Username already taken")
	}

	user, err := userStore.NewUser(ctx.Context(), cuid.New(), username)
	if err != nil {
		log.Print("userStore#NewUser error", err)
		return ctx.Status(http.StatusInternalServerError).SendString("Failed to register")
	}

	session, err := ctx.SessionClient().New(ctx.Context())
	if err != nil {
		ctx.ClearCookie("session_id")
		log.Print("ctx#SessionClient#New error", err)
		return ctx.Status(http.StatusInternalServerError).SendString("Failed to register")
	}

	waoptions, wasession, err := ctx.WebAuthn().BeginRegistration(user)
	if err != nil {
		ctx.ClearCookie("session_id")
		log.Print("ctx#WebAuth#BeginRegistration error", err)
		return ctx.Status(http.StatusInternalServerError).SendString("Failed to register")
	}

	encodedSes, err := json.Marshal(wasession)
	if err != nil {
		ctx.ClearCookie("session_id")
		log.Print("json$Marshal error", err)
		return ctx.Status(http.StatusInternalServerError).SendString("Failed to register")
	}

	session, err = session.UpdateMeta(ctx.Context(), map[string]string{"user_id": user.ID, "type": "passkey", "data": string(encodedSes), "username": user.Username})
	if err != nil {
		ctx.ClearCookie("session_id")
		log.Print("session#UpdateMeta error", err)
		return ctx.Status(http.StatusInternalServerError).SendString("Failed to register")
	}

	cookie, err := session.Cookie()
	if err != nil {
		ctx.ClearCookie("session_id")
		log.Print("session#Cookie error", err)
		return ctx.Status(http.StatusInternalServerError).SendString("Failed to register")
	}
	ctx.Cookie(cookie)
	return ctx.JSON(waoptions)
}

var RegistrationFinish brewed.Endpoint[*handler.Context] = func(ctx *handler.Context) error {
	session, ok := ctx.Session()
	if !ok || session.Meta["type"] != "passkey" || session.Meta["user_id"] == "" || session.Meta["username"] == "" || session.Meta["data"] == "" {
		return ctx.Status(http.StatusBadRequest).SendString("Missing credentials")
	}

	var wasession webauthn.SessionData
	err := json.Unmarshal([]byte(session.Meta["data"]), &wasession)
	if err != nil {
		ctx.ClearCookie("session_id")
		log.Println("json#Unmarshal failed", err.Error())
		return ctx.Status(http.StatusInternalServerError).SendString("Invalid credentials")
	}

	userStore := auth.NewStore(ctx.Database())
	user, err := userStore.NewUser(ctx.Context(), session.Meta["user_id"], session.Meta["username"])
	if err != nil {
		ctx.ClearCookie("session_id")
		log.Println("userStore#GetOrCreateUser failed", err.Error())
		return ctx.Status(http.StatusInternalServerError).SendString("Invalid credentials")
	}

	credential, err := ctx.WebAuthn().FinishRegistration(user, wasession, ctx.Request())
	if err != nil {
		ctx.ClearCookie("session_id")
		log.Println("WebAuthn#FinishLogin failed", err.Error())
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
		return ctx.Status(http.StatusInternalServerError).SendString("Invalid credentials")
	}

	err = userStore.CreateUser(ctx.Context(), &user)
	if err != nil {
		ctx.ClearCookie("session_id")
		log.Println("userStore#SaveUser failed", err.Error())
		return ctx.Status(http.StatusInternalServerError).SendString("Invalid credentials")
	}

	session, err = session.UpdateUser(ctx.Context(), null.StringFrom(user.ID), null.String{})
	if err != nil {
		ctx.ClearCookie("session_id")
		log.Println("session#UpdateUser failed", err.Error())
		return ctx.Status(http.StatusInternalServerError).SendString("Invalid credentials")
	}

	cookie, err := session.Cookie()
	if err != nil {
		ctx.ClearCookie("session_id")
		log.Println("session#Cookie failed", err.Error())
		return ctx.Status(http.StatusInternalServerError).SendString("Invalid credentials")
	}
	ctx.Cookie(cookie)
	return ctx.SendStatus(http.StatusOK)
}
