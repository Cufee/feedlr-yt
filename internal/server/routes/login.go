package root

import (
	"encoding/base64"
	"net/http"
	"time"

	"github.com/a-h/templ"
	"github.com/cufee/feedlr-yt/internal/logic"
	"github.com/cufee/feedlr-yt/internal/server/handler"
	"github.com/cufee/feedlr-yt/internal/templates/layouts"
	"github.com/cufee/feedlr-yt/internal/templates/pages"
	"github.com/cufee/tpot/brewed"
)

var Login brewed.Page[*handler.Context] = func(ctx *handler.Context) (brewed.Layout[*handler.Context], templ.Component, error) {
	session, _ := ctx.Session()
	_, ok := session.UserID()
	if ok {
		return nil, nil, ctx.Redirect("/app", http.StatusTemporaryRedirect)
	}

	session, err := ctx.SessionClient().New(ctx.Context())
	if err != nil {
		return nil, nil, ctx.Err(err)
	}

	ip, _ := ctx.RealIP()
	nonceValue := base64.StdEncoding.EncodeToString([]byte(logic.HashString(ip + time.Now().Format(time.RFC3339Nano))))
	nonce, err := ctx.Database().NewAuthNonce(ctx.Context(), time.Now().Add(time.Minute*5), nonceValue)
	if err != nil {
		return nil, nil, ctx.Err(err)
	}

	session.Meta["auth_nonce"] = nonce.Value
	session, err = session.UpdateMeta(ctx.Context(), session.Meta)
	if err != nil {
		return nil, nil, ctx.Err(err)
	}

	cookie, err := session.Cookie()
	if err != nil {
		return nil, nil, ctx.Err(err)
	}
	ctx.Cookie(cookie)

	return layouts.Main, pages.Login(nonce.Value), nil
}
