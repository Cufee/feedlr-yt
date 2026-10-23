package database

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/aarondl/null/v9"
	"github.com/cufee/feedlr-yt/internal/database/models"
)

func TestSessionChallengeCanBeConsumedOnlyOnceAcrossConnections(t *testing.T) {
	clients := podcastProcessingFixture(t, 0)
	ctx := context.Background()
	meta := map[string]string{"type": "webauthn-login", "data": "challenge", "user_id": "viewer"}
	encoded, err := json.Marshal(meta)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := clients[0].CreateSession(ctx, &models.Session{ID: "ceremony", ExpiresAt: time.Now().Add(time.Hour), Meta: encoded}); err != nil {
		t.Fatal(err)
	}
	if ok, err := clients[1].ConsumeSessionChallenge(ctx, "ceremony", map[string]string{"type": "webauthn-register"}); ok || err != nil {
		t.Fatalf("wrong challenge consumed session: %v %v", ok, err)
	}
	type result struct {
		claimed bool
		err     error
	}
	results := make(chan result, 20)
	start := make(chan struct{})
	var wg sync.WaitGroup
	for n := range 20 {
		wg.Go(func() {
			<-start
			ok, err := clients[n%2].ConsumeSessionChallenge(ctx, "ceremony", meta)
			results <- result{ok, err}
		})
	}
	close(start)
	wg.Wait()
	close(results)
	claims := 0
	for result := range results {
		if result.err != nil {
			t.Fatal(result.err)
		}
		if result.claimed {
			claims++
		}
	}
	if claims != 1 {
		t.Fatalf("challenge claimed %d times, want once", claims)
	}
	for _, state := range []string{"expired", "deleted"} {
		expiry := time.Now().Add(time.Hour)
		if state == "expired" {
			expiry = time.Now().Add(-time.Hour)
		}
		if _, err := clients[0].db.ExecContext(ctx, "UPDATE sessions SET meta=?,expires_at=?,deleted=? WHERE id='ceremony'", encoded, expiry, state == "deleted"); err != nil {
			t.Fatal(err)
		}
		if ok, err := clients[1].ConsumeSessionChallenge(ctx, "ceremony", meta); ok || err != nil {
			t.Fatalf("%s challenge consumed: %v %v", state, ok, err)
		}
	}
}

func TestRegistrationRollsBackAccountWhenCredentialCannotBeStored(t *testing.T) {
	clients := podcastProcessingFixture(t, 0)
	ctx := context.Background()
	if _, err := clients[0].db.ExecContext(ctx, "CREATE TRIGGER reject_initial_key BEFORE INSERT ON passkeys BEGIN SELECT RAISE(ABORT,'key storage failure'); END"); err != nil {
		t.Fatal(err)
	}
	if err := clients[0].CreateUserWithPasskey(ctx, "failed", "new-user", []byte(`{"id":"a2V5"}`)); err == nil {
		t.Fatal("credential storage failure unexpectedly accepted")
	}
	if _, err := clients[1].GetUser(ctx, "failed"); !IsErrNotFound(err) {
		t.Fatalf("failed credential reserved account: %v", err)
	}
	if _, err := clients[0].db.ExecContext(ctx, "DROP TRIGGER reject_initial_key"); err != nil {
		t.Fatal(err)
	}
	if err := clients[1].CreateUserWithPasskey(ctx, "good", "new-user", []byte(`{"id":"a2V5"}`)); err != nil {
		t.Fatal(err)
	}
	if err := clients[0].CreateUserWithPasskey(ctx, "duplicate", "NEW-USER", []byte(`{"id":"b3RoZXI="}`)); !errors.Is(err, ErrUsernameNotAvailable) {
		t.Fatalf("case-insensitive username conflict accepted: %v", err)
	}
	if keys, err := clients[0].GetUserPasskeys(ctx, "good"); err != nil || len(keys) != 1 {
		t.Fatalf("initial passkey missing: %v %v", keys, err)
	}
}

func TestCredentialIDsAreGloballyUniqueAcrossConcurrentRegistrations(t *testing.T) {
	clients := podcastProcessingFixture(t, 0)
	ctx := context.Background()
	errs := make(chan error, 16)
	var wg sync.WaitGroup
	for n := range 16 {
		wg.Go(func() {
			errs <- clients[n%2].CreateUserWithPasskey(ctx, fmt.Sprintf("user-%d", n), fmt.Sprintf("user-%d", n), []byte(`{"id":"Z2xvYmFs"}`))
		})
	}
	wg.Wait()
	close(errs)
	success := 0
	for err := range errs {
		if err == nil {
			success++
		} else if !errors.Is(err, ErrCredentialAlreadyRegistered) {
			t.Fatal(err)
		}
	}
	if success != 1 {
		t.Fatalf("same credential enrolled in %d accounts", success)
	}
	if _, err := clients[0].CreateUser(ctx, "other", "other"); err != nil {
		t.Fatal(err)
	}
	if err := clients[1].AddUserPasskey(ctx, "other", []byte(`{"id":"Z2xvYmFs"}`)); !errors.Is(err, ErrCredentialAlreadyRegistered) {
		t.Fatalf("additional enrollment reused another account's credential: %v", err)
	}
}

func TestPasskeyUpdatesPreserveLabelsAndCannotResurrectDeletedKeys(t *testing.T) {
	clients := podcastProcessingFixture(t, 0)
	ctx := context.Background()
	if err := clients[0].CreateUserWithPasskey(ctx, "viewer", "viewer", []byte(`{"id":"a2V5"}`)); err != nil {
		t.Fatal(err)
	}
	keys, err := clients[0].GetUserPasskeys(ctx, "viewer")
	if err != nil {
		t.Fatal(err)
	}
	key := keys[0]
	claim := func(id string) AuthenticationProof {
		meta := map[string]string{"type": "webauthn-login", "data": id}
		encoded, err := json.Marshal(meta)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := clients[0].CreateSession(ctx, &models.Session{ID: id, ExpiresAt: time.Now().Add(time.Hour), Meta: encoded}); err != nil {
			t.Fatal(err)
		}
		if ok, err := clients[0].ConsumeSessionChallenge(ctx, id, meta); err != nil || !ok {
			t.Fatalf("claim: %v %v", ok, err)
		}
		return AuthenticationProof{SourceID: id, UserID: "viewer", Claim: SessionChallengeClaim(meta), PasskeyID: key.ID, ExpectedCredential: key.Data, UpdatedCredential: []byte(`{"id":"a2V5","authenticator":{"signCount":1}}`)}
	}
	if _, err := clients[1].db.ExecContext(ctx, "UPDATE passkeys SET label='My phone' WHERE id=?", key.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := clients[0].AuthenticateSession(ctx, claim("current")); err != nil {
		t.Fatalf("credential CAS: %v", err)
	}
	keys, err = clients[1].GetUserPasskeys(ctx, "viewer")
	if err != nil || keys[0].Label != "My phone" {
		t.Fatalf("authentication overwrote passkey label: %v %v", keys, err)
	}
	if _, err := clients[0].AuthenticateSession(ctx, claim("stale")); !errors.Is(err, ErrCredentialChanged) {
		t.Fatalf("stale credential update succeeded: %v", err)
	}
	if err := clients[1].DeleteUserPasskey(ctx, "viewer", key.ID); err != nil {
		t.Fatal(err)
	}
	deleted := claim("deleted")
	deleted.ExpectedCredential = keys[0].Data
	if _, err := clients[0].AuthenticateSession(ctx, deleted); !errors.Is(err, ErrCredentialChanged) {
		t.Fatalf("deleted credential resurrected: %v", err)
	}
}

func TestConcurrentPasskeyRemovalAlwaysLeavesOneCredential(t *testing.T) {
	clients := podcastProcessingFixture(t, 0)
	ctx := context.Background()
	if err := clients[0].CreateUserWithPasskey(ctx, "viewer", "viewer", []byte(`{"id":"a2V5"}`)); err != nil {
		t.Fatal(err)
	}
	if err := clients[0].AddUserPasskey(ctx, "viewer", []byte(`{"id":"b3RoZXI="}`)); err != nil {
		t.Fatal(err)
	}
	keys, err := clients[0].GetUserPasskeys(ctx, "viewer")
	if err != nil {
		t.Fatal(err)
	}
	errs := make(chan error, 2)
	var wg sync.WaitGroup
	for n, key := range keys {
		wg.Go(func() { errs <- clients[n].DeleteUserPasskeyKeepingOne(ctx, "viewer", key.ID) })
	}
	wg.Wait()
	close(errs)
	success, protected := 0, 0
	for err := range errs {
		if err == nil {
			success++
		} else if errors.Is(err, ErrLastPasskey) {
			protected++
		} else {
			t.Fatal(err)
		}
	}
	if success != 1 || protected != 1 {
		t.Fatalf("concurrent removals: %d succeeded, %d protected", success, protected)
	}
	keys, err = clients[0].GetUserPasskeys(ctx, "viewer")
	if err != nil || len(keys) != 1 {
		t.Fatalf("account can no longer authenticate: %v %v", keys, err)
	}
}

func TestAuthenticationRotatesOnceAndCannotUndoLogoutOrNewChallenge(t *testing.T) {
	clients := podcastProcessingFixture(t, 0)
	ctx := context.Background()
	if err := clients[0].CreateUserWithPasskey(ctx, "viewer", "viewer", []byte(`{"id":"a2V5"}`)); err != nil {
		t.Fatal(err)
	}
	keys, err := clients[0].GetUserPasskeys(ctx, "viewer")
	if err != nil {
		t.Fatal(err)
	}
	challenge := map[string]string{"type": "webauthn-login", "data": "challenge-A"}
	meta, err := json.Marshal(challenge)
	if err != nil {
		t.Fatal(err)
	}
	proof := AuthenticationProof{UserID: "viewer", Claim: SessionChallengeClaim(challenge), PasskeyID: keys[0].ID, ExpectedCredential: keys[0].Data, UpdatedCredential: keys[0].Data}
	for _, id := range []string{"ready", "logged-out", "new-challenge"} {
		if _, err := clients[0].CreateSession(ctx, &models.Session{ID: id, ExpiresAt: time.Now().Add(time.Hour), Meta: meta}); err != nil {
			t.Fatal(err)
		}
		if ok, err := clients[0].ConsumeSessionChallenge(ctx, id, challenge); err != nil || !ok {
			t.Fatalf("claim: %v %v", ok, err)
		}
	}
	if err := clients[0].DeleteSession(ctx, "logged-out"); err != nil {
		t.Fatal(err)
	}
	proof.SourceID = "logged-out"
	if _, err := clients[1].AuthenticateSession(ctx, proof); !IsErrNotFound(err) {
		t.Fatalf("finish undid logout: %v", err)
	}
	newer := map[string]string{"type": "webauthn-login", "data": "challenge-B"}
	if err := clients[0].UpdateSessionMeta(ctx, "new-challenge", newer); err != nil {
		t.Fatal(err)
	}
	if ok, err := clients[0].ConsumeSessionChallenge(ctx, "new-challenge", newer); err != nil || !ok {
		t.Fatalf("newer claim: %v %v", ok, err)
	}
	proof.SourceID = "new-challenge"
	if _, err := clients[1].AuthenticateSession(ctx, proof); !IsErrNotFound(err) {
		t.Fatalf("old finish superseded newer begin: %v", err)
	}
	type result struct {
		session *models.Session
		err     error
	}
	results := make(chan result, 2)
	var wg sync.WaitGroup
	proof.SourceID = "ready"
	for n := range 2 {
		wg.Go(func() { s, err := clients[n].AuthenticateSession(ctx, proof); results <- result{s, err} })
	}
	wg.Wait()
	close(results)
	success := 0
	for r := range results {
		if r.err == nil {
			success++
			if r.session.ID == "ready" || r.session.UserID.String != "viewer" {
				t.Fatalf("wrong rotated session: %+v", r.session)
			}
		} else if !IsErrNotFound(r.err) {
			t.Fatal(r.err)
		}
	}
	if success != 1 {
		t.Fatalf("consumed ceremony authenticated %d sessions", success)
	}
	if _, err := clients[0].GetSessionReadOnly(ctx, "ready"); !IsErrNotFound(err) {
		t.Fatal("anonymous ceremony session still valid after rotation")
	}
}

func TestSupersededRegistrationAndAdditionalKeyHaveNoCommittedSideEffects(t *testing.T) {
	clients := podcastProcessingFixture(t, 0)
	ctx := context.Background()
	if err := clients[0].CreateUserWithPasskey(ctx, "viewer", "viewer", []byte(`{"id":"a2V5"}`)); err != nil {
		t.Fatal(err)
	}
	first := map[string]string{"type": "webauthn-register", "data": "challenge-A"}
	second := map[string]string{"type": "webauthn-register", "data": "challenge-B"}
	encoded, err := json.Marshal(first)
	if err != nil {
		t.Fatal(err)
	}
	for _, id := range []string{"signup", "add"} {
		session := &models.Session{ID: id, ExpiresAt: time.Now().Add(time.Hour), Meta: encoded}
		if id == "add" {
			session.UserID = null.StringFrom("viewer")
		}
		if _, err := clients[0].CreateSession(ctx, session); err != nil {
			t.Fatal(err)
		}
		if ok, err := clients[0].ConsumeSessionChallenge(ctx, id, first); !ok || err != nil {
			t.Fatalf("first claim: %v %v", ok, err)
		}
		if err := clients[0].UpdateSessionMeta(ctx, id, second); err != nil {
			t.Fatal(err)
		}
		if ok, err := clients[0].ConsumeSessionChallenge(ctx, id, second); !ok || err != nil {
			t.Fatalf("second claim: %v %v", ok, err)
		}
	}
	if _, err := clients[1].CreateUserWithPasskeySession(ctx, "stale", "stale-account", []byte(`{"id":"c3RhbGU="}`), "signup", SessionChallengeClaim(first)); !IsErrNotFound(err) {
		t.Fatalf("stale registration committed: %v", err)
	}
	if _, err := clients[1].GetUser(ctx, "stale"); !IsErrNotFound(err) {
		t.Fatalf("stale registration reserved username: %v", err)
	}
	if err := clients[1].AddUserPasskeyForClaim(ctx, "viewer", []byte(`{"id":"c3RhbGU="}`), "add", SessionChallengeClaim(first)); !IsErrNotFound(err) {
		t.Fatalf("stale key addition committed: %v", err)
	}
	keys, err := clients[0].GetUserPasskeys(ctx, "viewer")
	if err != nil || len(keys) != 1 {
		t.Fatalf("stale addition changed keys: %v %v", keys, err)
	}
}
