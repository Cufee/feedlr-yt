package database

import (
	"log"

	"github.com/byvko-dev/youtube-app/prisma/db"
)

var Client *db.PrismaClient

func init() {
	Client = db.NewClient()
	if err := Client.Prisma.Connect(); err != nil {
		log.Fatalf("unable to connect to database: %v", err)
	}
}
