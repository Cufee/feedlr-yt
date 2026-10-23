package database

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/aarondl/null/v9"
	"github.com/cufee/feedlr-yt/internal/database/models"
)

func TestSessionRefreshCannotRestoreConsumedChallengeOrRevokedSession(t *testing.T) {
	clients := podcastProcessingFixture(t, 0)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if _, err := clients[0].CreateUser(ctx, "viewer", "viewer"); err != nil {
		t.Fatal(err)
	}
	challenge := map[string]string{"type": "webauthn-add", "user_id": "viewer", "data": "challenge"}
	meta, err := json.Marshal(challenge)
	if err != nil {
		t.Fatal(err)
	}
	consumed := claimedSessionMeta(SessionChallengeClaim(challenge))
	for _, id := range []string{"consumed", "revoked"} {
		if _, err := clients[0].CreateSession(ctx, &models.Session{ID: id, ExpiresAt: time.Now().Add(time.Hour), UserID: null.StringFrom("viewer"), Meta: meta}); err != nil {
			t.Fatal(err)
		}
		// Commit the security change while another connection tries to refresh.
		// BEGIN IMMEDIATE models the native consume/logout transaction boundary;
		// it never depends on SQLBoiler hooks that refresh no longer invokes.
		conn, err := clients[0].db.Conn(ctx)
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { _, _ = conn.ExecContext(context.Background(), "ROLLBACK"); _ = conn.Close() })
		if _, err := conn.ExecContext(ctx, "BEGIN IMMEDIATE"); err != nil {
			t.Fatal(err)
		}
		deleted := id == "revoked"
		if _, err := conn.ExecContext(ctx, "UPDATE sessions SET meta=?,deleted=? WHERE id=?", consumed, deleted, id); err != nil {
			t.Fatal(err)
		}
		type result struct {
			s   *models.Session
			err error
		}
		done := make(chan result, 1)
		go func() {
			s, err := clients[1].SetSessionExpiration(ctx, id, time.Now().Add(7*24*time.Hour))
			done <- result{s, err}
		}()
		// Observe the independent connection entering SQLite while the security
		// write is uncommitted, then release it. The test is bounded on failure.
		deadline := time.Now().Add(time.Second)
		for clients[1].db.Stats().InUse == 0 && time.Now().Before(deadline) {
			time.Sleep(time.Millisecond)
		}
		if clients[1].db.Stats().InUse == 0 {
			t.Fatal("refresh never entered independent SQLite connection")
		}
		if _, err := conn.ExecContext(ctx, "COMMIT"); err != nil {
			t.Fatal(err)
		}
		if err := conn.Close(); err != nil {
			t.Fatal(err)
		}
		var refreshed result
		select {
		case refreshed = <-done:
		case <-ctx.Done():
			t.Fatal(ctx.Err())
		}
		if deleted {
			if !IsErrNotFound(refreshed.err) {
				t.Fatalf("revoked session refreshed: %+v %v", refreshed.s, refreshed.err)
			}
		} else if refreshed.err != nil || string(refreshed.s.Meta) != string(consumed) || refreshed.s.UserID.String != "viewer" {
			t.Fatalf("refresh returned stale security fields: %+v %v", refreshed.s, refreshed.err)
		}
		var gotDeleted bool
		var gotMeta []byte
		if err := clients[0].db.QueryRowContext(ctx, "SELECT deleted,meta FROM sessions WHERE id=?", id).Scan(&gotDeleted, &gotMeta); err != nil {
			t.Fatal(err)
		}
		if gotDeleted != deleted || string(gotMeta) != string(consumed) {
			t.Fatalf("refresh restored security state: deleted=%v meta=%s", gotDeleted, gotMeta)
		}
	}
}
