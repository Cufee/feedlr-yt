package mock

import (
	"context"
	"database/sql"
	"time"

	"github.com/cufee/feedlr-yt/internal/database/models"
)

type AuthStore struct {
	current *models.AppConfiguration
}

func (s *AuthStore) GetConfiguration(ctx context.Context, id string) (*models.AppConfiguration, error) {
	if s.current == nil {
		return nil, sql.ErrNoRows
	}
	return s.current, nil
}
func (s *AuthStore) UpdateConfiguration(ctx context.Context, config *models.AppConfiguration) error {
	s.current = config
	return nil
}
func (s *AuthStore) UpsertConfiguration(ctx context.Context, config *models.AppConfiguration) (*models.AppConfiguration, error) {
	s.current = config
	return s.current, nil
}
func (s *AuthStore) CreateConfiguration(ctx context.Context, key string, value []byte) (*models.AppConfiguration, error) {
	s.current = &models.AppConfiguration{
		ID:        "1",
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
		Version:   1,
		Data:      value,
	}
	return s.current, nil
}
