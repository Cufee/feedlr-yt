package database

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/aarondl/sqlboiler/v4/boil"
	"github.com/cufee/feedlr-yt/internal/database/models"
)

func TestTVSyncAccountListingCanReadCompleteWorkerSet(t *testing.T) {
	db, err := sql.Open("sqlite3", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	db.SetMaxOpenConns(1)
	paths, err := filepath.Glob("migrations/*.sql")
	if err != nil || len(paths) == 0 {
		t.Fatalf("migrations: %v", err)
	}
	for _, path := range paths {
		migration, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := db.Exec(string(migration)); err != nil {
			t.Fatalf("%s: %v", path, err)
		}
	}
	c := &sqliteClient{db: db}
	ctx := context.Background()
	for i := range 105 {
		id := fmt.Sprintf("user-%03d", i)
		user := &models.User{ID: id, Username: id}
		if err := user.Insert(ctx, db, boil.Infer()); err != nil {
			t.Fatal(err)
		}
		if err := c.UpsertYouTubeTVSyncCredentials(ctx, id, id, "TV", []byte("encrypted"), "hash"); err != nil {
			t.Fatal(err)
		}
	}
	for limit, want := range map[int]int{100: 100, 0: 105} {
		accounts, err := c.ListEnabledYouTubeTVSyncAccounts(ctx, limit)
		if err != nil || len(accounts) != want {
			t.Fatalf("limit=%d: got %d, want %d: %v", limit, len(accounts), want, err)
		}
	}
	if err := c.SetYouTubeTVSyncAccountEnabled(ctx, "user-000", false); err != nil {
		t.Fatal(err)
	}
	accounts, err := c.ListEnabledYouTubeTVSyncAccounts(ctx, 0)
	if err != nil || len(accounts) != 104 {
		t.Fatalf("disabled account retained: %d, %v", len(accounts), err)
	}
}
