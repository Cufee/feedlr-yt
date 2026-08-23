package server

import (
	"io"
	"net/http/httptest"
	"testing"
	"testing/fstest"

	"github.com/gofiber/fiber/v2"
)

func TestStaticWithCacheMiddlewareServesHashedAssetURL(t *testing.T) {
	assets := fstest.MapFS{
		"assets/css/style.css": &fstest.MapFile{Data: []byte("body { color: white; }")},
	}
	app := fiber.New()
	app.Use("/assets", staticWithCacheMiddleware("assets", assets))

	initialResponse, err := app.Test(httptest.NewRequest("GET", "/assets/css/style.css", nil))
	if err != nil {
		t.Fatalf("requesting stable asset URL: %v", err)
	}
	if initialResponse.StatusCode != fiber.StatusTemporaryRedirect {
		t.Fatalf("stable asset status = %d, want %d", initialResponse.StatusCode, fiber.StatusTemporaryRedirect)
	}
	hashedURL := initialResponse.Header.Get("Location")
	if hashedURL == "" {
		t.Fatal("stable asset response did not include a hashed location")
	}
	if initialResponse.Header.Get("Cache-Control") != "no-store" {
		t.Fatalf("stable asset cache control = %q, want no-store", initialResponse.Header.Get("Cache-Control"))
	}

	hashedResponse, err := app.Test(httptest.NewRequest("GET", hashedURL, nil))
	if err != nil {
		t.Fatalf("requesting hashed asset URL: %v", err)
	}
	defer hashedResponse.Body.Close()
	if hashedResponse.StatusCode != fiber.StatusOK {
		t.Fatalf("hashed asset status = %d, want %d", hashedResponse.StatusCode, fiber.StatusOK)
	}
	if hashedResponse.Header.Get("Cache-Control") != "public, max-age=31536000, immutable" {
		t.Fatalf("hashed asset cache control = %q", hashedResponse.Header.Get("Cache-Control"))
	}
	body, err := io.ReadAll(hashedResponse.Body)
	if err != nil {
		t.Fatalf("reading hashed asset: %v", err)
	}
	if string(body) != "body { color: white; }" {
		t.Fatalf("hashed asset body = %q", body)
	}
}
