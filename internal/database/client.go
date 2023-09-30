package database

import (
	"log"

	"github.com/byvko-dev/youtube-app/prisma/db"
)

type Client struct {
	p *db.PrismaClient
}

var C *Client

func init() {
	client := db.NewClient()
	if err := client.Prisma.Connect(); err != nil {
		log.Fatalf("unable to connect to database: %v", err)
	}

	C = &Client{
		p: client,
	}
}
