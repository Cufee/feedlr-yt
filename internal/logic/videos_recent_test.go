package logic

import (
	"context"
	"testing"

	"github.com/cufee/feedlr-yt/internal/database"
	"github.com/cufee/feedlr-yt/internal/database/models"
)

type recentVideosMockDB struct {
	recentViews      []*models.View
	findVideosResult []*models.Video
	findVideosCalled bool
}

func (m *recentVideosMockDB) GetRecentUserViews(_ context.Context, _ string, _ int) ([]*models.View, error) {
	return m.recentViews, nil
}

func (m *recentVideosMockDB) FindVideos(_ context.Context, _ ...database.VideoQuery) ([]*models.Video, error) {
	m.findVideosCalled = true
	return m.findVideosResult, nil
}

func (*recentVideosMockDB) GetUserViews(context.Context, string, ...string) ([]*models.View, error) {
	return nil, nil
}

func (*recentVideosMockDB) UpsertView(context.Context, *models.View) error {
	return nil
}

func (*recentVideosMockDB) GetVideoByID(context.Context, string, ...database.VideoQuery) (*models.Video, error) {
	return nil, nil
}

func (*recentVideosMockDB) UpsertVideos(context.Context, ...*models.Video) error {
	return nil
}

func (*recentVideosMockDB) TouchVideoUpdatedAt(context.Context, string) error {
	return nil
}

func TestGetRecentVideosProps_EmptyViewsReturnsEmptyFeed(t *testing.T) {
	db := &recentVideosMockDB{
		recentViews: []*models.View{},
		findVideosResult: []*models.Video{
			{ID: "video-from-other-account"},
		},
	}

	feed, err := GetRecentVideosProps(context.Background(), db, "user-a")
	if err != nil {
		t.Fatalf("GetRecentVideosProps returned error: %v", err)
	}
	if db.findVideosCalled {
		t.Fatal("expected no video lookup when user has no recent views")
	}
	if len(feed) != 0 {
		t.Fatalf("expected empty feed, got %d item(s)", len(feed))
	}
}

