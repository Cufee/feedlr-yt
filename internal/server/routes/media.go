package root

import (
	"net/http"
	"strings"

	"github.com/a-h/templ"
	"github.com/cufee/feedlr-yt/internal/api/youtube"
	"github.com/cufee/feedlr-yt/internal/database"
	"github.com/cufee/feedlr-yt/internal/server/handler"
	"github.com/cufee/tpot/brewed"
)

// MediaStream redirects to the episode's media enclosure. Keeping the
// indirection through our own URL means enclosure rotation is transparent to
// clients and nothing upstream is exposed in markup.
var MediaStream brewed.Partial[*handler.Context] = func(ctx *handler.Context) (templ.Component, error) {
	mediaID := strings.TrimSpace(ctx.Params("id"))
	if mediaID == "" {
		return nil, ctx.SendStatus(http.StatusBadRequest)
	}

	video, err := ctx.Database().GetVideoByID(ctx.Context(), mediaID)
	if err != nil {
		if database.IsErrNotFound(err) {
			return nil, ctx.SendStatus(http.StatusNotFound)
		}
		return nil, ctx.SendStatus(http.StatusInternalServerError)
	}

	if youtube.VideoType(video.Type) != youtube.VideoTypePodcastEpisode || !video.MediaURL.Valid {
		return nil, ctx.SendStatus(http.StatusNotFound)
	}

	// Enclosure URLs can rotate; cache the hop briefly only.
	ctx.Set("Cache-Control", "public, max-age=300")
	return nil, ctx.Redirect(video.MediaURL.String, http.StatusFound)
}
