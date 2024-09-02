package auth

import (
	"encoding/json"
	"log"
	"net/http"
	"strings"

	"github.com/cufee/feedlr-yt/internal/auth"
	"github.com/cufee/feedlr-yt/internal/server/handler"
	"github.com/cufee/tpot/brewed"
	"github.com/go-webauthn/webauthn/webauthn"
	"github.com/volatiletech/null/v8"
)

type authForm struct {
	Username string `json:"username"`
}

var LoginBegin brewed.Endpoint[*handler.Context] = func(ctx *handler.Context) error {
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

	username := strings.TrimSpace(ctx.Sanitize(form.Username))
	if username == "" || len(username) < 5 || len(username) > 18 || username != form.Username {
		return ctx.Status(http.StatusBadRequest).SendString("Invalid username")
	}

	userStore := auth.NewStore(ctx.Database())
	user, err := userStore.FindUser(ctx.Context(), username)
	if err != nil {
		log.Print("userStore#FindUser error", err)
		return ctx.Status(http.StatusInternalServerError).SendString("Account not found")
	}

	session, err := ctx.SessionClient().New(ctx.Context())
	if err != nil {
		ctx.ClearCookie("session_id")
		log.Print("ctx#SessionClient#New error", err)
		return ctx.Status(http.StatusInternalServerError).SendString("Failed to log in")
	}

	waoptions, wasession, err := ctx.WebAuthn().BeginLogin(user)
	if err != nil {
		ctx.ClearCookie("session_id")
		log.Print("ctx#WebAuthn#BeginLogin error", err)
		return ctx.Status(http.StatusInternalServerError).SendString("Failed to log in")
	}

	encodedSes, err := json.Marshal(wasession)
	if err != nil {
		ctx.ClearCookie("session_id")
		log.Print("json#Marshal error", err)
		return ctx.Status(http.StatusInternalServerError).SendString("Failed to log in")
	}

	session, err = session.UpdateMeta(ctx.Context(), map[string]string{"user_id": user.ID, "type": "passkey", "data": string(encodedSes)})
	if err != nil {
		ctx.ClearCookie("session_id")
		log.Print("session#UpdateMeta error", err)
		return ctx.Status(http.StatusInternalServerError).SendString("Failed to log in")
	}

	cookie, err := session.Cookie()
	if err != nil {
		ctx.ClearCookie("session_id")
		log.Print("session#Cookie error", err)
		return ctx.Status(http.StatusInternalServerError).SendString("Failed to log in")
	}
	ctx.Cookie(cookie)
	return ctx.JSON(waoptions)
}

var LoginFinish brewed.Endpoint[*handler.Context] = func(ctx *handler.Context) error {
	session, ok := ctx.Session()
	if !ok || session.Meta["type"] != "passkey" || session.Meta["user_id"] == "" || session.Meta["data"] == "" {
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
	user, err := userStore.GetUser(ctx.Context(), session.Meta["user_id"])
	if err != nil {
		ctx.ClearCookie("session_id")
		log.Println("userStore#GetOrCreateUser failed", err.Error())
		return ctx.Status(http.StatusInternalServerError).SendString("Invalid credentials")
	}

	credential, err := ctx.WebAuthn().FinishLogin(user, wasession, ctx.Request())
	if err != nil {
		ctx.ClearCookie("session_id")
		log.Println("WebAuthn#FinishLogin failed", err.Error())
		return ctx.Status(http.StatusInternalServerError).SendString("Invalid credentials")
	}

	// Handle credential.Authenticator.CloneWarning
	if credential.Authenticator.CloneWarning {
		log.Printf("the authenticator may be cloned\n")
	}

	err = user.UpdateCredential(*credential)
	if err != nil {
		ctx.ClearCookie("session_id")
		log.Println("user#UpdateCredential failed", err.Error())
		return ctx.Status(http.StatusInternalServerError).SendString("Invalid credentials")
	}

	err = userStore.SaveUser(ctx.Context(), &user)
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
