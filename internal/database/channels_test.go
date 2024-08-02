package database

import (
	"context"
	"os"
	"testing"

	"github.com/cufee/feedlr-yt/internal/database/models"
	"github.com/matryer/is"
)

func TestInsertAndGetChannels(t *testing.T) {
	is := is.New(t)

	c, err := NewSQLiteClient(os.Getenv("DATABASE_PATH"))
	is.NoErr(err)

	c1 := models.Channel{
		ID:          "channel-1",
		Title:       "Channel 1 Title",
		Description: "Channel 1 Description",
	}
	err = c.UpsertChannel(context.Background(), &c1)
	is.NoErr(err)

	rc1, err := c.GetChannel(context.Background(), "channel-1")
	is.NoErr(err)
	is.True(rc1.ID == c1.ID)
	is.True(rc1.Title == c1.Title)
	is.True(rc1.Description == c1.Description)

	rc2, err := c.GetChannels(context.Background())
	is.NoErr(err)
	is.True(len(rc2) == 1)
	is.True(rc2[0].ID == c1.ID)
}
