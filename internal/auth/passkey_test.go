package auth

import (
	"context"
	"database/sql"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/cufee/feedlr-yt/internal/database"
	"github.com/cufee/feedlr-yt/internal/database/models"
)

func TestLegacyPasskeyLoginUpdatesPersist(t *testing.T) {
	path := filepath.Join(t.TempDir(), "passkeys.db")
	raw, err := sql.Open("sqlite3", path)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = raw.Close() })
	migrations, err := filepath.Glob("../database/migrations/*.sql")
	if err != nil || len(migrations) == 0 {
		t.Fatalf("find migrations: %v", err)
	}
	for _, path := range migrations {
		migration, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := raw.Exec(string(migration)); err != nil {
			t.Fatalf("apply %s: %v", path, err)
		}
	}
	db, err := database.NewSQLiteClient(path)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	ctx := context.Background()
	if _, err := db.CreateUser(ctx, "test-user", "test-user"); err != nil {
		t.Fatal(err)
	}
	// The old library stored the attestation format in attestationType.
	legacy := []byte(`{"id":"AQID","publicKey":"BAUG","attestationType":"packed","flags":{"userPresent":true,"userVerified":true,"backupEligible":true,"backupState":false},"authenticator":{"signCount":1}}`)
	key := &models.Passkey{UserID: "test-user", Label: "existing key", Data: legacy}
	if err := db.SaveUserPasskey(ctx, key); err != nil {
		t.Fatal(err)
	}
	store := NewStore(db)
	user, err := store.GetUser(ctx, "test-user")
	if err != nil {
		t.Fatal(err)
	}
	credential := user.WebAuthnCredentials()[0]
	if credential.AttestationFormat != "packed" || credential.AttestationType != "" {
		t.Fatalf("legacy attestation was not migrated: %+v", credential)
	}
	credential.Authenticator.SignCount = 2
	credential.Flags.BackupState = true
	if err := user.UpdateCredential(credential); err != nil {
		t.Fatal(err)
	}
	for range 2 {
		if err := store.SaveUser(ctx, &user); err != nil {
			t.Fatal(err)
		}
	}
	keys, err := db.GetUserPasskeys(ctx, user.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(keys) != 1 || keys[0].ID != key.ID || keys[0].Label != key.Label {
		t.Fatalf("credential update replaced or duplicated the existing key: %+v", keys)
	}
	if err := json.Unmarshal(keys[0].Data, &credential); err != nil {
		t.Fatal(err)
	}
	if credential.Authenticator.SignCount != 2 || !credential.Flags.BackupState || credential.AttestationFormat != "packed" {
		t.Fatalf("credential state was not persisted: %+v", credential)
	}
}
