package youtube

import (
	"context"
	"database/sql"
	"testing"
	"time"

	"github.com/cufee/feedlr-yt/internal/database/models"
	"github.com/matryer/is"
	"github.com/pkg/errors"
)

type testDatabaseStore struct {
	current *models.AppConfiguration
}

func (s *testDatabaseStore) GetConfiguration(ctx context.Context, id string) (*models.AppConfiguration, error) {
	if s.current == nil {
		return nil, sql.ErrNoRows
	}
	return s.current, nil
}
func (s *testDatabaseStore) UpdateConfiguration(ctx context.Context, config *models.AppConfiguration) error {
	s.current = config
	return nil
}
func (s *testDatabaseStore) UpsertConfiguration(ctx context.Context, config *models.AppConfiguration) (*models.AppConfiguration, error) {
	s.current = config
	return s.current, nil
}
func (s *testDatabaseStore) CreateConfiguration(ctx context.Context, key string, value []byte) (*models.AppConfiguration, error) {
	s.current = &models.AppConfiguration{
		ID:        "1",
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
		Version:   1,
		Data:      value,
	}
	return s.current, nil
}

func testAuthClient() (*OAuth2Client, error) {
	client := NewOAuthClient(&testDatabaseStore{})
	authed, err := client.Authenticate(context.Background())
	if err != nil {
		return nil, err
	}
	<-authed

	status := client.AuthStatus()
	if status != AuthStatusAuthenticated {
		return nil, errors.New("bad auth status")
	}
	return client, nil
}

func TestLoginFlow(t *testing.T) {
	is := is.New(t)

	client, err := testAuthClient()
	is.NoErr(err)

	err = client.RefreshToken(context.Background())
	is.NoErr(err)
}
