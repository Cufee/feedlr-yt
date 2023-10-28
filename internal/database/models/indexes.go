package models

import (
	"log"

	"go.mongodb.org/mongo-driver/mongo"
)

var indexHandlers = []func(collection *mongo.Database) error{}

func addIndexHandler(collection string, handler func(db *mongo.Collection) error) {
	indexHandlers = append(indexHandlers, func(db *mongo.Database) error {
		err := handler(db.Collection(collection))
		if err != nil {
			log.Printf("Failed to create index for %s: %s", collection, err)
		}
		return err
	})
}

func SyncIndexes(db *mongo.Database) error {
	log.Print("Syncing indexes...")
	defer log.Print("Done syncing indexes")

	for _, handler := range indexHandlers {
		err := handler(db)
		if err != nil {
			return err
		}
	}
	return nil
}
