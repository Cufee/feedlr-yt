package webapi

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"slices"
	"strings"
	"time"
	"unicode"

	"github.com/cufee/feedlr-yt/internal/auth"
	"github.com/cufee/feedlr-yt/internal/database"
	"github.com/cufee/feedlr-yt/internal/database/models"
	"github.com/cufee/feedlr-yt/internal/sessions"
	"github.com/go-webauthn/webauthn/protocol"
	"github.com/go-webauthn/webauthn/webauthn"
	"github.com/gofiber/fiber/v3"
	"github.com/lucsky/cuid"
	"github.com/microcosm-cc/bluemonday"
)

var usernamePolicy = bluemonday.StrictPolicy()

// An omitted username is the primary discoverable/passkey flow. Supplying a
// username explicitly selects compatibility with older non-discoverable keys.
type LoginBeginRequest struct {
	Username string `json:"username,omitempty"`
}

type passkeyPolicy struct {
	Version                  int  `json:"version"`
	UserVerificationRequired bool `json:"userVerificationRequired"`
	DiscoverableRequired     bool `json:"discoverableRequired"`
}

func requiredPasskeyVerification(data []byte) bool {
	var stored struct {
		Policy *passkeyPolicy `json:"feedlrPolicy"`
	}
	return json.Unmarshal(data, &stored) == nil && stored.Policy != nil && stored.Policy.UserVerificationRequired
}

func encodePasskey(credential *webauthn.Credential, existing []byte, enrolling bool) ([]byte, error) {
	encoded, err := json.Marshal(credential)
	if err != nil {
		return nil, err
	}
	var fields map[string]json.RawMessage
	if err = json.Unmarshal(encoded, &fields); err != nil {
		return nil, err
	}
	if enrolling {
		fields["feedlrPolicy"], err = json.Marshal(passkeyPolicy{Version: 1, UserVerificationRequired: true, DiscoverableRequired: true})
	} else {
		var previous map[string]json.RawMessage
		if err = json.Unmarshal(existing, &previous); err != nil {
			return nil, err
		}
		if policy, ok := previous["feedlrPolicy"]; ok {
			fields["feedlrPolicy"] = policy
		}
	}
	if err != nil {
		return nil, err
	}
	return json.Marshal(fields)
}

func registrationOptions(c fiber.Ctx, user auth.User) []webauthn.RegistrationOption {
	opts := []webauthn.RegistrationOption{
		webauthn.WithAuthenticatorSelection(protocol.AuthenticatorSelection{UserVerification: protocol.VerificationRequired}),
		webauthn.WithResidentKeyRequirement(protocol.ResidentKeyRequirementRequired),
		webauthn.WithConveyancePreference(protocol.PreferNoAttestation),
		webauthn.WithExclusions(webauthn.Credentials(user.WebAuthnCredentials()).CredentialDescriptors()),
		webauthn.WithExtensions(webauthn.WithExtensionCredProps()),
	}
	if origin := c.Get("Origin"); origin != "" {
		opts = append(opts, webauthn.WithRegistrationOrigin(origin))
	}
	return opts
}

func (a *API) authConfigured() error {
	if a.wa == nil || a.sessions == nil {
		return fail(503, "passkeys_unavailable", "Passkey authentication is not configured")
	}
	return nil
}
func usernameBody(c fiber.Ctx) (string, error) {
	var req struct {
		Username string `json:"username"`
	}
	if err := body(c, &req); err != nil {
		return "", err
	}
	return req.Username, validateUsername(req.Username)
}
func validateUsername(name string) error {
	if len(name) < 5 || len(name) > 18 || strings.TrimSpace(name) != name || usernamePolicy.Sanitize(name) != name {
		return invalid("username", "Use a username between 5 and 18 characters")
	}
	for _, r := range name {
		if unicode.IsControl(r) {
			return invalid("username", "Use a username without control characters")
		}
	}
	return nil
}
func sessionCookie(c fiber.Ctx, s sessions.Session) error {
	cookie, err := s.Cookie()
	if err != nil {
		return err
	}
	c.Cookie(cookie)
	return nil
}
func (a *API) startChallenge(c fiber.Ctx, kind string, user auth.User, data *webauthn.SessionData, existing *sessions.Session) error {
	// Browser timeouts are advisory. Every server-side challenge expires too.
	deadline := time.Now().Add(5 * time.Minute)
	if data.Expires.IsZero() || data.Expires.After(deadline) {
		data.Expires = deadline
	}
	encoded, err := json.Marshal(data)
	if err != nil {
		return err
	}
	var s sessions.Session
	if existing != nil {
		s = *existing
	} else if pending, ok := c.Locals("webapi_session").(sessions.Session); ok && pending.Valid() {
		s = pending
	} else {
		s, err = a.sessions.New(c.Context())
		if err != nil {
			return err
		}
	}
	_, err = s.UpdateMeta(c.Context(), map[string]string{"type": kind, "user_id": user.ID, "username": user.Username, "data": string(encoded)})
	if err != nil {
		return err
	}
	return sessionCookie(c, s)
}

func (a *API) loginBegin(c fiber.Ctx) error {
	if err := a.authConfigured(); err != nil {
		return err
	}
	if viewer(c) != "" {
		return fail(409, "already_authenticated", "You are already signed in")
	}
	var request LoginBeginRequest
	if err := body(c, &request); err != nil {
		return err
	}
	opts := []webauthn.LoginOption{webauthn.WithUserVerification(protocol.VerificationRequired)}
	if origin := c.Get("Origin"); origin != "" {
		opts = append(opts, webauthn.WithLoginOrigin(origin))
	}
	if request.Username == "" {
		options, data, err := a.wa.BeginDiscoverableLogin(opts...)
		if err != nil {
			return fail(503, "passkeys_unavailable", "A passkey challenge could not be created")
		}
		if err = a.startChallenge(c, "webauthn-discoverable", auth.User{User: &models.User{}}, data, nil); err != nil {
			return err
		}
		return c.JSON(options)
	}
	if err := validateUsername(request.Username); err != nil {
		return err
	}
	user, err := auth.NewStore(a.db).FindUser(c.Context(), request.Username)
	if err != nil {
		if database.IsErrNotFound(err) {
			return fail(401, "invalid_credentials", "This account or passkey could not be verified")
		}
		return err
	}
	// Explicit named fallback supports old non-discoverable/U2F keys. New keys
	// keep their durable UV requirement even when the account also has old keys.
	verification := protocol.VerificationRequired
	for _, credential := range user.WebAuthnCredentials() {
		record, _ := user.PasskeyRecord(credential.ID)
		if record != nil && !requiredPasskeyVerification(record.Data) {
			verification = protocol.VerificationPreferred
			break
		}
	}
	opts = append(opts, webauthn.WithUserVerification(verification))
	options, data, err := a.wa.BeginLogin(user, opts...)
	if err != nil {
		return fail(401, "invalid_credentials", "This account or passkey could not be verified")
	}
	if err = a.startChallenge(c, "webauthn-login", user, data, nil); err != nil {
		return err
	}
	return c.JSON(options)
}

func (a *API) registerBegin(c fiber.Ctx) error {
	if err := a.authConfigured(); err != nil {
		return err
	}
	if viewer(c) != "" {
		return fail(409, "already_authenticated", "You are already signed in")
	}
	name, err := usernameBody(c)
	if err != nil {
		return err
	}
	store := auth.NewStore(a.db)
	if _, err = store.FindUser(c.Context(), name); err == nil {
		return fail(409, "username_taken", "This username is already in use")
	} else if !database.IsErrNotFound(err) {
		return err
	}
	user, err := store.NewUser(c.Context(), cuid.New(), name)
	if err != nil {
		return err
	}
	options, data, err := a.wa.BeginRegistration(user, registrationOptions(c, user)...)
	if err != nil {
		return fail(503, "passkeys_unavailable", "A passkey challenge could not be created")
	}
	if err = a.startChallenge(c, "webauthn-register", user, data, nil); err != nil {
		return err
	}
	return c.JSON(options)
}

func (a *API) passkeyBegin(c fiber.Ctx) error {
	if err := a.authConfigured(); err != nil {
		return err
	}
	s, err := a.sessions.Get(c.Context(), c.Cookies("session_id"))
	if err != nil {
		return fail(401, "unauthenticated", "Please sign in to continue")
	}
	user, err := auth.NewStore(a.db).GetUser(c.Context(), viewer(c))
	if err != nil {
		return err
	}
	options, data, err := a.wa.BeginRegistration(user, registrationOptions(c, user)...)
	if err != nil {
		return fail(503, "passkeys_unavailable", "A passkey challenge could not be created")
	}
	if err = a.startChallenge(c, "webauthn-add", user, data, &s); err != nil {
		return err
	}
	return c.JSON(options)
}

func (a *API) consumeChallenge(c fiber.Ctx, kinds ...string) (sessions.Session, webauthn.SessionData, error) {
	var data webauthn.SessionData
	s, err := a.sessions.Get(c.Context(), c.Cookies("session_id"))
	if err != nil || !s.Valid() {
		return s, data, fail(401, "challenge_expired", "Start the passkey request again")
	}
	kind := s.Meta["type"]
	if !slices.Contains(kinds, kind) || (kind != "webauthn-discoverable" && s.Meta["user_id"] == "") || s.Meta["data"] == "" {
		return s, data, fail(401, "challenge_expired", "Start the passkey request again")
	}
	if kind == "webauthn-add" && s.Meta["user_id"] != viewer(c) {
		return s, data, fail(401, "challenge_expired", "Start the passkey request again")
	}
	if err = json.Unmarshal([]byte(s.Meta["data"]), &data); err != nil {
		return s, data, fail(401, "challenge_expired", "Start the passkey request again")
	}
	claimer, ok := a.db.(interface {
		ConsumeSessionChallenge(context.Context, string, map[string]string) (bool, error)
	})
	if !ok {
		return s, data, fail(503, "passkeys_unavailable", "Passkey storage is unavailable")
	}
	claimed, err := claimer.ConsumeSessionChallenge(c.Context(), s.ID(), s.Meta)
	if err != nil {
		return s, data, err
	}
	if !claimed {
		return s, data, fail(401, "challenge_expired", "Start the passkey request again")
	}
	return s, data, nil
}

func (a *API) loginFinish(c fiber.Ctx) error {
	if err := a.authConfigured(); err != nil {
		return err
	}
	if viewer(c) != "" {
		return fail(409, "already_authenticated", "You are already signed in")
	}
	var raw json.RawMessage
	if err := body(c, &raw); err != nil {
		return err
	}
	parsed, err := protocol.ParseCredentialRequestResponseBody(bytes.NewReader(raw))
	if err != nil {
		return fail(400, "invalid_credential", "The passkey response is invalid")
	}
	s, data, err := a.consumeChallenge(c, "webauthn-login", "webauthn-discoverable")
	if err != nil {
		return err
	}
	store := auth.NewStore(a.db)
	for range 3 {
		var user auth.User
		var credential *webauthn.Credential
		if s.Meta["type"] == "webauthn-discoverable" {
			resolved, verified, verifyErr := a.wa.ValidatePasskeyLogin(func(rawID, userHandle []byte) (webauthn.User, error) {
				if len(userHandle) == 0 || len(userHandle) > 64 {
					return nil, errors.New("invalid user handle")
				}
				candidate, err := store.GetUser(c.Context(), string(userHandle))
				if err != nil {
					return nil, err
				}
				if _, owned := candidate.PasskeyRecord(rawID); !owned {
					return nil, errors.New("credential does not belong to account")
				}
				return candidate, nil
			}, data, parsed)
			err = verifyErr
			if err == nil {
				user, credential = resolved.(auth.User), verified
			}
		} else {
			user, err = store.GetUser(c.Context(), s.Meta["user_id"])
			if err == nil {
				credential, err = a.wa.ValidateLogin(user, data, parsed)
			}
		}
		if err != nil {
			return fail(401, "invalid_credentials", "This account or passkey could not be verified")
		}
		record, owned := user.PasskeyRecord(credential.ID)
		if !owned || (requiredPasskeyVerification(record.Data) && !parsed.Response.AuthenticatorData.Flags.HasUserVerified()) {
			return fail(401, "invalid_credentials", "This account or passkey could not be verified")
		}
		// Zero counters are valid for synced credentials. The SDK preserves the
		// maximum counter and records warnings; reject regression on device-bound
		// keys while retaining that risk signal for backup-eligible passkeys.
		if credential.Authenticator.CloneWarning && !credential.Flags.BackupEligible {
			return fail(401, "invalid_credentials", "This account or passkey could not be verified")
		}
		encoded, err := encodePasskey(credential, record.Data, false)
		if err != nil {
			return err
		}
		writer, ok := a.db.(interface {
			AuthenticateSession(context.Context, database.AuthenticationProof) (*models.Session, error)
		})
		if !ok {
			return fail(503, "passkeys_unavailable", "Passkey storage is unavailable")
		}
		authenticated, err := writer.AuthenticateSession(c.Context(), database.AuthenticationProof{SourceID: s.ID(), UserID: user.ID, Claim: database.SessionChallengeClaim(s.Meta), PasskeyID: record.ID, ExpectedCredential: record.Data, UpdatedCredential: encoded})
		if errors.Is(err, database.ErrCredentialChanged) {
			continue
		}
		if database.IsErrNotFound(err) {
			return fail(401, "challenge_expired", "Start the passkey request again")
		}
		if err != nil {
			return err
		}
		if err := a.authenticatedCookie(c, authenticated.ID); err != nil {
			return err
		}
		return c.SendStatus(204)
	}
	return fail(409, "credential_changed", "This passkey changed during verification. Please try again.")
}

func (a *API) registerFinish(c fiber.Ctx) error { return a.finishRegistration(c, false) }
func (a *API) passkeyFinish(c fiber.Ctx) error  { return a.finishRegistration(c, true) }
func (a *API) finishRegistration(c fiber.Ctx, additional bool) error {
	if err := a.authConfigured(); err != nil {
		return err
	}
	if !additional && viewer(c) != "" {
		return fail(409, "already_authenticated", "You are already signed in")
	}
	var raw json.RawMessage
	if err := body(c, &raw); err != nil {
		return err
	}
	parsed, err := protocol.ParseCredentialCreationResponseBody(bytes.NewReader(raw))
	if err != nil {
		return fail(400, "invalid_credential", "The passkey response is invalid")
	}
	kind := "webauthn-register"
	if additional {
		kind = "webauthn-add"
	}
	s, data, err := a.consumeChallenge(c, kind)
	if err != nil {
		return err
	}
	store := auth.NewStore(a.db)
	var user auth.User
	if additional {
		user, err = store.GetUser(c.Context(), viewer(c))
	} else {
		user, err = store.NewUser(c.Context(), s.Meta["user_id"], s.Meta["username"])
	}
	if err != nil {
		return err
	}
	credential, err := a.wa.CreateCredential(user, data, parsed)
	if err != nil {
		return fail(401, "invalid_credential", "This passkey could not be verified. Start again.")
	}
	if err = user.AddCredential(*credential); err != nil {
		return fail(409, "duplicate_passkey", "This passkey is already registered")
	}
	if credential.Extensions.RK != nil && !*credential.Extensions.RK {
		return fail(401, "invalid_credential", "A discoverable passkey is required")
	}
	if additional {
		if err = a.persistNewCredential(c.Context(), user.ID, credential, s); err != nil {
			return err
		}
	} else {
		creator, ok := a.db.(interface {
			CreateUserWithPasskeySession(context.Context, string, string, []byte, string, string) (*models.Session, error)
		})
		if !ok {
			return fail(503, "passkeys_unavailable", "Passkey storage is unavailable")
		}
		encoded, err := encodePasskey(credential, nil, true)
		if err != nil {
			return err
		}
		authenticated, err := creator.CreateUserWithPasskeySession(c.Context(), user.ID, user.Username, encoded, s.ID(), database.SessionChallengeClaim(s.Meta))
		if err != nil {
			if errors.Is(err, database.ErrUsernameNotAvailable) {
				return fail(409, "username_taken", "This username is already in use")
			}
			if errors.Is(err, database.ErrCredentialAlreadyRegistered) {
				return fail(409, "duplicate_passkey", "This passkey is already registered")
			}
			if database.IsErrNotFound(err) {
				return fail(401, "challenge_expired", "Start the passkey request again")
			}
			return err
		}
		if err := a.authenticatedCookie(c, authenticated.ID); err != nil {
			return err
		}
		return c.SendStatus(204)
	}
	if err = sessionCookie(c, s); err != nil {
		return err
	}
	return c.SendStatus(204)
}

func (a *API) persistNewCredential(ctx context.Context, userID string, credential *webauthn.Credential, s sessions.Session) error {
	encoded, err := encodePasskey(credential, nil, true)
	if err != nil {
		return err
	}
	writer, ok := a.db.(interface {
		AddUserPasskeyForClaim(context.Context, string, []byte, string, string) error
	})
	if !ok {
		return fail(503, "passkeys_unavailable", "Passkey storage is unavailable")
	}
	if err = writer.AddUserPasskeyForClaim(ctx, userID, encoded, s.ID(), database.SessionChallengeClaim(s.Meta)); errors.Is(err, database.ErrCredentialAlreadyRegistered) {
		return fail(409, "duplicate_passkey", "This passkey is already registered")
	}
	if database.IsErrNotFound(err) {
		return fail(401, "challenge_expired", "Start the passkey request again")
	}
	return err
}

func (a *API) authenticatedCookie(c fiber.Ctx, id string) error {
	s, err := a.sessions.GetReadOnly(c.Context(), id)
	if err != nil {
		return err
	}
	return sessionCookie(c, s)
}
