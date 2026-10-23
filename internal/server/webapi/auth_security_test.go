package webapi

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/binary"
	"encoding/json"
	"testing"
	"time"

	"github.com/cufee/feedlr-yt/internal/database/models"
	"github.com/cufee/feedlr-yt/internal/sessions"
	"github.com/fxamacker/cbor/v2"
	"github.com/go-webauthn/webauthn/webauthn"
	"github.com/gofiber/fiber/v3"
)

type assertionKey struct {
	private    *ecdsa.PrivateKey
	id         []byte
	credential webauthn.Credential
}

func makeAssertionKey(t *testing.T, id string, backup bool, count uint32) assertionKey {
	t.Helper()
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	public, err := cbor.Marshal(map[int]any{1: 2, 3: -7, -1: 1, -2: key.X.FillBytes(make([]byte, 32)), -3: key.Y.FillBytes(make([]byte, 32))})
	if err != nil {
		t.Fatal(err)
	}
	return assertionKey{private: key, id: []byte(id), credential: webauthn.Credential{ID: []byte(id), PublicKey: public, AttestationType: "none", Flags: webauthn.CredentialFlags{BackupEligible: backup, UserPresent: true, UserVerified: true}, Authenticator: webauthn.Authenticator{SignCount: count}}}
}

func saveAssertionKey(t *testing.T, f *fixture, key assertionKey, modern bool) {
	t.Helper()
	var encoded []byte
	var err error
	if modern {
		encoded, err = encodePasskey(&key.credential, nil, true)
	} else {
		encoded, err = json.Marshal(key.credential)
	}
	if err != nil {
		t.Fatal(err)
	}
	if err = f.db.SaveUserPasskey(context.Background(), &models.Passkey{UserID: "viewer", Data: encoded}); err != nil {
		t.Fatal(err)
	}
}

type assertionInput struct {
	id, handle            []byte
	challenge, origin, rp string
	flags                 byte
	count                 uint32
	crossOrigin           bool
	badSignature          bool
}

func (key assertionKey) assertion(t *testing.T, input assertionInput) string {
	t.Helper()
	clientData, err := json.Marshal(map[string]any{"type": "webauthn.get", "challenge": input.challenge, "origin": input.origin, "crossOrigin": input.crossOrigin})
	if err != nil {
		t.Fatal(err)
	}
	rpHash := sha256.Sum256([]byte(input.rp))
	authData := append([]byte{}, rpHash[:]...)
	authData = append(authData, input.flags, 0, 0, 0, 0)
	binary.BigEndian.PutUint32(authData[33:], input.count)
	clientHash := sha256.Sum256(clientData)
	payload := append(append([]byte{}, authData...), clientHash[:]...)
	hash := sha256.Sum256(payload)
	signature, err := ecdsa.SignASN1(rand.Reader, key.private, hash[:])
	if err != nil {
		t.Fatal(err)
	}
	if input.badSignature {
		signature[len(signature)-1] ^= 1
	}
	response := map[string]any{"clientDataJSON": base64.RawURLEncoding.EncodeToString(clientData), "authenticatorData": base64.RawURLEncoding.EncodeToString(authData), "signature": base64.RawURLEncoding.EncodeToString(signature)}
	if input.handle != nil {
		response["userHandle"] = base64.RawURLEncoding.EncodeToString(input.handle)
	}
	encoded, err := json.Marshal(map[string]any{"id": base64.RawURLEncoding.EncodeToString(input.id), "rawId": base64.RawURLEncoding.EncodeToString(input.id), "type": "public-key", "response": response, "clientExtensionResults": map[string]any{}})
	if err != nil {
		t.Fatal(err)
	}
	return string(encoded)
}

func beginAssertion(t *testing.T, f *fixture, body, cookie string) (string, string, map[string]any) {
	t.Helper()
	response, value := f.request(t, "POST", "/auth/login/begin", body, cookie, "")
	if response.StatusCode != 200 {
		t.Fatalf("begin: %d %#v", response.StatusCode, value)
	}
	f.check(t, value, "WebAuthnRequestOptions")
	options := value.(map[string]any)["publicKey"].(map[string]any)
	return options["challenge"].(string), sessionID(t, response), options
}

func TestDiscoverableAssertionsBindOriginRPHandleCredentialAndVerification(t *testing.T) {
	f := newFixture(t)
	key := makeAssertionKey(t, "modern", false, 0)
	saveAssertionKey(t, f, key, true)
	for _, test := range []struct {
		name   string
		change func(*assertionInput)
	}{
		{"missing handle", func(i *assertionInput) { i.handle = nil }},
		{"unknown handle", func(i *assertionInput) { i.handle = []byte("missing-user") }},
		{"other account handle", func(i *assertionInput) { i.handle = []byte("other") }},
		{"oversize handle", func(i *assertionInput) { i.handle = make([]byte, 65) }},
		{"unknown credential", func(i *assertionInput) { i.id = []byte("not-registered") }},
		{"wrong origin", func(i *assertionInput) { i.origin = "https://evil.example" }},
		{"wrong RP", func(i *assertionInput) { i.rp = "evil.example" }},
		{"wrong challenge", func(i *assertionInput) { i.challenge = base64.RawURLEncoding.EncodeToString(make([]byte, 32)) }},
		{"no presence", func(i *assertionInput) { i.flags = 4 }},
		{"no verification", func(i *assertionInput) { i.flags = 1 }},
		{"cross origin", func(i *assertionInput) { i.crossOrigin = true }},
		{"bad signature", func(i *assertionInput) { i.badSignature = true }},
		{"changed backup eligibility", func(i *assertionInput) { i.flags |= 8 }},
		{"invalid backup state", func(i *assertionInput) { i.flags |= 16 }},
	} {
		t.Run(test.name, func(t *testing.T) {
			challenge, cookie, options := beginAssertion(t, f, `{}`, "")
			if options["userVerification"] != "required" {
				t.Fatalf("discoverable login weakened UV: %#v", options)
			}
			if allowed, ok := options["allowCredentials"]; ok && len(allowed.([]any)) != 0 {
				t.Fatalf("discoverable login disclosed credential IDs: %#v", options)
			}
			input := assertionInput{id: key.id, handle: []byte("viewer"), challenge: challenge, origin: "http://example.com", rp: "example.com", flags: 5, count: 1}
			test.change(&input)
			body := key.assertion(t, input)
			for range 2 {
				res, value := f.request(t, "POST", "/auth/login/finish", body, cookie, "")
				if res.StatusCode != 401 {
					t.Fatalf("invalid assertion or replay: %d %#v", res.StatusCode, value)
				}
				f.check(t, value, "Error")
			}
		})
	}
	challenge, cookie, _ := beginAssertion(t, f, `{}`, "")
	body := key.assertion(t, assertionInput{id: key.id, handle: []byte("viewer"), challenge: challenge, origin: "http://example.com", rp: "example.com", flags: 5, count: 1})
	response, value := f.request(t, "POST", "/auth/login/finish", body, cookie, "")
	if response.StatusCode != 204 {
		t.Fatalf("valid discoverable assertion: %d %#v", response.StatusCode, value)
	}
	authenticated := sessionID(t, response)
	if authenticated == cookie {
		t.Fatal("authentication did not rotate anonymous session")
	}
	response, value = f.request(t, "GET", "/session", "", authenticated, "")
	if response.StatusCode != 200 || value.(map[string]any)["userId"] != "viewer" {
		t.Fatalf("wrong authenticated account: %#v", value)
	}
	response, value = f.request(t, "POST", "/auth/login/finish", body, cookie, "")
	if response.StatusCode != 401 {
		t.Fatalf("consumed anonymous session still usable: %d %#v", response.StatusCode, value)
	}
}

func TestLegacyFallbackCannotDowngradeNewPasskeyVerification(t *testing.T) {
	f := newFixture(t)
	legacy := makeAssertionKey(t, "legacy", false, 0)
	modern := makeAssertionKey(t, "modern", false, 0)
	saveAssertionKey(t, f, legacy, false)
	saveAssertionKey(t, f, modern, true)
	challenge, cookie, options := beginAssertion(t, f, `{"username":"viewer"}`, "")
	if options["userVerification"] != "preferred" {
		t.Fatalf("old nonresident keys cannot use compatibility flow: %#v", options)
	}
	input := assertionInput{id: modern.id, challenge: challenge, origin: "http://example.com", rp: "example.com", flags: 1, count: 1}
	res, value := f.request(t, "POST", "/auth/login/finish", modern.assertion(t, input), cookie, "")
	if res.StatusCode != 401 {
		t.Fatalf("new passkey lost UV requirement via named fallback: %d %#v", res.StatusCode, value)
	}
	challenge, cookie, _ = beginAssertion(t, f, `{"username":"viewer"}`, "")
	input.id, input.challenge = legacy.id, challenge
	res, value = f.request(t, "POST", "/auth/login/finish", legacy.assertion(t, input), cookie, "")
	if res.StatusCode != 204 {
		t.Fatalf("genuinely old key was locked out: %d %#v", res.StatusCode, value)
	}
	challenge, cookie, _ = beginAssertion(t, f, `{"username":"viewer"}`, "")
	input.id, input.challenge, input.flags = modern.id, challenge, 5
	res, value = f.request(t, "POST", "/auth/login/finish", modern.assertion(t, input), cookie, "")
	if res.StatusCode != 204 {
		t.Fatalf("verified modern fallback failed: %d %#v", res.StatusCode, value)
	}
	keys, err := f.db.GetUserPasskeys(context.Background(), "viewer")
	if err != nil {
		t.Fatal(err)
	}
	for _, key := range keys {
		var credential webauthn.Credential
		if err := json.Unmarshal(key.Data, &credential); err != nil {
			t.Fatal(err)
		}
		if string(credential.ID) == "modern" && !requiredPasskeyVerification(key.Data) {
			t.Fatal("login dropped durable UV policy")
		}
	}
}

func TestBackupEligibleZeroCountersAndDeviceBoundRegressionPolicy(t *testing.T) {
	f := newFixture(t)
	key := makeAssertionKey(t, "synced", true, 0)
	saveAssertionKey(t, f, key, true)
	for _, flags := range []byte{13, 29, 13} {
		challenge, cookie, _ := beginAssertion(t, f, `{}`, "")
		res, value := f.request(t, "POST", "/auth/login/finish", key.assertion(t, assertionInput{id: key.id, handle: []byte("viewer"), challenge: challenge, origin: "http://example.com", rp: "example.com", flags: flags, count: 0}), cookie, "")
		if res.StatusCode != 204 {
			t.Fatalf("valid synced passkey/counter/backup state: %d %#v", res.StatusCode, value)
		}
		keys, err := f.db.GetUserPasskeys(context.Background(), "viewer")
		if err != nil {
			t.Fatal(err)
		}
		var stored webauthn.Credential
		if err := json.Unmarshal(keys[0].Data, &stored); err != nil {
			t.Fatal(err)
		}
		if !stored.Flags.BackupEligible || stored.Flags.BackupState != (flags&16 != 0) || stored.Authenticator.SignCount != 0 || !requiredPasskeyVerification(keys[0].Data) {
			t.Fatalf("synced state/policy did not persist: %+v", stored)
		}
	}
	device := makeAssertionKey(t, "device", false, 10)
	saveAssertionKey(t, f, device, true)
	challenge, cookie, _ := beginAssertion(t, f, `{}`, "")
	res, value := f.request(t, "POST", "/auth/login/finish", device.assertion(t, assertionInput{id: device.id, handle: []byte("viewer"), challenge: challenge, origin: "http://example.com", rp: "example.com", flags: 5, count: 9}), cookie, "")
	if res.StatusCode != 401 {
		t.Fatalf("device-bound counter regression ignored: %d %#v", res.StatusCode, value)
	}
}

func TestAPIDeletingFinalPasskeyReturnsAnExplicitConflict(t *testing.T) {
	f := newFixture(t)
	saveAssertionKey(t, f, makeAssertionKey(t, "last", false, 0), true)
	keys, err := f.db.GetUserPasskeys(context.Background(), "viewer")
	if err != nil {
		t.Fatal(err)
	}
	response, value := f.request(t, "DELETE", "/passkeys/"+keys[0].ID, "", f.session, "")
	if response.StatusCode != 409 || value.(map[string]any)["error"].(map[string]any)["code"] != "last_passkey" {
		t.Fatalf("final passkey deletion did not return recoverable conflict: %d %#v", response.StatusCode, value)
	}
	f.check(t, value, "Error")
}

func TestDiscoverableBeginSupersedesPriorChallengeAndServerExpiresIt(t *testing.T) {
	f := newFixture(t)
	key := makeAssertionKey(t, "modern", false, 0)
	saveAssertionKey(t, f, key, true)
	first, cookie, _ := beginAssertion(t, f, `{}`, "")
	second, next, _ := beginAssertion(t, f, `{}`, cookie)
	if first == second || next != cookie {
		t.Fatal("begin did not reuse namespace with a fresh challenge")
	}
	sc, err := sessions.New(f.db)
	if err != nil {
		t.Fatal(err)
	}
	s, err := sc.Get(context.Background(), cookie)
	if err != nil {
		t.Fatal(err)
	}
	var data webauthn.SessionData
	if err := json.Unmarshal([]byte(s.Meta["data"]), &data); err != nil {
		t.Fatal(err)
	}
	if data.Expires.IsZero() || data.Expires.After(time.Now().Add(5*time.Minute)) {
		t.Fatal("challenge has no bounded server deadline")
	}
	data.Expires = time.Now().Add(-time.Second)
	encoded, err := json.Marshal(data)
	if err != nil {
		t.Fatal(err)
	}
	s.Meta["data"] = string(encoded)
	if err := f.db.UpdateSessionMeta(context.Background(), cookie, s.Meta); err != nil {
		t.Fatal(err)
	}
	res, value := f.request(t, "POST", "/auth/login/finish", key.assertion(t, assertionInput{id: key.id, handle: []byte("viewer"), challenge: second, origin: "http://example.com", rp: "example.com", flags: 5, count: 1}), cookie, "")
	if res.StatusCode != 401 {
		t.Fatalf("expired challenge accepted: %d %#v", res.StatusCode, value)
	}
}

func TestRegistrationRequiresDiscoverableVerifiedPrivatePasskeys(t *testing.T) {
	f := newFixture(t)
	response, value := f.request(t, "POST", "/auth/register/begin", `{"username":"modernuser"}`, "", "")
	if response.StatusCode != 200 {
		t.Fatalf("registration: %d %#v", response.StatusCode, value)
	}
	f.check(t, value, "WebAuthnCreationOptions")
	options := value.(map[string]any)["publicKey"].(map[string]any)
	selection := options["authenticatorSelection"].(map[string]any)
	if selection["residentKey"] != "required" || selection["requireResidentKey"] != true || selection["userVerification"] != "required" || options["attestation"] != "none" {
		t.Fatalf("weak registration policy: %#v", options)
	}
	if attachment, ok := selection["authenticatorAttachment"]; ok && attachment != "" {
		t.Fatal("registration excluded security keys/hybrid/platform choice")
	}
	encodedID := options["user"].(map[string]any)["id"].(string)
	handle, err := base64.RawURLEncoding.DecodeString(encodedID)
	if err != nil || len(handle) == 0 || len(handle) > 64 || string(handle) == "modernuser" {
		t.Fatalf("user handle exposes identity or exceeds specification limit: %q %v", handle, err)
	}
}

func (key assertionKey) registration(t *testing.T, challenge, origin, rp string, flags byte, resident bool) string {
	t.Helper()
	client, err := json.Marshal(map[string]any{"type": "webauthn.create", "challenge": challenge, "origin": origin})
	if err != nil {
		t.Fatal(err)
	}
	rpHash := sha256.Sum256([]byte(rp))
	authData := append([]byte{}, rpHash[:]...)
	authData = append(authData, flags, 0, 0, 0, 0)
	authData = append(authData, make([]byte, 16)...)
	authData = binary.BigEndian.AppendUint16(authData, uint16(len(key.id)))
	authData = append(authData, key.id...)
	authData = append(authData, key.credential.PublicKey...)
	attestation, err := cbor.Marshal(map[string]any{"fmt": "none", "authData": authData, "attStmt": map[string]any{}})
	if err != nil {
		t.Fatal(err)
	}
	encoded, err := json.Marshal(map[string]any{
		"id": base64.RawURLEncoding.EncodeToString(key.id), "rawId": base64.RawURLEncoding.EncodeToString(key.id), "type": "public-key",
		"response":               map[string]any{"clientDataJSON": base64.RawURLEncoding.EncodeToString(client), "attestationObject": base64.RawURLEncoding.EncodeToString(attestation), "transports": []string{"internal", "hybrid"}},
		"clientExtensionResults": map[string]any{"credProps": map[string]any{"rk": resident}},
	})
	if err != nil {
		t.Fatal(err)
	}
	return string(encoded)
}

func TestRegistrationVerificationRejectsUnverifiedOrInvalidScopeBeforeCreatingAccount(t *testing.T) {
	f := newFixture(t)
	key := makeAssertionKey(t, "enrollment", false, 0)
	for _, test := range []struct {
		name, origin, rp string
		flags            byte
		resident         bool
	}{
		{"no verification", "http://example.com", "example.com", 65, true},
		{"no presence", "http://example.com", "example.com", 68, true},
		{"wrong origin", "https://evil.example", "example.com", 69, true},
		{"wrong RP", "http://example.com", "evil.example", 69, true},
		{"invalid backup state", "http://example.com", "example.com", 85, true},
		{"non-discoverable result", "http://example.com", "example.com", 69, false},
	} {
		t.Run(test.name, func(t *testing.T) {
			response, value := f.request(t, "POST", "/auth/register/begin", `{"username":"enrollment"}`, "", "")
			if response.StatusCode != 200 {
				t.Fatalf("begin: %d %#v", response.StatusCode, value)
			}
			challenge := value.(map[string]any)["publicKey"].(map[string]any)["challenge"].(string)
			cookie := sessionID(t, response)
			body := key.registration(t, challenge, test.origin, test.rp, test.flags, test.resident)
			response, value = f.request(t, "POST", "/auth/register/finish", body, cookie, "")
			if response.StatusCode != 401 {
				t.Fatalf("invalid creation accepted: %d %#v", response.StatusCode, value)
			}
			if _, err := f.db.FindUser(context.Background(), "enrollment"); err == nil {
				t.Fatal("failed registration created an account")
			}
		})
	}
	response, value := f.request(t, "POST", "/auth/register/begin", `{"username":"enrollment"}`, "", "")
	if response.StatusCode != 200 {
		t.Fatalf("begin: %d %#v", response.StatusCode, value)
	}
	challenge := value.(map[string]any)["publicKey"].(map[string]any)["challenge"].(string)
	response, value = f.request(t, "POST", "/auth/register/finish", key.registration(t, challenge, "http://example.com", "example.com", 69, true), sessionID(t, response), "")
	if response.StatusCode != 204 {
		t.Fatalf("valid privacy-preserving enrollment failed: %d %#v", response.StatusCode, value)
	}
	user, err := f.db.FindUser(context.Background(), "enrollment")
	if err != nil {
		t.Fatal(err)
	}
	keys, err := f.db.GetUserPasskeys(context.Background(), user.ID)
	if err != nil || len(keys) != 1 || !requiredPasskeyVerification(keys[0].Data) {
		t.Fatalf("registration did not persist durable verification policy: %v %v", keys, err)
	}
}

func TestCeremonyOriginBindingDoesNotAcceptAnotherConfiguredOrigin(t *testing.T) {
	f := newFixture(t)
	key := makeAssertionKey(t, "modern", false, 0)
	saveAssertionKey(t, f, key, true)
	wa, err := webauthn.New(&webauthn.Config{RPDisplayName: "Feedlr", RPID: "example.com", RPOrigins: []string{"http://example.com", "https://example.com"}})
	if err != nil {
		t.Fatal(err)
	}
	sc, err := sessions.New(f.db)
	if err != nil {
		t.Fatal(err)
	}
	f.app = fiber.New()
	Register(f.app, f.db, sc, wa)
	response, value := f.request(t, "POST", "/auth/login/begin", `{}`, "", "http://example.com")
	if response.StatusCode != 200 {
		t.Fatalf("begin: %d %#v", response.StatusCode, value)
	}
	challenge := value.(map[string]any)["publicKey"].(map[string]any)["challenge"].(string)
	response, value = f.request(t, "POST", "/auth/login/finish", key.assertion(t, assertionInput{id: key.id, handle: []byte("viewer"), challenge: challenge, origin: "https://example.com", rp: "example.com", flags: 5, count: 1}), sessionID(t, response), "http://example.com")
	if response.StatusCode != 401 {
		t.Fatalf("a different allowed origin completed a bound ceremony: %d %#v", response.StatusCode, value)
	}
}
