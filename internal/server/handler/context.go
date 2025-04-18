package handler

import (
	"bytes"
	"context"
	"fmt"
	"net/http"
	"net/url"

	"github.com/cufee/feedlr-yt/internal/database"
	"github.com/cufee/feedlr-yt/internal/sessions"
	"github.com/cufee/tpot"
	"github.com/go-webauthn/webauthn/webauthn"
	"github.com/gofiber/fiber/v2"
	"github.com/microcosm-cc/bluemonday"
	"github.com/pkg/errors"
)

type contextKey byte

const (
	ContextKeyCustomCtx contextKey = iota
)

var _ tpot.Context = &Context{}

type Context struct {
	*fiber.Ctx

	c   context.Context
	db  database.Client
	ses *sessions.SessionClient

	w http.ResponseWriter
	r *http.Request

	formParsed bool

	wa     *webauthn.WebAuthn
	policy *bluemonday.Policy
}

func (ctx *Context) Writer() http.ResponseWriter {
	return ctx.w
}
func (ctx *Context) Request() *http.Request {
	return ctx.r
}
func (ctx *Context) Context() context.Context {
	return ctx.Ctx.Context()
}

func (c *Context) Status(status int) *Context {
	c.Response().SetStatusCode(status)
	c.w.WriteHeader(status)
	return c
}

func NewBuilder(db database.Client, ses *sessions.SessionClient, policy *bluemonday.Policy, wa *webauthn.WebAuthn) func(*fiber.Ctx) tpot.ContextBuilder[*Context] {
	return func(c *fiber.Ctx) tpot.ContextBuilder[*Context] {
		return func(w http.ResponseWriter, r *http.Request) *Context {
			return &Context{
				Ctx:    c,
				c:      r.Context(),
				db:     db,
				ses:    ses,
				w:      w,
				r:      r,
				policy: policy,
				wa:     wa,
			}
		}
	}
}

func (ctx *Context) SetSession(s sessions.Session) bool {
	_, ok := ctx.Locals("session", s).(sessions.Session)
	return ok
}

func (ctx *Context) Session() (sessions.Session, bool) {
	session, ok := ctx.Locals("session").(sessions.Session)
	if !ok {
		var err error
		session, err = ctx.ses.Get(ctx.Context(), ctx.Cookies("session_id"))
		if err != nil {
			return sessions.Session{}, false
		}
	}
	if !session.Valid() {
		return sessions.Session{}, false
	}
	return session, true
}

func (ctx *Context) WebAuthn() *webauthn.WebAuthn {
	return ctx.wa
}

func (ctx *Context) SessionClient() *sessions.SessionClient {
	return ctx.ses
}

func (ctx *Context) UserID() (string, bool) {
	session, ok := ctx.Session()
	if !ok {
		return "", false
	}
	id, ok := session.UserID()
	if !ok || id == "" {
		return "", false
	}
	return id, true
}

func (ctx *Context) Authenticated() bool {
	session, ok := ctx.Session()
	if !ok {
		return false
	}
	id, ok := session.UserID()
	if !ok || id == "" {
		return false
	}
	return true
}

func (ctx *Context) Database() database.Client {
	return ctx.db
}

func (ctx *Context) Sanitize(input string) string {
	return ctx.policy.Sanitize(input)
}

func (ctx *Context) FormValue(key string) (string, error) {
	if ctx.formParsed {
		return ctx.r.Form.Get(key), nil
	}
	if err := ctx.r.ParseForm(); err != nil {
		return "", err
	}
	ctx.formParsed = true
	return ctx.Sanitize(ctx.r.Form.Get(key)), nil
}

func (ctx *Context) Query(key string, fallback ...string) string {
	return ctx.Sanitize(ctx.Ctx.Query(key, fallback...))
}

func (ctx *Context) RealIP() (string, bool) {
	if ip := ctx.r.Header.Get("X-Forwarded-For"); ip != "" {
		return ctx.Sanitize(ip), true
	}
	if ip := ctx.r.RemoteAddr; ip != "" {
		return ctx.Sanitize(ip), true
	}
	return "", false
}

/*
Redirects a user to /error with an error message set as query param
*/
func (ctx *Context) Err(err error) error {
	query := make(url.Values)
	if err != nil {
		query.Set("message", err.Error())
	}
	return ctx.Redirect("/error?"+query.Encode(), http.StatusTemporaryRedirect)
}

/*
Creates a new error and calls ctx.Err()
*/
func (ctx *Context) Error(format string, args ...any) error {
	return ctx.Err(errors.Errorf(format, args...))

}

func (ctx *Context) String(format string, args ...any) error {
	return ctx.r.Write(bytes.NewBufferString(fmt.Sprintf(format, args...)))
}

func (ctx *Context) Redirect(path string, code int) error {
	if ctx.r.Header.Get("HX-Request") == "true" {
		ctx.w.Header().Set("HX-Redirect", path)
		return nil
	}
	http.Redirect(ctx.w, ctx.r, path, code)
	return nil
}
