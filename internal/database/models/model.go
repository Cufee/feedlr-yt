package models

import (
	"fmt"
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

// DefaultModel struct contains a model's default fields.
type Model struct {
	IDField    `bson:",inline"`
	DateFields `bson:",inline"`
}

// IDField struct contains a model's ID field.
type IDField struct {
	ID string `json:"id" bson:"_id,omitempty"`
}

// PrepareID method prepares the ID value to be used for filtering
// e.g convert hex-string ID value to bson.ObjectId
func (f *IDField) PrepareID(id interface{}) (interface{}, error) {
	if idStr, ok := id.(string); ok {
		return idStr, nil
	}

	// Otherwise id must be ObjectId
	return nil, fmt.Errorf("invalid id type")
}

// GetID method returns a model's ID
func (f *IDField) GetID() interface{} {
	return f.ID
}

// SetID sets the value of a model's ID field.
func (f *IDField) SetID(id interface{}) {
	if idIod, ok := id.(primitive.ObjectID); ok {
		f.ID = idIod.Hex()
		return
	}
	f.ID = id.(string)
}

// DateFields struct contains the `created_at` and `updated_at`
// fields that autofill when inserting or updating a model.
type DateFields struct {
	CreatedAt time.Time `json:"created_at" bson:"created_at"`
	UpdatedAt time.Time `json:"updated_at" bson:"updated_at"`
}

//--------------------------------
// DateField methods
//--------------------------------

// Creating hook is used here to set the `created_at` field
// value when inserting a new model into the database.
// TODO: get context as param the next version(4).
func (f *DateFields) Creating() error {
	f.CreatedAt = time.Now().UTC()
	return nil
}

// Saving hook is used here to set the `updated_at` field
// value when creating or updating a model.
// TODO: get context as param the next version(4).
func (f *DateFields) Saving() error {
	f.UpdatedAt = time.Now().UTC()
	return nil
}
