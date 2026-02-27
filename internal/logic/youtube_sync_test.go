package logic

import (
	"context"
	stdErrors "errors"
	"testing"

	"github.com/matryer/is"
	"golang.org/x/oauth2"
)

func TestYouTubeSyncCryptoRoundTrip(t *testing.T) {
	is := is.New(t)

	crypto := newYouTubeSyncCrypto("super-secret")
	aad := "user-1"
	plaintext := []byte("refresh-token")

	encrypted, err := crypto.Encrypt(plaintext, aad)
	is.NoErr(err)
	is.True(len(encrypted) > 0)

	decrypted, err := crypto.Decrypt(encrypted, aad)
	is.NoErr(err)
	is.Equal(string(decrypted), string(plaintext))
}

func TestBuildPlaylistSyncPlanSplitBudget(t *testing.T) {
	is := is.New(t)

	desired := []string{"v1", "v2", "v3", "v4"}
	remote := []playlistRemoteItem{
		{ItemID: "i1", VideoID: "v2", Position: 0},
		{ItemID: "i2", VideoID: "x1", Position: 1},
		{ItemID: "i3", VideoID: "x2", Position: 2},
	}

	plan := buildPlaylistSyncPlan(desired, remote, 4)
	is.Equal(len(plan.ToAdd), 2)    // split insert budget
	is.Equal(len(plan.ToDelete), 2) // split delete budget
	is.Equal(plan.ToAdd[0].VideoID, "v3")
	is.Equal(plan.ToAdd[1].VideoID, "v1")
}

func TestBuildPlaylistSyncPlanBudgetBorrowing(t *testing.T) {
	is := is.New(t)

	desired := []string{"a", "b", "c", "d", "e"}
	remote := []playlistRemoteItem{
		{ItemID: "i1", VideoID: "x", Position: 10},
	}

	plan := buildPlaylistSyncPlan(desired, remote, 4)
	// 2 insert + 2 delete split, but only 1 delete candidate; borrow one more call to inserts.
	is.Equal(len(plan.ToAdd), 3)
	is.Equal(len(plan.ToDelete), 1)
}

func TestBuildPlaylistSyncPlanInsertPositionsPreserveFeedOrder(t *testing.T) {
	is := is.New(t)

	desired := []string{"a", "b", "c", "d"}
	remote := []playlistRemoteItem{
		{ItemID: "i1", VideoID: "a", Position: 0},
		{ItemID: "i2", VideoID: "d", Position: 1},
	}

	plan := buildPlaylistSyncPlan(desired, remote, 4)
	is.Equal(len(plan.ToAdd), 2)
	is.Equal(plan.ToAdd[0].VideoID, "c")
	is.Equal(plan.ToAdd[0].Position, int64(1))
	is.Equal(plan.ToAdd[1].VideoID, "b")
	is.Equal(plan.ToAdd[1].Position, int64(1))
}

func TestBuildPlaylistSyncPlanInsertPositionsAppendAtEnd(t *testing.T) {
	is := is.New(t)

	desired := []string{"a", "b", "c", "d"}
	remote := []playlistRemoteItem{
		{ItemID: "i1", VideoID: "a", Position: 0},
		{ItemID: "i2", VideoID: "b", Position: 1},
	}

	plan := buildPlaylistSyncPlan(desired, remote, 4)
	is.Equal(len(plan.ToAdd), 2)
	is.Equal(plan.ToAdd[0].VideoID, "d")
	is.Equal(plan.ToAdd[0].Position, int64(2))
	is.Equal(plan.ToAdd[1].VideoID, "c")
	is.Equal(plan.ToAdd[1].Position, int64(2))
}

func TestBuildPlaylistSyncPlanInsertPositionsIgnoreItemsPlannedForDelete(t *testing.T) {
	is := is.New(t)

	desired := []string{"n1", "a", "b", "c"}
	remote := []playlistRemoteItem{
		{ItemID: "stale-head", VideoID: "x", Position: 0},
		{ItemID: "i-a", VideoID: "a", Position: 1},
		{ItemID: "i-b", VideoID: "b", Position: 2},
		{ItemID: "i-c", VideoID: "c", Position: 3},
		{ItemID: "stale-tail", VideoID: "y", Position: 4},
	}

	plan := buildPlaylistSyncPlan(desired, remote, 4)
	is.Equal(len(plan.ToDelete), 2)
	is.Equal(plan.ToDelete[0], "stale-tail")
	is.Equal(plan.ToDelete[1], "stale-head")

	is.Equal(len(plan.ToAdd), 1)
	is.Equal(plan.ToAdd[0].VideoID, "n1")
	is.Equal(plan.ToAdd[0].Position, int64(0))
}

type fixedTokenSource struct {
	token *oauth2.Token
	err   error
}

func (s *fixedTokenSource) Token() (*oauth2.Token, error) {
	if s.err != nil {
		return nil, s.err
	}
	return s.token, nil
}

func TestPersistingRefreshTokenSourceSkipsPersistWhenRefreshTokenUnchanged(t *testing.T) {
	is := is.New(t)

	calls := 0
	src := &persistingRefreshTokenSource{
		next:                &fixedTokenSource{token: &oauth2.Token{RefreshToken: "same"}},
		ctx:                 context.Background(),
		currentRefreshToken: "same",
		onRefreshToken: func(context.Context, string) error {
			calls++
			return nil
		},
	}

	_, err := src.Token()
	is.NoErr(err)
	is.Equal(calls, 0)
}

func TestPersistingRefreshTokenSourcePersistsOnRotation(t *testing.T) {
	is := is.New(t)

	var gotToken string
	src := &persistingRefreshTokenSource{
		next:                &fixedTokenSource{token: &oauth2.Token{RefreshToken: "new"}},
		ctx:                 context.Background(),
		currentRefreshToken: "old",
		onRefreshToken: func(_ context.Context, refreshToken string) error {
			gotToken = refreshToken
			return nil
		},
	}

	_, err := src.Token()
	is.NoErr(err)
	is.Equal(gotToken, "new")
	is.Equal(src.currentRefreshToken, "new")
}

func TestPersistingRefreshTokenSourceReturnsPersistError(t *testing.T) {
	is := is.New(t)

	persistErr := stdErrors.New("persist failed")
	src := &persistingRefreshTokenSource{
		next:                &fixedTokenSource{token: &oauth2.Token{RefreshToken: "new"}},
		ctx:                 context.Background(),
		currentRefreshToken: "old",
		onRefreshToken: func(context.Context, string) error {
			return persistErr
		},
	}

	_, err := src.Token()
	is.True(stdErrors.Is(err, persistErr))
}
