package auth

import (
	"encoding/json"
	"log"
	"net/http"

	"github.com/cufee/feedlr-yt/internal/auth"
	"github.com/cufee/feedlr-yt/internal/server/handler"
	"github.com/cufee/tpot/brewed"
	"github.com/go-webauthn/webauthn/webauthn"
)

var AdditionalPasskeyBegin brewed.Endpoint[*handler.Context] = func(ctx *handler.Context) error {
	session, ok := ctx.Session()
	if !ok {
		return ctx.Redirect("/login", http.StatusTemporaryRedirect)
	}
	userID, ok := session.UserID()
	if !ok {
		return ctx.Redirect("/login", http.StatusTemporaryRedirect)
	}

	userStore := auth.NewStore(ctx.Database())
	user, err := userStore.GetUser(ctx.Context(), userID)
	if err != nil {
		return ctx.Redirect("/login", http.StatusTemporaryRedirect)
	}

	waoptions, wasession, err := ctx.WebAuthn().BeginRegistration(user)
	if err != nil {
		return ctx.Status(http.StatusInternalServerError).SendString("Failed to register")
	}

	encodedSes, err := json.Marshal(wasession)
	if err != nil {
		return ctx.Status(http.StatusInternalServerError).SendString("Failed to register")
	}

	session, err = session.UpdateMeta(ctx.Context(), map[string]string{"type": "passkey", "data": string(encodedSes)})
	if err != nil {
		return ctx.Status(http.StatusInternalServerError).SendString("Failed to register")
	}

	cookie, err := session.Cookie()
	if err != nil {
		return ctx.Status(http.StatusInternalServerError).SendString("Failed to register")
	}
	ctx.Cookie(cookie)
	return ctx.JSON(waoptions)
}

var AdditionalPasskeyFinish brewed.Endpoint[*handler.Context] = func(ctx *handler.Context) error {
	session, ok := ctx.Session()
	if !ok {
		return ctx.Redirect("/login", http.StatusTemporaryRedirect)
	}
	userID, ok := session.UserID()
	if !ok {
		return ctx.Redirect("/login", http.StatusTemporaryRedirect)
	}
	if !ok || session.Meta["type"] != "passkey" || session.Meta["data"] == "" {
		return ctx.Status(http.StatusBadRequest).SendString("Missing credentials")
	}

	var wasession webauthn.SessionData
	err := json.Unmarshal([]byte(session.Meta["data"]), &wasession)
	if err != nil {
		log.Println("json#Unmarshal failed", err.Error())
		return ctx.Status(http.StatusInternalServerError).SendString("Invalid credentials")
	}

	userStore := auth.NewStore(ctx.Database())
	user, err := userStore.GetUser(ctx.Context(), userID)
	if err != nil {
		log.Println("userStore#GetUser failed", err.Error())
		return ctx.Status(http.StatusInternalServerError).SendString("Invalid credentials")
	}

	credential, err := ctx.WebAuthn().FinishRegistration(user, wasession, ctx.Request())
	if err != nil {
		log.Println("WebAuthn#AdditionalPasskeyFinish failed", err.Error())
		return ctx.Status(http.StatusInternalServerError).SendString("Invalid credentials")
	}

	// Handle credential.Authenticator.CloneWarning
	if credential.Authenticator.CloneWarning {
		log.Printf("the authenticator may be cloned\n")
	}

	err = user.AddCredential(*credential)
	if err != nil {
		log.Println("user#UpdateCredential failed", err.Error())
		return ctx.Status(http.StatusInternalServerError).SendString("Invalid credentials")
	}

	err = userStore.SaveUser(ctx.Context(), &user)
	if err != nil {
		log.Println("userStore#SaveUser failed", err.Error())
		return ctx.Status(http.StatusInternalServerError).SendString("Invalid credentials")
	}

	return ctx.SendStatus(http.StatusOK)
}
