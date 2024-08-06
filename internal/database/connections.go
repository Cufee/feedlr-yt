package database

import (
	"context"

	"github.com/cufee/feedlr-yt/internal/database/models"
	"github.com/pkg/errors"
	"github.com/volatiletech/sqlboiler/v4/boil"
	"github.com/volatiletech/sqlboiler/v4/queries/qm"
)

type ConnectionsClient interface {
	CreateConnection(ctx context.Context, userID string, connectionID string, kind connectionType) (*models.Connection, error)
	FindUserConnections(ctx context.Context, userID string, o ...ConnectionQuery) ([]*models.Connection, error)
	GetUserFromConnection(ctx context.Context, connectionID string) (*models.User, error)
	UpdateConnection(ctx context.Context, connection *models.Connection) error
	DeleteConnection(ctx context.Context, connectionID string) error
}

type connectionType string

func (t connectionType) String() string {
	return string(t)
}

const (
	ConnectionTypeGoogle  connectionType = "google"
	ConnectionTypeDiscord connectionType = "discord"
)

type connectionQuery struct {
	withUser bool
	types    []string
}

type ConnectionQuery func(*connectionQuery)

type connectionQuerySlice []ConnectionQuery

func (s connectionQuerySlice) opts() connectionQuery {
	var o connectionQuery
	for _, apply := range s {
		apply(&o)
	}
	return o
}

type Connection struct{}

func (Connection) WithUser() ConnectionQuery {
	return func(o *connectionQuery) {
		o.withUser = true
	}
}
func (Connection) Type(types ...connectionType) ConnectionQuery {
	return func(o *connectionQuery) {
		for _, t := range types {
			o.types = append(o.types, t.String())
		}
	}
}

func (c *sqliteClient) CreateConnection(ctx context.Context, userID string, connectionID string, kind connectionType) (*models.Connection, error) {
	user, err := models.FindUser(ctx, c.db, userID)
	if err != nil {
		return nil, errors.Wrap(err, "failed to find user")
	}

	connection := models.Connection{
		ID:     connectionID,
		Type:   kind.String(),
		UserID: userID,
	}

	err = connection.Insert(ctx, c.db, boil.Infer())
	if err != nil {
		return nil, errors.Wrap(err, "failed to insert connection")
	}

	err = connection.SetUser(ctx, c.db, false, user)
	if err != nil {
		return nil, errors.Wrap(err, "failed to set user")
	}

	return &connection, nil
}

func (c *sqliteClient) FindUserConnections(ctx context.Context, userID string, o ...ConnectionQuery) ([]*models.Connection, error) {
	opts := connectionQuerySlice(o).opts()

	mods := []qm.QueryMod{models.ConnectionWhere.UserID.EQ(userID)}
	if opts.types != nil {
		mods = append(mods, models.ConnectionWhere.Type.IN(opts.types))
	}

	var err error
	var connections []*models.Connection
	connections, err = models.Connections(mods...).All(ctx, c.db)
	if err != nil {
		return nil, err
	}

	if opts.withUser {
		err := models.Connection{}.L.LoadUser(ctx, c.db, false, &connections, nil)
		if err != nil {
			return nil, errors.Wrap(err, "failed to load user")
		}
	}

	return connections, nil
}

func (c *sqliteClient) GetUserFromConnection(ctx context.Context, connectionID string) (*models.User, error) {
	connection, err := models.FindConnection(ctx, c.db, connectionID, models.ConnectionColumns.UserID)
	if err != nil {
		return nil, err
	}

	user, err := models.FindUser(ctx, c.db, connection.UserID)
	if err != nil {
		return nil, err
	}

	return user, nil
}

func (c *sqliteClient) UpdateConnection(ctx context.Context, connection *models.Connection) error {
	_, err := connection.Update(ctx, c.db, boil.Infer())
	if err != nil {
		return err
	}
	return nil
}

func (c *sqliteClient) DeleteConnection(ctx context.Context, connectionID string) error {
	_, err := models.Connections(models.ConnectionWhere.ID.EQ(connectionID)).DeleteAll(ctx, c.db)
	if err != nil {
		return err
	}
	return nil
}
