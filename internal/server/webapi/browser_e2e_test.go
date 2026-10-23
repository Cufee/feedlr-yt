//go:build browser_e2e

package webapi

import (
	"context"
	"fmt"
	"io/fs"
	"mime"
	"net"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"testing"
	"time"

	"github.com/cufee/feedlr-yt/internal/sessions"
	"github.com/go-webauthn/webauthn/webauthn"
	"github.com/gofiber/fiber/v3"
)

// This opt-in test uses the actual production SPA, browser WebAuthn API, native
// JSON handlers, and an isolated migrated SQLite database. Missing build/browser
// prerequisites are errors: CI must build the frontend and install Chromium.
func TestBrowserPasskeyAccountLifecycle(t *testing.T) {
	f := newFixture(t)
	assetsRoot, err := filepath.Abs("../../../assets")
	if err != nil {
		t.Fatal(err)
	}
	index, err := os.ReadFile(filepath.Join(assetsRoot, "ui", "index.html"))
	if err != nil {
		t.Fatalf("production SPA is required; run npm run build first: %v", err)
	}
	script, err := filepath.Abs("../../../web/scripts/auth-browser.mjs")
	if err != nil {
		t.Fatal(err)
	}
	listener, err := net.Listen("tcp4", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = listener.Close() })
	port := listener.Addr().(*net.TCPAddr).Port
	origin := fmt.Sprintf("http://localhost:%d", port)
	t.Setenv("COOKIE_DOMAIN", fmt.Sprintf("localhost:%d", port))
	wa, err := webauthn.New(&webauthn.Config{
		RPDisplayName: "Feedlr browser test", RPID: "localhost", RPOrigins: []string{origin},
	})
	if err != nil {
		t.Fatal(err)
	}
	sc, err := sessions.New(f.db)
	if err != nil {
		t.Fatal(err)
	}
	app := fiber.New()
	Register(app, f.db, sc, wa)
	serveAsset := func(prefix string) fiber.Handler {
		return func(c fiber.Ctx) error {
			name := c.Params("*")
			if !fs.ValidPath(name) {
				return c.SendStatus(404)
			}
			data, err := os.ReadFile(filepath.Join(assetsRoot, prefix, filepath.FromSlash(name)))
			if err != nil {
				return c.SendStatus(404)
			}
			if contentType := mime.TypeByExtension(path.Ext(name)); contentType != "" {
				c.Set("Content-Type", contentType)
			}
			return c.Send(data)
		}
	}
	app.Get("/ui-assets/*", serveAsset("ui"))
	app.Get("/assets/*", serveAsset(""))
	app.Get("/*", func(c fiber.Ctx) error {
		c.Set("Content-Type", "text/html; charset=utf-8")
		return c.Send(index)
	})
	serverDone := make(chan error, 1)
	go func() { serverDone <- app.Listener(listener, fiber.ListenConfig{DisableStartupMessage: true}) }()
	t.Cleanup(func() {
		if err := app.ShutdownWithTimeout(3 * time.Second); err != nil {
			t.Errorf("shutdown browser test server: %v", err)
		}
		select {
		case err := <-serverDone:
			if err != nil {
				t.Errorf("browser test server: %v", err)
			}
		case <-time.After(3 * time.Second):
			t.Error("browser test server did not stop")
		}
	})
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()
	command := exec.CommandContext(ctx, "node", script)
	command.Env = append(os.Environ(), "FEEDLR_E2E_ORIGIN="+origin, "FEEDLR_E2E_OUTPUT="+t.TempDir())
	output, err := command.CombinedOutput()
	if err != nil {
		t.Fatalf("real browser passkey lifecycle failed: %v\n%s", err, output)
	}
	t.Logf("%s", output)
}
