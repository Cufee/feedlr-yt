package webapi

import (
	"context"
	"errors"
	"io"
	"net/http"
	"time"

	shared "github.com/cufee/feedlr-yt/internal/content"
	"github.com/gofiber/fiber/v3"
)

var legalHTTPClient = &http.Client{Timeout: 8 * time.Second, CheckRedirect: func(req *http.Request, via []*http.Request) error {
	if len(via) > 3 || req.URL.Scheme != "https" || req.URL.Host != "byvko-dev.github.io" {
		return errors.New("legal document redirect was rejected")
	}
	return nil
}}

func (a *API) legal(c fiber.Ctx) error {
	var location string
	switch c.Params("document") {
	case "privacy-policy":
		location = "https://byvko-dev.github.io/legal/privacy-policy"
	case "terms-of-service":
		location = "https://byvko-dev.github.io/legal/terms-of-service-partial"
	default:
		return fail(404, "not_found", "This legal document does not exist")
	}
	ctx, cancel := context.WithTimeout(c.Context(), 8*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, location, nil)
	if err != nil {
		return err
	}
	res, err := legalHTTPClient.Do(req)
	if err != nil {
		return fail(502, "legal_unavailable", "This document could not be loaded. Please try again.")
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		return fail(502, "legal_unavailable", "This document could not be loaded. Please try again.")
	}
	const limit = 1 << 20
	content, err := io.ReadAll(io.LimitReader(res.Body, limit+1))
	if err != nil || len(content) > limit {
		return fail(502, "legal_unavailable", "This document could not be loaded. Please try again.")
	}
	return c.JSON(fiber.Map{"html": shared.SanitizePodcastHTML(string(content))})
}
