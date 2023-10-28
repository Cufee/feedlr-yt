package models

import (
	"context"
	"log"
	"strings"

	"go.mongodb.org/mongo-driver/mongo"
	"golang.org/x/exp/slices"
)

var indexHandlers = make(map[string](func(collection *mongo.Database) ([]string, error)))

func addIndexHandler(collection string, handler func(db *mongo.Collection) ([]string, error)) {
	indexHandlers[collection] = (func(db *mongo.Database) ([]string, error) {
		names, err := handler(db.Collection(collection))
		if err != nil && strings.Contains(err.Error(), "Index already exists") {
			return names, nil
		}
		return names, err
	})
}

func SyncIndexes(db *mongo.Database) error {
	log.Print("Syncing indexes...")
	defer log.Print("Done syncing indexes")

	toDelete := make(map[string][]string)
	for collection, handler := range indexHandlers {
		names, err := handler(db)
		if err != nil {
			return err
		}
		log.Printf("Synced indexes for %s: %v", collection, names)

		current, err := db.Collection(collection).Indexes().ListSpecifications(context.Background())
		if err != nil {
			return err
		}

		for _, index := range current {
			if index.Name == "_id_" {
				continue
			}

			if !slices.Contains(names, index.Name) {
				toDelete[collection] = append(toDelete[collection], index.Name)
			}
		}
	}

	for collection, names := range toDelete {
		log.Printf("Deleting indexes for %s: %v", collection, names)
		for _, name := range names {
			_, err := db.Collection(collection).Indexes().DropOne(context.Background(), name)
			if err != nil {
				return err
			}
		}
	}

	return nil
}
