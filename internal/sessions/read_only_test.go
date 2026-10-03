package sessions

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/aarondl/null/v8"
	"github.com/cufee/feedlr-yt/internal/database"
	"github.com/cufee/feedlr-yt/internal/database/models"
)

type readOnlySessions struct {
	database.SessionsClient // Any accidental write/non-read-only lookup panics.
	record                  *models.Session
}

func (s readOnlySessions) GetSessionReadOnly(context.Context, string) (*models.Session, error) {
	return s.record, nil
}

func TestReadOnlyAuthenticationHonorsExpiryAndDeletion(t *testing.T) {
	for _, tc := range []struct {
		name                    string
		expired, deleted, valid bool
	}{
		{name: "valid", valid: true}, {name: "expired", expired: true}, {name: "revoked", deleted: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			expiry := time.Now().Add(time.Hour)
			if tc.expired {
				expiry = time.Now().Add(-time.Hour)
			}
			db := readOnlySessions{record: &models.Session{ID: "session", UserID: null.StringFrom("user"), ExpiresAt: expiry, Deleted: tc.deleted}}
			client, _ := New(db)
			session, err := client.GetReadOnly(context.Background(), "session")
			if tc.deleted && !errors.Is(err, ErrNotFound) {
				t.Fatalf("revoked session returned: %v", err)
			}
			_, valid := session.UserID()
			if valid != tc.valid {
				t.Fatalf("valid = %v", valid)
			}
		})
	}
}
