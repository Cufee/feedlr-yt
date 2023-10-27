package models

import "time"

// model Video {
//   id        String   @id @map("_id")
//   createdAt DateTime @default(now())
//   updatedAt DateTime @updatedAt

//   url         String
//   title       String
//   duration    Int     @default(0)
//   thumbnail   String?
//   description String  @db.String

//   views     VideoView[]
//   channel   Channel     @relation(fields: [channelId], references: [id], onDelete: Cascade)
//   channelId String

//   @@index([channelId], name: "channelId")
//   @@map("videos")
// }

const VideoCollection = "videos"

type Video struct {
	Model `bson:",inline"`

	ID          string    `json:"id" bson:"_id" field:"required"`
	URL         string    `json:"url" bson:"url" field:"required"`
	Title       string    `json:"title" bson:"title" field:"required"`
	Duration    int       `json:"duration" bson:"duration"`
	Thumbnail   string    `json:"thumbnail" bson:"thumbnail"`
	Description string    `json:"description" bson:"description"`
	PublishedAt time.Time `json:"publishedAt" bson:"publishedAt"`

	Views     []VideoView `json:"views" bson:"views,omitempty"`
	Channel   *Channel    `json:"channel" bson:"channel,omitempty"`
	ChannelId string      `json:"channelId" bson:"channelId" field:"required"`
}

type VideoOptions struct {
	Duration    *int
	Thumbnail   *string
	Description *string
}

func NewVideo(id, url, title, channelId string, publishedAt time.Time, opts ...VideoOptions) *Video {
	var duration int
	var thumbnail, description string
	if len(opts) > 0 {
		if opts[0].Duration != nil {
			duration = *opts[0].Duration
		}
		if opts[0].Thumbnail != nil {
			thumbnail = *opts[0].Thumbnail
		}
		if opts[0].Description != nil {
			description = *opts[0].Description
		}
	}

	return &Video{
		ID:          id,
		URL:         url,
		Title:       title,
		Duration:    duration,
		Thumbnail:   thumbnail,
		PublishedAt: publishedAt,
		Description: description,
		ChannelId:   channelId,
	}
}

func (v *Video) CollectionName() string {
	return VideoCollection
}
