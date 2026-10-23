package webapi

import (
	"context"
	"sort"
	"strings"

	"github.com/cufee/feedlr-yt/internal/api/podcastindex"
	"github.com/cufee/feedlr-yt/internal/api/rss"
	"github.com/cufee/feedlr-yt/internal/api/youtube"
	"github.com/cufee/feedlr-yt/internal/database"
	"github.com/cufee/feedlr-yt/internal/database/models"
	"github.com/cufee/feedlr-yt/internal/logic"
	"github.com/cufee/feedlr-yt/internal/sessions"
	"github.com/cufee/feedlr-yt/internal/types"
	"github.com/gofiber/fiber/v3"
)

func (a *API) session(c fiber.Ctx) error {
	r := Session{Capabilities: Capabilities{PodcastSearch: podcastindex.DefaultClient != nil, YouTubeSync: logic.DefaultYouTubeSync != nil, TVSync: logic.DefaultYouTubeTVSync != nil}}
	if id := viewer(c); id != "" {
		u, err := a.db.GetUser(c.Context(), id)
		if err != nil {
			return err
		}
		r.Authenticated = true
		r.UserID = id
		r.Username = u.Username
		if s, ok := c.Locals("webapi_session").(sessions.Session); ok {
			if err := s.Refresh(c); err != nil {
				return err
			}
		}
	}
	return c.JSON(r)
}

func source(c fiber.Ctx, fallback types.MediaSource) (types.MediaSource, error) {
	s, ok := types.ParseMediaSource(c.Query("source", string(fallback)))
	if !ok {
		return s, invalid("source", "Choose all, youtube or podcasts")
	}
	return s, nil
}

func (a *API) enrich(ctx context.Context, props []types.VideoProps, userID string) ([]MediaItem, error) {
	items := mediaDTOs(props)
	if userID == "" {
		return items, nil
	}
	pinned, err := logic.GetWatchLaterVideoIDs(ctx, a.db, userID)
	if err != nil {
		return nil, err
	}
	subs, err := logic.GetUserSubscribedChannels(ctx, a.db, userID)
	if err != nil {
		return nil, err
	}
	byID := make(map[string]types.ChannelProps, len(subs))
	for _, sub := range subs {
		byID[sub.ID] = sub
	}
	for i := range items {
		items[i].InWatchLater = pinned[items[i].ID]
		if sub, ok := byID[items[i].Channel.ID]; ok {
			items[i].Channel = channelDTO(sub, true)
		}
		if items[i].Kind == "podcast" {
			items[i].Channel.IsPodcast = true
			items[i].Channel.Source = "podcasts"
		}
	}
	return items, nil
}

func (a *API) feed(c fiber.Ctx) error {
	p, err := logic.GetUserVideosProps(c.Context(), a.db, viewer(c))
	if err != nil {
		return err
	}
	watch, _, err := logic.GetWatchLaterVideos(c.Context(), a.db, viewer(c), 12, 0)
	if err != nil {
		return err
	}
	n, err := a.enrich(c.Context(), trimFeedSection(p.New), viewer(c))
	if err != nil {
		return err
	}
	w, err := a.enrich(c.Context(), trimFeedSection(p.Watched), viewer(c))
	if err != nil {
		return err
	}
	later, err := a.enrich(c.Context(), watch, viewer(c))
	if err != nil {
		return err
	}
	return c.JSON(Feed{New: n, Watched: w, WatchLater: later, Onboarding: len(n)+len(w) == 0})
}

func trimFeedSection(videos []types.VideoProps) []types.VideoProps {
	// Twelve cards fill complete rows at every responsive grid width: 1, 2, 3 or 4 columns.
	const gridBatchSize = 12
	if len(videos) > gridBatchSize {
		return videos[:len(videos)-len(videos)%gridBatchSize]
	}
	return videos
}

func (a *API) history(c fiber.Ctx) error {
	s, err := source(c, types.MediaSourceAll)
	if err != nil {
		return err
	}
	limit, offset, err := pagination(c)
	if err != nil {
		return err
	}
	var views []*models.View
	switch s {
	case types.MediaSourceYouTube:
		views, err = a.db.GetRecentUserViewsForVideoType(c.Context(), viewer(c), string(youtube.VideoTypePodcastEpisode), true, limit+offset+1)
	case types.MediaSourcePodcasts:
		views, err = a.db.GetRecentUserViewsForVideoType(c.Context(), viewer(c), string(youtube.VideoTypePodcastEpisode), false, limit+offset+1)
	default:
		views, err = a.db.GetRecentUserViews(c.Context(), viewer(c), limit+offset+1)
	}
	if err != nil {
		return err
	}
	hasMore := len(views) > offset+limit
	if offset >= len(views) {
		views = nil
	} else {
		views = views[offset:min(len(views), offset+limit)]
	}
	props := make([]types.VideoProps, 0, len(views))
	for _, v := range views {
		p, err := a.db.GetVideoByID(c.Context(), v.VideoID, database.Video.WithChannel())
		if err != nil {
			return err
		}
		vp := types.VideoModelToProps(p, types.ChannelModelToProps(p.R.Channel))
		vp.Progress = int(v.Progress)
		vp.Hidden = v.Hidden.Bool
		props = append(props, vp)
	}
	items, err := a.enrich(c.Context(), props, viewer(c))
	if err != nil {
		return err
	}
	return c.JSON(MediaList{Items: items, Page: Page{Limit: limit, Offset: offset, HasMore: hasMore}})
}

func (a *API) watchLater(c fiber.Ctx) error {
	limit, offset, err := pagination(c)
	if err != nil {
		return err
	}
	p, more, err := logic.GetWatchLaterVideos(c.Context(), a.db, viewer(c), limit, offset)
	if err != nil {
		return err
	}
	items, err := a.enrich(c.Context(), p, viewer(c))
	if err != nil {
		return err
	}
	return c.JSON(MediaList{Items: items, Page: Page{Limit: limit, Offset: offset, HasMore: more}})
}

func (a *API) media(c fiber.Ctx) error {
	id := c.Params("id")
	if _, err := a.db.GetVideoByID(c.Context(), id); err != nil {
		if !database.IsErrNotFound(err) {
			return err
		}
		if youtube.DefaultClient == nil || !youtubeID.MatchString(id) {
			return err
		}
		if err = logic.EnsureVideoCached(c.Context(), a.db, id); err != nil {
			return fail(502, "media_unavailable", "This video could not be loaded")
		}
	}
	p, err := logic.GetPlayerPropsWithOpts(c.Context(), a.db, viewer(c), id, logic.GetPlayerOptions{WithProgress: viewer(c) != "", WithSegments: true})
	if err != nil {
		return err
	}
	items, err := a.enrich(c.Context(), []types.VideoProps{p.Video}, viewer(c))
	if err != nil {
		return err
	}
	r := MediaDetail{Item: items[0], Authenticated: viewer(c) != "", ReportProgress: viewer(c) != "", Volume: 100, SkipSegments: p.SkipSegments, PodcastSegments: p.PodcastSegments, Playlists: []Playlist{}, MembershipIDs: []string{}}
	if r.SkipSegments == nil {
		r.SkipSegments = []types.SegmentProps{}
	}
	if r.PodcastSegments.Status == "" {
		r.PodcastSegments.Status = "idle"
	}
	if r.PodcastSegments.Segments == nil {
		r.PodcastSegments.Segments = []types.PodcastSegmentProps{}
	}
	if viewer(c) != "" {
		settings, err := logic.GetUserSettings(c.Context(), a.db, viewer(c))
		if err != nil {
			return err
		}
		r.Volume = settings.PlayerVolume
		playlists, err := logic.GetUserPlaylistsProps(c.Context(), a.db, viewer(c))
		if err != nil {
			return err
		}
		r.Playlists = playlistDTOs(playlists)
		membership, err := logic.GetVideoPlaylistMembership(c.Context(), a.db, viewer(c), id)
		if err != nil {
			return err
		}
		for id, present := range membership {
			if present {
				r.MembershipIDs = append(r.MembershipIDs, id)
			}
		}
		sort.Strings(r.MembershipIDs)
	}
	return c.JSON(r)
}

func (a *API) subscriptions(c fiber.Ctx) error {
	s, err := source(c, types.MediaSourceAll)
	if err != nil {
		return err
	}
	props, err := logic.GetUserSubscribedChannels(c.Context(), a.db, viewer(c))
	if err != nil {
		return err
	}
	items := make([]Channel, 0, len(props))
	for _, p := range props {
		if s != types.MediaSourceAll && p.MediaSource() != s {
			continue
		}
		d := channelDTO(p, true)
		if p.IsPodcast {
			show, err := a.db.GetPodcastShow(c.Context(), p.ID)
			if err != nil {
				return err
			}
			d.FeedURL = show.FeedURL
		}
		items = append(items, d)
	}
	return c.JSON(fiber.Map{"items": items})
}

func (a *API) search(c fiber.Ctx) error {
	s, err := source(c, types.MediaSourceYouTube)
	if err != nil {
		return err
	}
	if s == types.MediaSourceAll {
		return invalid("source", "Choose youtube or podcasts")
	}
	q := strings.TrimSpace(c.Query("q"))
	maxLength := 100
	if s == types.MediaSourceYouTube {
		maxLength = 32
	}
	if len(q) < 3 || len(q) > maxLength {
		return invalid("q", "Enter a search query between 3 and "+fmtInt(maxLength)+" characters")
	}
	items := []Channel{}
	if s == types.MediaSourcePodcasts {
		if podcastindex.DefaultClient == nil {
			return fail(503, "catalog_unavailable", "Podcast search is unavailable. Add an RSS feed URL instead.")
		}
		results, err := logic.SearchPodcasts(c.Context(), q, 12)
		if err != nil {
			return fail(502, "search_failed", "The podcast catalog could not be searched")
		}
		subscribed, err := logic.GetUserSubscribedChannels(c.Context(), a.db, viewer(c))
		if err != nil {
			return err
		}
		byID := map[string]types.ChannelProps{}
		byFeed := map[string]types.ChannelProps{}
		for _, sub := range subscribed {
			if !sub.IsPodcast {
				continue
			}
			byID[sub.ID] = sub
			show, err := a.db.GetPodcastShow(c.Context(), sub.ID)
			if err != nil {
				return err
			}
			byFeed[show.FeedURL] = sub
		}
		for _, p := range results {
			d := Channel{ID: rss.ShowID(p.FeedURL), Title: p.Title, Description: p.Description, ArtworkURL: p.ArtworkURL, FeedURL: p.FeedURL, IsPodcast: true, Source: "podcasts", VideoFilter: "all"}
			if aliases, ok := a.db.(database.PodcastFeedAliasesClient); ok {
				if id, err := aliases.GetPodcastFeedAlias(c.Context(), p.FeedURL); err == nil {
					d.ID = id
				} else if !database.IsErrNotFound(err) {
					return err
				}
			}
			sub, ok := byID[d.ID]
			if !ok {
				sub, ok = byFeed[p.FeedURL]
			}
			if ok {
				d.ID = sub.ID
				d.Subscribed = true
				d.VideoFilter = string(sub.VideoFilter)
				if d.VideoFilter == "" {
					d.VideoFilter = "all"
				}
			}
			items = append(items, d)
		}
	} else {
		if youtube.DefaultClient == nil {
			return fail(503, "youtube_unavailable", "YouTube search is unavailable")
		}
		results, err := logic.SearchChannels(c.Context(), a.db, viewer(c), q, 12)
		if err != nil {
			return fail(502, "search_failed", "YouTube could not be searched")
		}
		for _, p := range results {
			d := channelDTO(types.ChannelProps{Channel: p.Channel}, p.Subscribed)
			d.ArtworkURL = p.Thumbnail
			items = append(items, d)
		}
	}
	return c.JSON(fiber.Map{"items": items})
}

func (a *API) channel(c fiber.Ctx) error {
	id := c.Params("id")
	if youtube.DefaultClient == nil {
		if _, err := a.db.GetChannel(c.Context(), id); err != nil {
			return err
		}
	}
	p, err := logic.GetChannelPageProps(c.Context(), a.db, viewer(c), id)
	if err != nil {
		return err
	}
	d := channelDTO(p.Channel.ChannelProps, p.Subscribed)
	if d.IsPodcast {
		show, err := a.db.GetPodcastShow(c.Context(), id)
		if err != nil {
			return err
		}
		d.FeedURL = show.FeedURL
	}
	items, err := a.enrich(c.Context(), p.Channel.Videos, viewer(c))
	if err != nil {
		return err
	}
	return c.JSON(fiber.Map{"channel": d, "items": items, "subscribed": p.Subscribed, "filter": p.VideoFilter})
}

func (a *API) playlists(c fiber.Ctx) error {
	p, err := logic.GetUserPlaylistsProps(c.Context(), a.db, viewer(c))
	if err != nil {
		return err
	}
	return c.JSON(fiber.Map{"items": playlistDTOs(p)})
}

func (a *API) ownedPlaylist(c fiber.Ctx, editable bool) (*models.Playlist, error) {
	p, err := a.db.GetPlaylistByID(c.Context(), c.Params("id"))
	if err != nil {
		return nil, err
	}
	if p.UserID != viewer(c) {
		return nil, fail(404, "not_found", "This playlist is unavailable")
	}
	if editable && p.System {
		return nil, fail(403, "system_playlist", "Use the dedicated Watch Later action for this playlist")
	}
	return p, nil
}

func (a *API) playlist(c fiber.Ctx) error {
	model, err := a.ownedPlaylist(c, false)
	if err != nil {
		return err
	}
	p, err := logic.GetPlaylistPageProps(c.Context(), a.db, viewer(c), model.ID)
	if err != nil {
		return err
	}
	d := playlistDTO(p.Playlist)
	d.System = model.System
	items, err := a.enrich(c.Context(), p.Videos, viewer(c))
	if err != nil {
		return err
	}
	return c.JSON(fiber.Map{"playlist": d, "items": items})
}
