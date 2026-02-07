package logic

import (
	"testing"

	"github.com/matryer/is"
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
	is.Equal(plan.ToAdd[0], "v1")
	is.Equal(plan.ToAdd[1], "v3")
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
