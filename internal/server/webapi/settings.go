package webapi

import (
	"context"
	"errors"
	"os"
	"slices"
	"strings"
	"time"

	"github.com/cufee/feedlr-yt/internal/api/sponsorblock"
	"github.com/cufee/feedlr-yt/internal/database"
	"github.com/cufee/feedlr-yt/internal/logic"
	"github.com/gofiber/fiber/v3"
	"github.com/google/uuid"
)

func (a *API) settings(c fiber.Ctx) error {
	p, err := logic.GetUserSettings(c.Context(), a.db, viewer(c))
	if err != nil {
		return err
	}
	r := Settings{SponsorBlock: SegmentSettings{Enabled: p.SponsorBlock.SponsorBlockEnabled, Categories: append([]string{}, p.SponsorBlock.SelectedSponsorBlockCategories...), AvailableCategories: categoryDTOs(sponsorblock.AvailableCategories)}, PodcastSegments: SegmentSettings{Enabled: p.PodcastSegments.Enabled, Categories: append([]string{}, p.PodcastSegments.SelectedCategories...), AvailableCategories: categoryDTOs(sponsorblock.PodcastAvailableCategories)}, Passkeys: []Passkey{}, Volume: p.PlayerVolume}
	if logic.DefaultYouTubeSync != nil {
		s, err := logic.DefaultYouTubeSync.Status(c.Context(), viewer(c))
		if err != nil {
			return err
		}
		r.YouTubeSync = YouTubeStatus{Available: s.Available, Connected: s.Connected, Enabled: s.Enabled, PlaylistID: s.PlaylistID, LastError: s.LastError, LastSyncedAt: stamp(s.LastSyncedAt)}
	}
	if logic.DefaultYouTubeTVSync != nil {
		s, err := logic.DefaultYouTubeTVSync.Status(c.Context(), viewer(c))
		if err != nil {
			return err
		}
		r.TVSync = TVStatus{Available: s.Available, Connected: s.Connected, Enabled: s.Enabled, ConnectionState: s.ConnectionState, StateReason: s.StateReason, ScreenName: s.ScreenName, LastError: s.LastError, LastConnectedAt: stamp(s.LastConnectedAt), LastEventAt: stamp(s.LastEventAt), LastDisconnectAt: stamp(s.LastDisconnectAt), LastUserActivityAt: stamp(s.LastUserActivityAt)}
	}
	keys, err := a.db.GetUserPasskeys(c.Context(), viewer(c))
	if err != nil {
		return err
	}
	for _, k := range keys {
		r.Passkeys = append(r.Passkeys, Passkey{ID: k.ID, Label: k.Label, CreatedAt: stamp(k.CreatedAt)})
	}
	return c.JSON(r)
}

type segmentPatch struct {
	Enabled    *bool     `json:"enabled"`
	Categories *[]string `json:"categories"`
}

func checkedCategories(values *[]string, valid []string, field string) error {
	if values == nil {
		return nil
	}
	unique := []string{}
	for _, v := range *values {
		if !slices.Contains(valid, v) {
			return invalid(field, "Choose available segment categories")
		}
		if !slices.Contains(unique, v) {
			unique = append(unique, v)
		}
	}
	*values = unique
	return nil
}
func (a *API) patchSettings(c fiber.Ctx) error {
	var req struct {
		SponsorBlock    *segmentPatch `json:"sponsorBlock"`
		PodcastSegments *segmentPatch `json:"podcastSegments"`
	}
	if err := body(c, &req); err != nil {
		return err
	}
	patch := database.SettingsPatch{}
	if req.SponsorBlock != nil {
		if err := checkedCategories(req.SponsorBlock.Categories, sponsorblock.ValidCategoryValues, "sponsorBlock.categories"); err != nil {
			return err
		}
		patch.SponsorEnabled = req.SponsorBlock.Enabled
		patch.SponsorCategories = req.SponsorBlock.Categories
	}
	if req.PodcastSegments != nil {
		if err := checkedCategories(req.PodcastSegments.Categories, sponsorblock.ValidPodcastCategoryValues, "podcastSegments.categories"); err != nil {
			return err
		}
		patch.PodcastEnabled = req.PodcastSegments.Enabled
		patch.PodcastCategories = req.PodcastSegments.Categories
	}
	if patch.SponsorEnabled == nil && patch.SponsorCategories == nil && patch.PodcastEnabled == nil && patch.PodcastCategories == nil {
		return invalid("settings", "Submit a preference change")
	}
	if _, err := logic.GetUserSettings(c.Context(), a.db, viewer(c)); err != nil {
		return err
	}
	if err := a.db.PatchUserSettings(c.Context(), viewer(c), patch); err != nil {
		return err
	}
	return c.SendStatus(204)
}

func (a *API) disconnectYouTube(c fiber.Ctx) error {
	if logic.DefaultYouTubeSync == nil {
		return fail(503, "youtube_sync_unavailable", "YouTube synchronization is not configured")
	}
	if err := logic.DefaultYouTubeSync.Disconnect(c.Context(), viewer(c)); err != nil {
		return err
	}
	return c.SendStatus(204)
}

func (a *API) connectYouTube(c fiber.Ctx) error {
	if logic.DefaultYouTubeSync == nil {
		return fail(503, "youtube_sync_unavailable", "YouTube synchronization is not configured")
	}
	state := uuid.NewString()
	c.Cookie(&fiber.Cookie{Name: "youtube_sync_state", Value: state, Path: "/", HTTPOnly: true, SameSite: "Lax", Secure: !strings.Contains(os.Getenv("COOKIE_DOMAIN"), "localhost"), Expires: time.Now().Add(10 * time.Minute)})
	return c.JSON(fiber.Map{"redirectUrl": logic.DefaultYouTubeSync.OAuthAuthURL(state + ":" + viewer(c))})
}
func enabledBody(c fiber.Ctx) (bool, error) {
	var req struct {
		Enabled *bool `json:"enabled"`
	}
	if err := body(c, &req); err != nil {
		return false, err
	}
	if req.Enabled == nil {
		return false, invalid("enabled", "Choose whether synchronization is enabled")
	}
	return *req.Enabled, nil
}
func (a *API) enableYouTube(c fiber.Ctx) error {
	enabled, err := enabledBody(c)
	if err != nil {
		return err
	}
	if logic.DefaultYouTubeSync == nil {
		return fail(503, "youtube_sync_unavailable", "YouTube synchronization is not configured")
	}
	if err = logic.DefaultYouTubeSync.SetEnabled(c.Context(), viewer(c), enabled); err != nil {
		return err
	}
	return c.SendStatus(204)
}
func (a *API) connectTV(c fiber.Ctx) error {
	var req struct {
		Code string `json:"code"`
	}
	if err := body(c, &req); err != nil {
		return err
	}
	code := strings.TrimSpace(req.Code)
	if len(code) < 3 || len(code) > 100 {
		return invalid("code", "Enter the pairing code shown on your TV")
	}
	if logic.DefaultYouTubeTVSync == nil {
		return fail(503, "tv_sync_unavailable", "TV synchronization is not configured")
	}
	if err := logic.DefaultYouTubeTVSync.PairWithCode(c.Context(), viewer(c), code); err != nil {
		return fail(502, "pairing_failed", "This TV could not be paired. Check the code and try again.")
	}
	return c.SendStatus(204)
}
func (a *API) disconnectTV(c fiber.Ctx) error {
	if logic.DefaultYouTubeTVSync == nil {
		return fail(503, "tv_sync_unavailable", "TV synchronization is not configured")
	}
	if err := logic.DefaultYouTubeTVSync.Disconnect(c.Context(), viewer(c)); err != nil {
		return err
	}
	return c.SendStatus(204)
}
func (a *API) enableTV(c fiber.Ctx) error {
	enabled, err := enabledBody(c)
	if err != nil {
		return err
	}
	if logic.DefaultYouTubeTVSync == nil {
		return fail(503, "tv_sync_unavailable", "TV synchronization is not configured")
	}
	if err = logic.DefaultYouTubeTVSync.SetEnabled(c.Context(), viewer(c), enabled); err != nil {
		return err
	}
	return c.SendStatus(204)
}
func (a *API) deletePasskey(c fiber.Ctx) error {
	writer, ok := a.db.(interface {
		DeleteUserPasskeyKeepingOne(context.Context, string, string) error
	})
	if !ok {
		return fail(503, "passkeys_unavailable", "Passkey storage is unavailable")
	}
	if err := writer.DeleteUserPasskeyKeepingOne(c.Context(), viewer(c), c.Params("id")); err != nil {
		if errors.Is(err, database.ErrLastPasskey) {
			return fail(409, "last_passkey", "Keep at least one passkey so you can sign in again")
		}
		return err
	}
	return c.SendStatus(204)
}
