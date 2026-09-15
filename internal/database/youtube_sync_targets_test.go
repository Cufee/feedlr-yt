package database

import (
	"context"
	"database/sql"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/aarondl/sqlboiler/v4/boil"
	"github.com/cufee/feedlr-yt/internal/database/models"
)

func TestYouTubeSyncTargetsPersistenceAndDisconnect(t *testing.T) {
	db, err := sql.Open("sqlite3", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	db.SetMaxOpenConns(1)
	migrations, err := filepath.Glob("migrations/*.sql")
	if err != nil || len(migrations) == 0 {
		t.Fatalf("find migrations: %v", err)
	}
	for _, path := range migrations {
		migration, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := db.Exec(string(migration)); err != nil {
			t.Fatalf("%s: %v", path, err)
		}
	}
	if _, err := db.Exec("PRAGMA foreign_keys = ON"); err != nil {
		t.Fatal(err)
	}
	c := &sqliteClient{db: db}
	ctx := context.Background()
	for _, id := range []string{"one", "two"} {
		user := &models.User{ID: id, Username: id}
		if err := user.Insert(ctx, db, boil.Infer()); err != nil {
			t.Fatal(err)
		}
		if err := c.UpsertYouTubeSyncCredentials(ctx, id, []byte("encrypted"), "hash"); err != nil {
			t.Fatal(err)
		}
		account, err := c.GetYouTubeSyncAccountByUserID(ctx, id)
		if err != nil {
			t.Fatal(err)
		}
		state := &models.YoutubeSyncTarget{AccountID: account.ID, SourceID: "feed", PlaylistID: "remote-" + id, LastAttemptAt: time.Now().UTC()}
		if err := c.UpsertYouTubeSyncTarget(ctx, state); err != nil {
			t.Fatal(err)
		}
		state.Title = "Updated"
		if err := c.UpsertYouTubeSyncTarget(ctx, state); err != nil {
			t.Fatal(err)
		}
		states, err := c.ListYouTubeSyncTargets(ctx, account.ID)
		if err != nil {
			t.Fatal(err)
		}
		if len(states) != 1 || states[0].PlaylistID != "remote-"+id || states[0].Title != "Updated" || !states[0].LastAttemptAt.Equal(state.LastAttemptAt) {
			t.Fatalf("unexpected stored states: %+v", states)
		}
		if id == "one" {
			if err := c.DeleteYouTubeSyncAccount(ctx, id); err != nil {
				t.Fatal(err)
			}
			states, err = c.ListYouTubeSyncTargets(ctx, account.ID)
			if err != nil || len(states) != 0 {
				t.Fatalf("disconnect retained export mappings: %v, %v", states, err)
			}
		}
	}
}
