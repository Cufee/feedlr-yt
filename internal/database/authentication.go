package database

import (
	"bytes"
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"time"

	"github.com/aarondl/null/v9"
	"github.com/aarondl/sqlboiler/v4/boil"
	"github.com/cufee/feedlr-yt/internal/database/models"
	"github.com/segmentio/ksuid"
)

var ErrCredentialAlreadyRegistered = errors.New("credential already registered")
var ErrLastPasskey = errors.New("at least one passkey must remain")
var ErrCredentialChanged = errors.New("credential changed during verification")

// Claim identity survives consumption without retaining a reusable challenge.
// Distinct ceremonies cannot become indistinguishable when both are consumed.
func SessionChallengeClaim(expected map[string]string) string {
	encoded, _ := json.Marshal(expected)
	hash := sha256.Sum256(encoded)
	return hex.EncodeToString(hash[:])
}

func claimedSessionMeta(claim string) []byte {
	encoded, _ := json.Marshal(map[string]string{"type": "webauthn-claimed", "claim": claim})
	return encoded
}

func requireSessionClaim(ctx context.Context, conn *stateConn, sourceID, claim string, userID *string) error {
	var owned bool
	query := "SELECT EXISTS(SELECT 1 FROM sessions WHERE id=? AND deleted=0 AND expires_at>? AND meta=? AND user_id IS NULL)"
	args := []any{sourceID, time.Now(), claimedSessionMeta(claim)}
	if userID != nil {
		query = "SELECT EXISTS(SELECT 1 FROM sessions WHERE id=? AND deleted=0 AND expires_at>? AND meta=? AND user_id=?)"
		args = append(args, *userID)
	}
	if err := conn.QueryRowContext(ctx, query, args...).Scan(&owned); err != nil {
		return err
	}
	if !owned {
		return sql.ErrNoRows
	}
	return nil
}

func credentialID(data []byte) ([]byte, error) {
	var credential struct {
		ID []byte `json:"id"`
	}
	if err := json.Unmarshal(data, &credential); err != nil {
		return nil, err
	}
	if len(credential.ID) == 0 || len(credential.ID) > 1023 {
		return nil, errors.New("invalid credential ID")
	}
	return credential.ID, nil
}

func ensureUniqueCredential(ctx context.Context, conn *stateConn, data []byte) error {
	id, err := credentialID(data)
	if err != nil {
		return err
	}
	rows, err := conn.QueryContext(ctx, "SELECT data FROM passkeys")
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		var stored []byte
		if err := rows.Scan(&stored); err != nil {
			return err
		}
		other, err := credentialID(stored)
		if err != nil {
			return err
		}
		if bytes.Equal(id, other) {
			return ErrCredentialAlreadyRegistered
		}
	}
	return rows.Err()
}

// ConsumeSessionChallenge claims a ceremony once, including when finish
// requests arrive simultaneously. A failed verification requires a new begin.
func (c *sqliteClient) ConsumeSessionChallenge(ctx context.Context, id string, expected map[string]string) (bool, error) {
	encoded, err := json.Marshal(expected)
	if err != nil {
		return false, err
	}
	claimed := false
	err = c.immediate(ctx, func(conn *stateConn) error {
		result, err := conn.ExecContext(ctx, "UPDATE sessions SET meta=? WHERE id=? AND meta=? AND deleted=0 AND expires_at>?", claimedSessionMeta(SessionChallengeClaim(expected)), id, encoded, time.Now())
		if err != nil {
			return err
		}
		count, err := result.RowsAffected()
		claimed = count == 1
		return err
	})
	return claimed, err
}

// Account and initial credential are one commit; a storage failure must not
// reserve a username for an account that cannot authenticate.
func (c *sqliteClient) CreateUserWithPasskey(ctx context.Context, id, username string, credential []byte) error {
	return c.immediate(ctx, func(conn *stateConn) error {
		return createUserWithPasskey(ctx, conn, id, username, credential)
	})
}

func createUserWithPasskey(ctx context.Context, conn *stateConn, id, username string, credential []byte) error {
	if err := ensureUniqueCredential(ctx, conn, credential); err != nil {
		return err
	}
	var taken bool
	if err := conn.QueryRowContext(ctx, "SELECT EXISTS(SELECT 1 FROM users WHERE lower(username)=lower(?))", username).Scan(&taken); err != nil {
		return err
	}
	if taken {
		return ErrUsernameNotAvailable
	}
	if err := (&models.User{ID: id, Username: username}).Insert(ctx, conn, boil.Infer()); err != nil {
		return err
	}
	return (&models.Passkey{UserID: id, Data: credential}).Insert(ctx, conn, boil.Infer())
}

func (c *sqliteClient) CreateUserWithPasskeySession(ctx context.Context, id, username string, data []byte, sourceID, claim string) (*models.Session, error) {
	var session *models.Session
	err := c.immediate(ctx, func(conn *stateConn) error {
		if err := requireSessionClaim(ctx, conn, sourceID, claim, nil); err != nil {
			return err
		}
		if err := createUserWithPasskey(ctx, conn, id, username, data); err != nil {
			return err
		}
		var err error
		session, err = rotateConsumedSession(ctx, conn, sourceID, id, claim)
		return err
	})
	return session, err
}

// AddUserPasskey enforces credential-ID uniqueness across every account in the
// same transaction as insertion. No identifying attestation is required.
func (c *sqliteClient) AddUserPasskey(ctx context.Context, userID string, data []byte) error {
	return c.immediate(ctx, func(conn *stateConn) error {
		if _, err := models.FindUser(ctx, conn, userID); err != nil {
			return err
		}
		if err := ensureUniqueCredential(ctx, conn, data); err != nil {
			return err
		}
		return (&models.Passkey{UserID: userID, Data: data}).Insert(ctx, conn, boil.Infer())
	})
}

func (c *sqliteClient) AddUserPasskeyForClaim(ctx context.Context, userID string, data []byte, sourceID, claim string) error {
	return c.immediate(ctx, func(conn *stateConn) error {
		if err := requireSessionClaim(ctx, conn, sourceID, claim, &userID); err != nil {
			return err
		}
		if _, err := models.FindUser(ctx, conn, userID); err != nil {
			return err
		}
		if err := ensureUniqueCredential(ctx, conn, data); err != nil {
			return err
		}
		return (&models.Passkey{UserID: userID, Data: data}).Insert(ctx, conn, boil.Infer())
	})
}

func (c *sqliteClient) DeleteUserPasskeyKeepingOne(ctx context.Context, userID, id string) error {
	return c.immediate(ctx, func(conn *stateConn) error {
		var owned bool
		if err := conn.QueryRowContext(ctx, "SELECT EXISTS(SELECT 1 FROM passkeys WHERE id=? AND user_id=?)", id, userID).Scan(&owned); err != nil {
			return err
		}
		if !owned {
			return sql.ErrNoRows
		}
		var count int
		if err := conn.QueryRowContext(ctx, "SELECT count(*) FROM passkeys WHERE user_id=?", userID).Scan(&count); err != nil {
			return err
		}
		if count <= 1 {
			return ErrLastPasskey
		}
		_, err := conn.ExecContext(ctx, "DELETE FROM passkeys WHERE id=? AND user_id=?", id, userID)
		return err
	})
}

type AuthenticationProof struct {
	SourceID, UserID, Claim, PasskeyID    string
	ExpectedCredential, UpdatedCredential []byte
}

// Signature validation produces a key snapshot. Persist it and rotate the exact
// consumed ceremony in one commit, losing to deletion, logout or a newer begin.
func (c *sqliteClient) AuthenticateSession(ctx context.Context, proof AuthenticationProof) (*models.Session, error) {
	var session *models.Session
	err := c.immediate(ctx, func(conn *stateConn) error {
		if err := requireSessionClaim(ctx, conn, proof.SourceID, proof.Claim, nil); err != nil {
			return err
		}
		if _, err := models.FindUser(ctx, conn, proof.UserID); err != nil {
			return err
		}
		result, err := conn.ExecContext(ctx, "UPDATE passkeys SET data=?,updated_at=? WHERE id=? AND user_id=? AND data=?", proof.UpdatedCredential, time.Now(), proof.PasskeyID, proof.UserID, proof.ExpectedCredential)
		if err != nil {
			return err
		}
		count, err := result.RowsAffected()
		if err != nil {
			return err
		}
		if count != 1 {
			return ErrCredentialChanged
		}
		session, err = rotateConsumedSession(ctx, conn, proof.SourceID, proof.UserID, proof.Claim)
		return err
	})
	return session, err
}

func rotateConsumedSession(ctx context.Context, conn *stateConn, sourceID, userID, claim string) (*models.Session, error) {
	id, err := ksuid.NewRandom()
	if err != nil {
		return nil, err
	}
	now := time.Now()
	result, err := conn.ExecContext(ctx, "UPDATE sessions SET deleted=1 WHERE id=? AND user_id IS NULL AND deleted=0 AND expires_at>? AND meta=?", sourceID, now, claimedSessionMeta(claim))
	if err != nil {
		return nil, err
	}
	count, err := result.RowsAffected()
	if err != nil {
		return nil, err
	}
	if count != 1 {
		return nil, sql.ErrNoRows
	}
	session := &models.Session{ID: id.String(), UserID: null.StringFrom(userID), ExpiresAt: now.Add(7 * 24 * time.Hour), LastUsed: now, Meta: []byte(`{}`)}
	if err := session.Insert(ctx, conn, boil.Infer()); err != nil {
		return nil, err
	}
	return session, nil
}
