package webapi

import (
	"context"
	"encoding/json"
	"sync"
	"testing"

	"github.com/cufee/feedlr-yt/internal/database"
	"github.com/cufee/feedlr-yt/internal/database/models"
	"github.com/cufee/feedlr-yt/internal/sessions"
	"github.com/go-webauthn/webauthn/webauthn"
	"github.com/gofiber/fiber/v3"
)

type authCommitStorage interface {
	database.Client
	ConsumeSessionChallenge(context.Context, string, map[string]string) (bool, error)
	AuthenticateSession(context.Context, database.AuthenticationProof) (*models.Session, error)
	CreateUserWithPasskeySession(context.Context, string, string, []byte, string, string) (*models.Session, error)
	AddUserPasskeyForClaim(context.Context, string, []byte, string, string) error
	GetSessionReadOnly(context.Context, string) (*models.Session, error)
	DeleteUserPasskeyKeepingOne(context.Context, string, string) error
}
type pausedAuthCommit struct {
	authCommitStorage
	beforeLogin, beforeRegister, beforeAdd func()
	login, register, add                   sync.Once
}

func (d *pausedAuthCommit) AuthenticateSession(ctx context.Context, p database.AuthenticationProof) (*models.Session, error) {
	d.login.Do(func() {
		if d.beforeLogin != nil {
			d.beforeLogin()
		}
	})
	return d.authCommitStorage.AuthenticateSession(ctx, p)
}
func (d *pausedAuthCommit) CreateUserWithPasskeySession(ctx context.Context, id, name string, data []byte, source, claim string) (*models.Session, error) {
	d.register.Do(func() {
		if d.beforeRegister != nil {
			d.beforeRegister()
		}
	})
	return d.authCommitStorage.CreateUserWithPasskeySession(ctx, id, name, data, source, claim)
}
func (d *pausedAuthCommit) AddUserPasskeyForClaim(ctx context.Context, user string, data []byte, source, claim string) error {
	d.add.Do(func() {
		if d.beforeAdd != nil {
			d.beforeAdd()
		}
	})
	return d.authCommitStorage.AddUserPasskeyForClaim(ctx, user, data, source, claim)
}
func pauseAuthentication(t *testing.T, f *fixture) *pausedAuthCommit {
	t.Helper()
	db := &pausedAuthCommit{authCommitStorage: f.db.(authCommitStorage)}
	sc, err := sessions.New(db)
	if err != nil {
		t.Fatal(err)
	}
	wa, err := webauthn.New(&webauthn.Config{RPDisplayName: "Feedlr", RPID: "example.com", RPOrigins: []string{"http://example.com"}})
	if err != nil {
		t.Fatal(err)
	}
	f.app = fiber.New()
	Register(f.app, db, sc, wa)
	return db
}

func TestAuthenticationCommitRejectsNewerConsumedCeremonyAndDeletedCredential(t *testing.T) {
	for _, scenario := range []string{"newer consumed challenge", "deleted credential", "logout"} {
		t.Run(scenario, func(t *testing.T) {
			f := newFixture(t)
			key := makeAssertionKey(t, "modern", false, 0)
			saveAssertionKey(t, f, key, true)
			saveAssertionKey(t, f, makeAssertionKey(t, "spare", false, 0), true)
			challenge, cookie, _ := beginAssertion(t, f, `{}`, "")
			db := pauseAuthentication(t, f)
			db.beforeLogin = func() {
				switch scenario {
				case "newer consumed challenge":
					newer, next, _ := beginAssertion(t, f, `{}`, cookie)
					if next != cookie {
						t.Fatal("pending namespace changed")
					}
					invalid := key.assertion(t, assertionInput{id: key.id, handle: []byte("viewer"), challenge: newer, origin: "http://example.com", rp: "example.com", flags: 5, count: 2, badSignature: true})
					response, value := f.request(t, "POST", "/auth/login/finish", invalid, cookie, "")
					if response.StatusCode != 401 {
						t.Fatalf("newer invalid assertion: %d %#v", response.StatusCode, value)
					}
				case "deleted credential":
					keys, err := f.db.GetUserPasskeys(context.Background(), "viewer")
					if err != nil {
						t.Fatal(err)
					}
					for _, stored := range keys {
						var credential webauthn.Credential
						if err := json.Unmarshal(stored.Data, &credential); err != nil {
							t.Fatal(err)
						}
						if string(credential.ID) == "modern" {
							if err := db.DeleteUserPasskeyKeepingOne(context.Background(), "viewer", stored.ID); err != nil {
								t.Fatal(err)
							}
						}
					}
				case "logout":
					response, value := f.request(t, "POST", "/logout", "", cookie, "")
					if response.StatusCode != 204 {
						t.Fatalf("logout: %d %#v", response.StatusCode, value)
					}
				}
			}
			body := key.assertion(t, assertionInput{id: key.id, handle: []byte("viewer"), challenge: challenge, origin: "http://example.com", rp: "example.com", flags: 5, count: 1})
			response, value := f.request(t, "POST", "/auth/login/finish", body, cookie, "")
			if response.StatusCode != 401 {
				t.Fatalf("older verification authorized a session: %d %#v", response.StatusCode, value)
			}
		})
	}
}

func TestSupersededEnrollmentDoesNotCreateAccountOrAdditionalKey(t *testing.T) {
	for _, additional := range []bool{false, true} {
		t.Run(map[bool]string{false: "signup", true: "additional key"}[additional], func(t *testing.T) {
			f := newFixture(t)
			key := makeAssertionKey(t, "enrollment", false, 0)
			path, body, cookie := "/auth/register", `{"username":"enrollment"}`, ""
			if additional {
				path, body, cookie = "/passkeys/add", "", f.session
				saveAssertionKey(t, f, makeAssertionKey(t, "existing", false, 0), true)
			}
			response, value := f.request(t, "POST", path+"/begin", body, cookie, "")
			if response.StatusCode != 200 {
				t.Fatalf("begin: %d %#v", response.StatusCode, value)
			}
			if !additional {
				cookie = sessionID(t, response)
			}
			challenge := value.(map[string]any)["publicKey"].(map[string]any)["challenge"].(string)
			db := pauseAuthentication(t, f)
			newerConsumed := func() {
				response, value := f.request(t, "POST", path+"/begin", body, cookie, "")
				if response.StatusCode != 200 {
					t.Fatalf("newer begin: %d %#v", response.StatusCode, value)
				}
				newer := value.(map[string]any)["publicKey"].(map[string]any)["challenge"].(string)
				response, value = f.request(t, "POST", path+"/finish", key.registration(t, newer, "http://example.com", "example.com", 65, true), cookie, "")
				if response.StatusCode != 401 {
					t.Fatalf("newer unverified creation: %d %#v", response.StatusCode, value)
				}
			}
			if additional {
				db.beforeAdd = newerConsumed
			} else {
				db.beforeRegister = newerConsumed
			}
			response, value = f.request(t, "POST", path+"/finish", key.registration(t, challenge, "http://example.com", "example.com", 69, true), cookie, "")
			if response.StatusCode != 401 {
				t.Fatalf("superseded enrollment succeeded: %d %#v", response.StatusCode, value)
			}
			if additional {
				keys, err := f.db.GetUserPasskeys(context.Background(), "viewer")
				if err != nil || len(keys) != 1 {
					t.Fatalf("stale finish added credential: %v %v", keys, err)
				}
			} else if _, err := f.db.FindUser(context.Background(), "enrollment"); !database.IsErrNotFound(err) {
				t.Fatalf("stale finish reserved account: %v", err)
			}
		})
	}
}
