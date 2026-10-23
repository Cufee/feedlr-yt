package webapi

import (
	"context"
	"errors"
	"net/url"
	"os"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/cufee/feedlr-yt/internal/api/youtube"
	"github.com/cufee/feedlr-yt/internal/database"
	"github.com/cufee/feedlr-yt/internal/logic"
	"github.com/cufee/feedlr-yt/internal/sessions"
	"github.com/cufee/feedlr-yt/internal/types"
	"github.com/gofiber/fiber/v3"
)

var youtubeID = regexp.MustCompile(`^[A-Za-z0-9_-]{11}$`)

func fmtInt(n int) string { return strconv.Itoa(n) }

func (a *API) logout(c fiber.Ctx) error {
	if s, ok := c.Locals("webapi_session").(sessions.Session); ok && a.sessions != nil {
		if err := a.sessions.Delete(c.Context(), s.ID()); err != nil && !database.IsErrNotFound(err) {
			return err
		}
	}
	host := os.Getenv("COOKIE_DOMAIN")
	c.Cookie(&fiber.Cookie{Name: "session_id", Value: "", Path: "/", Domain: strings.Split(host, ":")[0], HTTPOnly: true, Secure: !strings.Contains(host, "localhost"), SameSite: "lax", Expires: time.Unix(1, 0), MaxAge: -1})
	return c.SendStatus(204)
}

func (a *API) ensureMedia(ctx context.Context, id string) error {
	if _, err := a.db.GetVideoByID(ctx, id); err == nil {
		return nil
	} else if !database.IsErrNotFound(err) {
		return err
	}
	if !youtubeID.MatchString(id) {
		return fail(404, "not_found", "This media item is unavailable")
	}
	if youtube.DefaultClient == nil {
		return fail(503, "youtube_unavailable", "YouTube is unavailable")
	}
	if err := logic.EnsureVideoCached(ctx, a.db, id); err != nil {
		return fail(502, "media_unavailable", "This video could not be loaded")
	}
	return nil
}

func (a *API) setWatchLater(c fiber.Ctx) error {
	var req struct {
		Present *bool `json:"present"`
	}
	if err := body(c, &req); err != nil {
		return err
	}
	if req.Present == nil {
		return invalid("present", "Choose whether this item should be saved")
	}
	if *req.Present {
		if err := a.ensureMedia(c.Context(), c.Params("id")); err != nil {
			return err
		}
	}
	affected, err := a.db.SetWatchLaterMembership(c.Context(), viewer(c), c.Params("id"), *req.Present)
	if err != nil {
		return err
	}
	return c.JSON(fiber.Map{"mediaId": c.Params("id"), "present": *req.Present, "affectedIds": affected})
}

func (a *API) patchMedia(c fiber.Ctx) error {
	var req struct {
		Progress  *int  `json:"progressSeconds"`
		Hidden    *bool `json:"hidden"`
		Completed *bool `json:"completed"`
		Volume    *int  `json:"volume"`
	}
	if err := body(c, &req); err != nil {
		return err
	}
	if req.Progress == nil && req.Hidden == nil && req.Completed == nil && req.Volume == nil {
		return invalid("state", "Submit a progress, visibility, completion or volume change")
	}
	if req.Progress != nil && (*req.Progress < 0 || *req.Progress > 315360000) {
		return invalid("progressSeconds", "Use a nonnegative position in seconds")
	}
	if req.Volume != nil && (*req.Volume < 0 || *req.Volume > 100) {
		return invalid("volume", "Use a volume from 0 to 100")
	}
	if req.Completed != nil && req.Progress != nil {
		return invalid("completed", "Submit completion or progress, rather than both")
	}
	if err := a.ensureMedia(c.Context(), c.Params("id")); err != nil {
		return err
	}
	if req.Completed != nil {
		progress := 0
		if *req.Completed {
			v, err := a.db.GetVideoByID(c.Context(), c.Params("id"))
			if err != nil {
				return err
			}
			if v.Duration <= 0 {
				return fail(409, "duration_unknown", "This item's duration is not available yet")
			}
			progress = int(v.Duration)
		}
		req.Progress = &progress
	}
	if req.Progress != nil || req.Hidden != nil {
		if _, err := a.db.PatchViewState(c.Context(), viewer(c), c.Params("id"), database.ViewStatePatch{Progress: req.Progress, Hidden: req.Hidden}); err != nil {
			return err
		}
	}
	if req.Volume != nil {
		if _, err := logic.GetUserSettings(c.Context(), a.db, viewer(c)); err != nil {
			return err
		}
		if err := a.db.PatchUserSettings(c.Context(), viewer(c), database.SettingsPatch{Volume: req.Volume}); err != nil {
			return err
		}
	}
	return c.SendStatus(204)
}

func (a *API) subscribe(c fiber.Ctx) error {
	var req struct {
		ChannelID string `json:"channelId"`
		FeedURL   string `json:"feedUrl"`
	}
	if err := body(c, &req); err != nil {
		return err
	}
	if (req.ChannelID == "") == (req.FeedURL == "") {
		return invalid("channelId", "Submit a channel ID or an RSS feed URL")
	}
	if req.FeedURL != "" {
		if _, err := logic.NewPodcastSubscription(c.Context(), a.db, viewer(c), strings.TrimSpace(req.FeedURL)); err != nil {
			if errors.Is(err, logic.ErrInvalidFeedURL) {
				return invalid("feedUrl", "Enter an HTTP or HTTPS podcast RSS feed URL")
			}
			return fail(502, "feed_unavailable", "The podcast feed could not be loaded")
		}
	} else {
		exists, err := logic.SubscriptionExists(c.Context(), a.db, viewer(c), req.ChannelID)
		if err != nil {
			return err
		}
		if exists {
			return c.SendStatus(204)
		}
		if youtube.DefaultClient == nil {
			if _, err = a.db.GetPodcastShow(c.Context(), req.ChannelID); err != nil {
				return fail(503, "youtube_unavailable", "YouTube is unavailable")
			}
		}
		if _, err = logic.NewSubscription(c.Context(), a.db, viewer(c), req.ChannelID); err != nil {
			if present, e := logic.SubscriptionExists(c.Context(), a.db, viewer(c), req.ChannelID); e != nil || !present {
				return fail(502, "subscribe_failed", "The subscription could not be added")
			}
		}
	}
	return c.SendStatus(204)
}

func (a *API) openPodcast(c fiber.Ctx) error {
	var req struct {
		FeedURL string `json:"feedUrl"`
	}
	if err := body(c, &req); err != nil {
		return err
	}
	req.FeedURL = strings.TrimSpace(req.FeedURL)
	if req.FeedURL == "" {
		return invalid("feedUrl", "Enter a podcast RSS feed URL")
	}
	if aliases, ok := a.db.(database.PodcastFeedAliasesClient); ok {
		if id, err := aliases.GetPodcastFeedAlias(c.Context(), req.FeedURL); err == nil {
			return c.JSON(fiber.Map{"channelId": id})
		} else if !database.IsErrNotFound(err) {
			return err
		}
	}
	if show, err := a.db.GetPodcastShowByFeedURL(c.Context(), req.FeedURL); err == nil {
		return c.JSON(fiber.Map{"channelId": show.ChannelID})
	} else if !database.IsErrNotFound(err) {
		return err
	}
	id, err := logic.OpenPodcast(c.Context(), a.db, req.FeedURL)
	if err != nil {
		if errors.Is(err, logic.ErrInvalidFeedURL) {
			return invalid("feedUrl", "Enter an HTTP or HTTPS podcast RSS feed URL")
		}
		return fail(502, "feed_unavailable", "This podcast feed could not be loaded")
	}
	return c.JSON(fiber.Map{"channelId": id})
}
func (a *API) unsubscribe(c fiber.Ctx) error {
	if err := logic.DeleteSubscription(c.Context(), a.db, viewer(c), c.Params("id")); err != nil {
		return err
	}
	return c.SendStatus(204)
}
func (a *API) subscriptionFilter(c fiber.Ctx) error {
	var req struct {
		Filter string `json:"filter"`
	}
	if err := body(c, &req); err != nil {
		return err
	}
	if req.Filter != "all" && req.Filter != "videos" && req.Filter != "streams" {
		return invalid("filter", "Choose all, videos or streams")
	}
	if err := logic.UpdateSubscriptionFilter(c.Context(), a.db, viewer(c), c.Params("id"), types.VideoFilter(req.Filter)); err != nil {
		return err
	}
	return c.SendStatus(204)
}
func (a *API) refreshChannel(c fiber.Ctx) error {
	if _, err := a.db.GetChannel(c.Context(), c.Params("id")); err != nil {
		return err
	}
	if youtube.DefaultClient == nil {
		if _, err := a.db.GetPodcastShow(c.Context(), c.Params("id")); err != nil {
			return fail(503, "youtube_unavailable", "YouTube is unavailable")
		}
	}
	ctx, cancel := context.WithTimeout(c.Context(), 60*time.Second)
	defer cancel()
	if _, err := logic.RefreshChannelFeed(ctx, a.db, c.Params("id")); err != nil {
		if errors.Is(err, logic.ErrRefreshTooSoon) {
			return fail(409, "refresh_too_soon", "This channel was refreshed recently")
		}
		return fail(502, "refresh_failed", "This feed could not be refreshed")
	}
	return c.SendStatus(204)
}

func (a *API) playlistResult(c fiber.Ctx, id string) error {
	p, err := logic.GetPlaylistPageProps(c.Context(), a.db, viewer(c), id)
	if err != nil {
		return err
	}
	return c.JSON(playlistDTO(p.Playlist))
}
func (a *API) createPlaylist(c fiber.Ctx) error {
	var req struct {
		Name        string `json:"name"`
		Description string `json:"description"`
	}
	if err := body(c, &req); err != nil {
		return err
	}
	if strings.TrimSpace(req.Name) == "" || len(req.Name) > 200 {
		return invalid("name", "Enter a playlist name up to 200 characters")
	}
	if len(req.Description) > 4000 {
		return invalid("description", "Use at most 4000 characters")
	}
	p, err := logic.CreateUserPlaylist(c.Context(), a.db, viewer(c), req.Name, req.Description)
	if err != nil {
		return err
	}
	c.Status(201)
	return a.playlistResult(c, p.ID)
}
func (a *API) updatePlaylist(c fiber.Ctx) error {
	p, err := a.ownedPlaylist(c, true)
	if err != nil {
		return err
	}
	var req struct {
		Name        *string `json:"name"`
		Description *string `json:"description"`
	}
	if err := body(c, &req); err != nil {
		return err
	}
	if req.Name == nil && req.Description == nil {
		return invalid("name", "Submit a name or description change")
	}
	if req.Name != nil {
		if strings.TrimSpace(*req.Name) == "" || len(*req.Name) > 200 {
			return invalid("name", "Enter a playlist name up to 200 characters")
		}
		p.Name = *req.Name
	}
	if req.Description != nil {
		if len(*req.Description) > 4000 {
			return invalid("description", "Use at most 4000 characters")
		}
		p.Description = *req.Description
	}
	if err = logic.UpdateUserPlaylist(c.Context(), a.db, viewer(c), p.ID, p.Name, p.Description); err != nil {
		return err
	}
	return a.playlistResult(c, p.ID)
}
func (a *API) deletePlaylist(c fiber.Ctx) error {
	p, err := a.ownedPlaylist(c, true)
	if err != nil {
		return err
	}
	if err = logic.DeleteUserPlaylist(c.Context(), a.db, viewer(c), p.ID); err != nil {
		return err
	}
	return c.SendStatus(204)
}
func (a *API) importPlaylist(c fiber.Ctx) error {
	var req struct {
		URL string `json:"url"`
	}
	if err := body(c, &req); err != nil {
		return err
	}
	id, ok := logic.PlaylistIDFromURL(req.URL)
	if !ok {
		return invalid("url", "Enter a YouTube playlist URL or ID")
	}
	if youtube.DefaultClient == nil {
		return fail(503, "youtube_unavailable", "YouTube is unavailable")
	}
	ctx, cancel := context.WithTimeout(c.Context(), 60*time.Second)
	defer cancel()
	created, err := logic.ImportYouTubePlaylist(ctx, a.db, viewer(c), id)
	if err != nil {
		e := &apiError{status: 502, code: "import_failed", message: "The playlist could not be fully imported"}
		if created != "" {
			e.fields = map[string]string{"playlistId": created}
		}
		return e
	}
	c.Status(201)
	return a.playlistResult(c, created)
}
func (a *API) syncPlaylist(c fiber.Ctx) error {
	p, err := a.ownedPlaylist(c, true)
	if err != nil {
		return err
	}
	if !p.YoutubePlaylistID.Valid {
		return fail(409, "not_imported", "This playlist has no YouTube source")
	}
	if youtube.DefaultClient == nil {
		return fail(503, "youtube_unavailable", "YouTube is unavailable")
	}
	ctx, cancel := context.WithTimeout(c.Context(), 60*time.Second)
	defer cancel()
	if _, err = logic.SyncYouTubePlaylist(ctx, a.db, viewer(c), p.ID); err != nil {
		if errors.Is(err, logic.ErrSyncTooSoon) {
			return fail(409, "sync_too_soon", "This playlist was synchronized recently")
		}
		return fail(502, "sync_failed", "The playlist could not be fully synchronized")
	}
	return c.SendStatus(204)
}

func videoIDInput(input string) (string, bool) {
	input = strings.TrimSpace(input)
	if youtubeID.MatchString(input) {
		return input, true
	}
	u, err := url.Parse(input)
	if err != nil || u == nil || (u.Scheme != "https" && u.Scheme != "http") {
		return "", false
	}
	host := strings.ToLower(u.Hostname())
	if host != "youtube.com" && host != "www.youtube.com" && host != "m.youtube.com" && host != "music.youtube.com" && host != "youtu.be" {
		return "", false
	}
	id, ok := logic.VideoIDFromURL(input)
	return id, ok && youtubeID.MatchString(id)
}
func (a *API) addPlaylistItem(c fiber.Ctx) error {
	if _, err := a.ownedPlaylist(c, true); err != nil {
		return err
	}
	var req struct {
		MediaID string `json:"mediaId"`
		URL     string `json:"url"`
	}
	if err := body(c, &req); err != nil {
		return err
	}
	if (req.MediaID == "") == (req.URL == "") {
		return invalid("mediaId", "Submit a media ID or YouTube URL")
	}
	if req.URL != "" {
		id, ok := videoIDInput(req.URL)
		if !ok {
			return invalid("url", "Enter a valid YouTube video URL")
		}
		req.MediaID = id
	}
	if err := a.ensureMedia(c.Context(), req.MediaID); err != nil {
		return err
	}
	if _, err := a.db.SetPlaylistMembership(c.Context(), viewer(c), c.Params("id"), req.MediaID, true); err != nil {
		return err
	}
	return c.SendStatus(204)
}
func (a *API) setPlaylistItem(c fiber.Ctx) error {
	if _, err := a.ownedPlaylist(c, true); err != nil {
		return err
	}
	var req struct {
		Present *bool `json:"present"`
	}
	if err := body(c, &req); err != nil {
		return err
	}
	if req.Present == nil {
		return invalid("present", "Choose whether this item belongs in the playlist")
	}
	if *req.Present {
		if err := a.ensureMedia(c.Context(), c.Params("mediaId")); err != nil {
			return err
		}
	}
	if _, err := a.db.SetPlaylistMembership(c.Context(), viewer(c), c.Params("id"), c.Params("mediaId"), *req.Present); err != nil {
		return err
	}
	return c.SendStatus(204)
}
func (a *API) movePlaylistItem(c fiber.Ctx) error {
	if _, err := a.ownedPlaylist(c, true); err != nil {
		return err
	}
	var req struct {
		Direction string `json:"direction"`
	}
	if err := body(c, &req); err != nil {
		return err
	}
	if req.Direction != "up" && req.Direction != "down" {
		return invalid("direction", "Choose up or down")
	}
	if err := logic.MovePlaylistItem(c.Context(), a.db, viewer(c), c.Params("id"), c.Params("mediaId"), req.Direction); err != nil {
		return err
	}
	return c.SendStatus(204)
}
