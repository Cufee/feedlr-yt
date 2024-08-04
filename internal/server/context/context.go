package context

import (
	"bytes"
	"context"
	"fmt"
	"net/http"
	"net/url"

	"github.com/cufee/feedlr-yt/internal/database"
	"github.com/cufee/tpot"
	"github.com/pkg/errors"
)

type Session struct {
	UserID string
}

var _ tpot.Context = &Ctx{}

type Ctx struct {
	c  context.Context
	db database.Client

	w http.ResponseWriter
	r *http.Request

	formParsed bool
}

func NewBuilder(db database.Client) tpot.ContextBuilder[*Ctx] {
	return func(w http.ResponseWriter, r *http.Request) *Ctx {
		return &Ctx{
			c:  r.Context(),
			db: db,
			w:  w,
			r:  r,
		}
	}
}

func (ctx *Ctx) Session() (Session, bool) {
	return Session{}, false
}

func (ctx *Ctx) Ctx() context.Context {
	return ctx.c
}

func (ctx *Ctx) Writer() http.ResponseWriter {
	return ctx.w
}
func (ctx *Ctx) Request() *http.Request {
	return ctx.r
}

func (ctx *Ctx) Cookie(key string) (*http.Cookie, error) {
	return ctx.r.Cookie(key)
}
func (ctx *Ctx) SetCookie(cookie *http.Cookie) {
	http.SetCookie(ctx.w, cookie)
}

func (ctx *Ctx) Query() (url.Values, error) {
	return ctx.r.URL.Query(), nil
}
func (ctx *Ctx) QueryValue(key string) string {
	return ctx.r.URL.Query().Get(key)
}

func (ctx *Ctx) FormValue(key string) (string, error) {
	if ctx.formParsed {
		return ctx.r.Form.Get(key), nil
	}
	if err := ctx.r.ParseForm(); err != nil {
		return "", err
	}
	ctx.formParsed = true
	return ctx.r.Form.Get(key), nil
}
func (ctx *Ctx) Form() (url.Values, error) {
	if ctx.formParsed {
		return ctx.r.Form, nil
	}
	if err := ctx.r.ParseForm(); err != nil {
		return nil, err
	}
	ctx.formParsed = true
	return ctx.r.Form, nil
}

func (ctx *Ctx) PathValue(key string) string {
	return ctx.r.PathValue(key)
}
func (ctx *Ctx) URL() *url.URL {
	return ctx.r.URL
}
func (ctx *Ctx) SetHeader(key, value string) {
	ctx.w.Header().Set(key, value)
}
func (ctx *Ctx) GetHeader(key string) string {
	return ctx.r.Header.Get(key)
}
func (ctx *Ctx) RealIP() (string, bool) {
	if ip := ctx.r.Header.Get("X-Forwarded-For"); ip != "" {
		return ip, true
	}
	if ip := ctx.r.RemoteAddr; ip != "" {
		return ip, true
	}
	return "", false
}

/*
Redirects a user to /error with an error message set as query param
*/
func (ctx *Ctx) Err(err error) {
	query := make(url.Values)
	if err != nil {
		query.Set("message", err.Error())
	}
	ctx.Redirect("/error?"+query.Encode(), http.StatusTemporaryRedirect)
}

/*
Creates a new error and calls ctx.Err()
*/
func (ctx *Ctx) Error(format string, args ...any) {
	ctx.Err(errors.Errorf(format, args...))
}

func (ctx *Ctx) String(format string, args ...any) {
	ctx.r.Write(bytes.NewBufferString(fmt.Sprintf(format, args...)))
}

func (ctx *Ctx) Redirect(path string, code int) {
	if ctx.r.Header.Get("HX-Request") == "true" {
		ctx.w.Header().Set("HX-Redirect", path)
		return
	}
	http.Redirect(ctx.w, ctx.r, path, code)
}

func (ctx *Ctx) SetStatus(code int) {
	ctx.w.WriteHeader(code)
}
