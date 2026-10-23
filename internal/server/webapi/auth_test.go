package webapi

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/cufee/feedlr-yt/internal/database/models"
	"github.com/cufee/feedlr-yt/internal/logic"
	"github.com/cufee/feedlr-yt/internal/sessions"
	"github.com/go-webauthn/webauthn/webauthn"
	"github.com/gofiber/fiber/v3"
)

func sessionID(t *testing.T, res *http.Response) string {
	t.Helper()
	for _, cookie := range res.Cookies() {
		if cookie.Name == "session_id" {
			return cookie.Value
		}
	}
	t.Fatal("ceremony did not set a session cookie")
	return ""
}

func TestNativePasskeyBeginsAndErrorsAreJSON(t *testing.T) {
	f := newFixture(t)
	for _, test := range []struct {
		path, body, cookie string
		status             int
	}{
		{"/auth/register/begin", `{`, "", 400},
		{"/auth/register/begin", `{"username":"tiny"}`, "", 422},
		{"/auth/register/begin", `{"username":"viewer"}`, "", 409},
		{"/auth/login/begin", `{"username":"missing"}`, "", 401},
		{"/auth/login/begin", `{"username":"viewer"}`, f.session, 409},
		{"/auth/login/finish", `{}`, "", 400},
		{"/auth/register/finish", `{}`, "", 400},
		{"/passkeys/add/begin", "", "", 401},
		{"/passkeys/add/finish", `{}`, f.session, 400},
	} {
		res, v := f.request(t, "POST", test.path, test.body, test.cookie, "")
		if res.StatusCode != test.status {
			t.Fatalf("%s %d want%d %#v", test.path, res.StatusCode, test.status, v)
		}
		f.check(t, v, "Error")
	}
	res, v := f.request(t, "POST", "/auth/register/begin", `{"username":"newviewer"}`, "", "")
	if res.StatusCode != 200 {
		t.Fatalf("register begin %d %#v", res.StatusCode, v)
	}
	f.check(t, v, "WebAuthnCreationOptions")
	pending := sessionID(t, res)
	sc, err := sessions.New(f.db)
	if err != nil {
		t.Fatal(err)
	}
	s, err := sc.Get(context.Background(), pending)
	if err != nil {
		t.Fatal(err)
	}
	if s.Meta["type"] != "webauthn-register" || s.Meta["username"] != "newviewer" || s.Meta["data"] == "" {
		t.Fatal("registration challenge was not bound to its user and ceremony")
	}
	if _, authenticated := s.UserID(); authenticated {
		t.Fatal("begin authenticated an unverified account")
	}
	res, v = f.request(t, "POST", "/passkeys/add/begin", "", f.session, "")
	if res.StatusCode != 200 {
		t.Fatalf("add begin %d %#v", res.StatusCode, v)
	}
	f.check(t, v, "WebAuthnCreationOptions")
	res, v = f.request(t, "POST", "/logout", "", pending, "")
	if res.StatusCode != 204 {
		t.Fatalf("pending logout %d %#v", res.StatusCode, v)
	}
	if _, err = sc.GetReadOnly(context.Background(), pending); err == nil {
		t.Fatal("logout did not revoke a pending ceremony session")
	}
}

func TestPasskeysUnavailableReturnsJSONWithoutPanicking(t *testing.T) {
	f := newFixture(t)
	sc, err := sessions.New(f.db)
	if err != nil {
		t.Fatal(err)
	}
	f.app = fiber.New()
	Register(f.app, f.db, sc, nil)
	for _, path := range []string{"/auth/login/begin", "/auth/login/finish", "/auth/register/begin", "/auth/register/finish", "/passkeys/add/begin", "/passkeys/add/finish"} {
		res, v := f.request(t, "POST", path, `{}`, f.session, "")
		if res.StatusCode != 503 {
			t.Fatalf("%s unavailable %d %#v", path, res.StatusCode, v)
		}
		f.check(t, v, "Error")
	}
}

func TestLoginChallengeIsOneUseAndPurposeBound(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	encoded, err := json.Marshal(webauthn.Credential{ID: []byte{1, 2, 3}, PublicKey: []byte("fixture")})
	if err != nil {
		t.Fatal(err)
	}
	if err = f.db.SaveUserPasskey(ctx, &models.Passkey{UserID: "viewer", Data: encoded}); err != nil {
		t.Fatal(err)
	}
	res, v := f.request(t, "POST", "/auth/login/begin", `{"username":"viewer"}`, "", "")
	if res.StatusCode != 200 {
		t.Fatalf("login begin %d %#v", res.StatusCode, v)
	}
	f.check(t, v, "WebAuthnRequestOptions")
	loginSession := sessionID(t, res)
	clientData := base64.RawURLEncoding.EncodeToString([]byte(`{"type":"webauthn.get","challenge":"dGVzdA","origin":"http://example.com"}`))
	authenticatorData := base64.RawURLEncoding.EncodeToString(make([]byte, 37))
	credential := `{"id":"AQID","rawId":"AQID","type":"public-key","response":{"clientDataJSON":"` + clientData + `","authenticatorData":"` + authenticatorData + `","signature":"AQID"}}`
	res, v = f.request(t, "POST", "/auth/register/begin", `{"username":"newviewer"}`, "", "")
	if res.StatusCode != 200 {
		t.Fatalf("register begin %d %#v", res.StatusCode, v)
	}
	wrongPurpose := sessionID(t, res)
	res, v = f.request(t, "POST", "/auth/login/finish", credential, wrongPurpose, "")
	if res.StatusCode != 401 {
		t.Fatalf("wrong ceremony accepted %d %#v", res.StatusCode, v)
	}
	f.check(t, v, "Error")
	for range 2 {
		res, v = f.request(t, "POST", "/auth/login/finish", credential, loginSession, "")
		if res.StatusCode != 401 {
			t.Fatalf("invalid credential/challenge replay %d %#v", res.StatusCode, v)
		}
		f.check(t, v, "Error")
	}
	sc, err := sessions.New(f.db)
	if err != nil {
		t.Fatal(err)
	}
	s, err := sc.Get(ctx, loginSession)
	if err != nil {
		t.Fatal(err)
	}
	if s.Meta["type"] != "webauthn-claimed" || s.Meta["data"] != "" {
		t.Fatal("the failed verification retained a reusable challenge")
	}
	if _, ok := s.UserID(); ok {
		t.Fatal("a failed passkey response authenticated a user")
	}
}

func TestOAuthBeginAndCallbackBindingUseNativeJSON(t *testing.T) {
	f := newFixture(t)
	t.Setenv("YOUTUBE_OAUTH_CLIENT_ID", "test-client")
	t.Setenv("YOUTUBE_OAUTH_CLIENT_SECRET", "test-secret")
	t.Setenv("YOUTUBE_OAUTH_REDIRECT_URL", "http://example.com/auth/youtube/callback")
	t.Setenv("YOUTUBE_SYNC_ENCRYPTION_SECRET", "test-only-encryption-secret")
	service, err := logic.NewYouTubeSyncService(f.db)
	if err != nil {
		t.Fatal(err)
	}
	previous := logic.DefaultYouTubeSync
	logic.DefaultYouTubeSync = service
	t.Cleanup(func() { logic.DefaultYouTubeSync = previous })
	res, v := f.request(t, "POST", "/settings/youtube/connect", "", f.session, "")
	if res.StatusCode != 200 {
		t.Fatalf("OAuth begin %d %#v", res.StatusCode, v)
	}
	f.check(t, v, "RedirectURL")
	var state string
	for _, cookie := range res.Cookies() {
		if cookie.Name == "youtube_sync_state" {
			state = cookie.Value
			if !cookie.HttpOnly {
				t.Fatal("OAuth state cookie is not HTTP-only")
			}
		}
	}
	if state == "" || !strings.Contains(v.(map[string]any)["redirectUrl"].(string), "accounts.google.com") {
		t.Fatal("OAuth begin did not provide provider navigation")
	}
	for _, path := range []string{"/auth/youtube/callback", "/api/settings/youtube-sync/connect/callback"} {
		for _, test := range []struct {
			state, cookie string
			want          int
		}{{"wrong", f.session, 400}, {state + ":viewer", f.session, 400}, {state + ":viewer", f.otherSession, 400}, {state + ":viewer", "", 401}} {
			req := httptest.NewRequest("GET", "http://example.com"+path+"?state="+test.state, nil)
			req.AddCookie(&http.Cookie{Name: "youtube_sync_state", Value: state})
			if test.cookie != "" {
				req.AddCookie(&http.Cookie{Name: "session_id", Value: test.cookie})
			}
			response, err := f.app.Test(req)
			if err != nil {
				t.Fatal(err)
			}
			var value any
			err = json.NewDecoder(response.Body).Decode(&value)
			response.Body.Close()
			if err != nil {
				t.Fatal(err)
			}
			if response.StatusCode != test.want || response.Header.Get("Location") != "" {
				t.Fatalf("callback binding %s: %d %#v", path, response.StatusCode, value)
			}
			f.check(t, value, "Error")
		}
	}
}
