package database

import (
	"context"

	"github.com/aarondl/sqlboiler/v4/boil"
	"github.com/cufee/feedlr-yt/internal/database/models"
)

type ConfigurationClient interface {
	CreateConfiguration(ctx context.Context, key string, value []byte) (*models.AppConfiguration, error)
	GetConfiguration(ctx context.Context, id string) (*models.AppConfiguration, error)
	UpdateConfiguration(ctx context.Context, config *models.AppConfiguration) error
	UpsertConfiguration(ctx context.Context, config *models.AppConfiguration) (*models.AppConfiguration, error)
}

func (c *sqliteClient) GetConfiguration(ctx context.Context, key string) (*models.AppConfiguration, error) {
	config, err := models.AppConfigurations(models.AppConfigurationWhere.ID.EQ(key)).One(ctx, c.db)
	if err != nil {
		return nil, err
	}
	return config, nil
}

func (c *sqliteClient) UpdateConfiguration(ctx context.Context, config *models.AppConfiguration) error {
	_, err := config.Update(ctx, c.db, boil.Whitelist(models.AppConfigurationColumns.Data, models.AppConfigurationColumns.Version))
	if err != nil {
		return err
	}
	return nil
}

func (c *sqliteClient) UpsertConfiguration(ctx context.Context, config *models.AppConfiguration) (*models.AppConfiguration, error) {
	err := config.Upsert(ctx, c.db, true, []string{models.AppConfigurationColumns.ID}, boil.Whitelist(models.AppConfigurationColumns.Data, models.AppConfigurationColumns.Version), boil.Infer())
	if err != nil {
		return nil, err
	}
	return config, nil
}

func (c *sqliteClient) CreateConfiguration(ctx context.Context, key string, value []byte) (*models.AppConfiguration, error) {
	data := &models.AppConfiguration{
		ID:      key,
		Data:    value,
		Version: 1,
	}
	err := data.Insert(ctx, c.db, boil.Infer())
	if err != nil {
		return nil, err
	}
	return data, nil
}
