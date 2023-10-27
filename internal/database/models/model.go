package models

import (
	"errors"
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

// DefaultModel struct contains a model's default fields.
type Model struct {
	ID primitive.ObjectID `json:"id" bson:"_id,omitempty"`

	CreatedAt time.Time `json:"createdAt" bson:"createdAt"`
	UpdatedAt time.Time `json:"updatedAt" bson:"updatedAt"`
}

func (model *Model) Prepare() {
	if model.CreatedAt.IsZero() {
		model.CreatedAt = time.Now()
	}
	model.UpdatedAt = time.Now()
}

func (model *Model) ParseID(id interface{}) error {
	switch id := id.(type) {
	case primitive.ObjectID:
		model.ID = id
	default:
		return errors.New("invalid id type")
	}
	return nil
}
