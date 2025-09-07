package database

import (
	"context"

	"github.com/aarondl/sqlboiler/v4/boil"
	"github.com/cufee/feedlr-yt/internal/database/models"
)

type SettingsClient interface {
	GetUserSettings(ctx context.Context, userID string) (*models.Setting, error)
	UpsertSettings(ctx context.Context, settings *models.Setting) error
}

func (c *sqliteClient) GetUserSettings(ctx context.Context, userID string) (*models.Setting, error) {
	settings, err := models.Settings(models.SettingWhere.UserID.EQ(userID)).One(ctx, c.db)
	if err != nil {
		return nil, err
	}
	return settings, nil
}

func (c *sqliteClient) UpsertSettings(ctx context.Context, settings *models.Setting) error {
	err := settings.Upsert(ctx, c.db, true, []string{models.SettingColumns.ID, models.SettingColumns.UserID}, boil.Infer(), boil.Infer())
	if err != nil {
		return err
	}
	return nil
}
