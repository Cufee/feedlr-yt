package root

import (
	"fmt"
	"net/http"
	"strings"

	"github.com/a-h/templ"
	"github.com/cufee/feedlr-yt/internal/database"
	"github.com/cufee/feedlr-yt/internal/server/handler"
	"github.com/cufee/tpot/brewed"
)

const (
	thumbnailCacheControl    = "public, max-age=3600, s-maxage=604800, stale-while-revalidate=86400"
	thumbnailEdgeCacheHeader = "public, max-age=604800, stale-while-revalidate=86400"
)

func setThumbnailCacheHeaders(ctx *handler.Context) {
	// Keep browser caching modest while allowing CDN/Cloudflare to cache more aggressively.
	ctx.Set("Cache-Control", thumbnailCacheControl)
	ctx.Set("CDN-Cache-Control", thumbnailEdgeCacheHeader)
	ctx.Set("Cloudflare-CDN-Cache-Control", thumbnailEdgeCacheHeader)
}

func videoThumbnailFile(variant string) (string, bool) {
	switch strings.TrimSpace(variant) {
	case "0", "default":
		return "0.jpg", true
	case "sddefault":
		return "sddefault.jpg", true
	case "hqdefault":
		return "hqdefault.jpg", true
	case "maxresdefault":
		return "maxresdefault.jpg", true
	default:
		return "", false
	}
}

var VideoThumbnail brewed.Partial[*handler.Context] = func(ctx *handler.Context) (templ.Component, error) {
	videoID := strings.TrimSpace(ctx.Params("id"))
	if videoID == "" {
		return nil, ctx.SendStatus(http.StatusBadRequest)
	}

	file, ok := videoThumbnailFile(ctx.Params("variant"))
	if !ok {
		return nil, ctx.SendStatus(http.StatusBadRequest)
	}

	if _, err := ctx.Database().GetVideoByID(ctx.Context(), videoID); err != nil {
		if database.IsErrNotFound(err) {
			return nil, ctx.SendStatus(http.StatusNotFound)
		}
		return nil, ctx.SendStatus(http.StatusInternalServerError)
	}

	setThumbnailCacheHeaders(ctx)
	return nil, ctx.Redirect(fmt.Sprintf("https://i.ytimg.com/vi/%s/%s", videoID, file), http.StatusFound)
}

var ChannelThumbnail brewed.Partial[*handler.Context] = func(ctx *handler.Context) (templ.Component, error) {
	channelID := strings.TrimSpace(ctx.Params("id"))
	if channelID == "" {
		return nil, ctx.SendStatus(http.StatusBadRequest)
	}

	channel, err := ctx.Database().GetChannel(ctx.Context(), channelID)
	if err != nil {
		if database.IsErrNotFound(err) {
			return nil, ctx.SendStatus(http.StatusNotFound)
		}
		return nil, ctx.SendStatus(http.StatusInternalServerError)
	}

	thumbnailURL := strings.TrimSpace(channel.Thumbnail)
	if thumbnailURL == "" {
		return nil, ctx.SendStatus(http.StatusNotFound)
	}

	setThumbnailCacheHeaders(ctx)
	return nil, ctx.Redirect(thumbnailURL, http.StatusFound)
}
