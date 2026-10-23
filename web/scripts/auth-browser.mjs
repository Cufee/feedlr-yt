import assert from "node:assert/strict";
import { existsSync } from "node:fs";
import { join } from "node:path";
import { chromium, expect } from "@playwright/test";

const origin = process.env.FEEDLR_E2E_ORIGIN;
assert.ok(origin, "FEEDLR_E2E_ORIGIN must point to the isolated Go test server");
assert.equal(new URL(origin).hostname, "localhost");
assert.equal(new URL(origin).protocol, "http:");

const executablePath =
  process.env.PLAYWRIGHT_CHROMIUM_EXECUTABLE_PATH ??
  (existsSync("/run/current-system/sw/bin/chromium")
    ? "/run/current-system/sw/bin/chromium"
    : undefined);
const browser = await chromium.launch({ executablePath, headless: true });
const context = await browser.newContext({ baseURL: origin, serviceWorkers: "block" });
const page = await context.newPage();
page.setDefaultTimeout(15_000);
const browserErrors = [];
const externalRequests = [];
const apiResponses = [];
page.on("pageerror", (error) => browserErrors.push(error.message));
page.on("console", (message) => {
  if (message.type() === "error") {
    browserErrors.push(message.text());
  }
});
page.on("response", (response) => {
  if (response.url().startsWith(`${origin}/api/v1/`)) {
    apiResponses.push({ path: new URL(response.url()).pathname, status: response.status() });
  }
});
await context.route("**/*", (route) => {
  const url = new URL(route.request().url());
  if (url.origin !== origin) {
    externalRequests.push(url.origin);
    return route.abort("blockedbyclient");
  }
  return route.continue();
});

async function actionResponse(path, status, action) {
  const pending = page.waitForResponse((response) => response.url() === `${origin}/api/v1${path}`);
  await action();
  const response = await pending;
  if (response.status() !== status) {
    throw new Error(
      `${path}: expected ${status}, received ${response.status()}: ${await response.text()}`,
    );
  }
  return response;
}

async function metadata(title, description, pathname) {
  await expect(page.locator("head title")).toHaveCount(1);
  await expect(page).toHaveTitle(`${title} · Feedlr`);
  await expect(page.locator('meta[name="description"]')).toHaveCount(1);
  await expect(page.locator('meta[name="description"]')).toHaveAttribute("content", description);
  await expect(page.locator('meta[property="og:title"]')).toHaveAttribute(
    "content",
    `${title} · Feedlr`,
  );
  await expect(page.locator('meta[property="og:description"]')).toHaveAttribute(
    "content",
    description,
  );
  await expect(page.locator('link[rel="canonical"]')).toHaveCount(1);
  await expect(page.locator('link[rel="canonical"]')).toHaveAttribute(
    "href",
    `${origin}${pathname}`,
  );
  await expect(page.locator('meta[property="og:url"]')).toHaveAttribute(
    "content",
    `${origin}${pathname}`,
  );
}

// Exercise the browser's real WebAuthn serializer and authenticator, then send
// deliberate protocol violations directly to the JSON endpoint. Controlled
// rejection responses stay outside page console noise; no API is mocked.
async function nativeAssertion() {
  return page.evaluate(async () => {
    const begin = await fetch("/api/v1/auth/login/begin", {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: "{}",
    });
    if (!begin.ok) {
      throw new Error(`Native test begin failed: ${begin.status}`);
    }
    const options = await begin.json();
    const decode = (value) =>
      Uint8Array.from(atob(value.replaceAll("-", "+").replaceAll("_", "/")), (c) =>
        c.charCodeAt(0),
      );
    const publicKey =
      typeof PublicKeyCredential.parseRequestOptionsFromJSON === "function"
        ? PublicKeyCredential.parseRequestOptionsFromJSON(options.publicKey)
        : { ...options.publicKey, challenge: decode(options.publicKey.challenge) };
    const credential = await navigator.credentials.get({ publicKey });
    if (!credential) {
      throw new Error("Native test authenticator returned no credential");
    }
    if (typeof credential.toJSON === "function") {
      return credential.toJSON();
    }
    const encode = (value) =>
      btoa(String.fromCharCode(...new Uint8Array(value)))
        .replaceAll("+", "-")
        .replaceAll("/", "_")
        .replaceAll("=", "");
    return {
      id: credential.id,
      rawId: encode(credential.rawId),
      type: credential.type,
      clientExtensionResults: credential.getClientExtensionResults(),
      response: {
        authenticatorData: encode(credential.response.authenticatorData),
        clientDataJSON: encode(credential.response.clientDataJSON),
        signature: encode(credential.response.signature),
        userHandle: credential.response.userHandle ? encode(credential.response.userHandle) : null,
      },
    };
  });
}

try {
  const cdp = await context.newCDPSession(page);
  await cdp.send("WebAuthn.enable");
  const authenticatorOptions = {
    protocol: "ctap2",
    transport: "internal",
    hasResidentKey: true,
    hasUserVerification: true,
    isUserVerified: true,
    automaticPresenceSimulation: true,
  };
  let { authenticatorId } = await cdp.send("WebAuthn.addVirtualAuthenticator", {
    options: authenticatorOptions,
  });
  const username = "browseruser";
  await page.goto("/login");
  await expect(page.getByRole("heading", { name: "Welcome to Feedlr" })).toBeVisible();
  await expect(page.getByLabel("Username", { exact: true })).toHaveCount(0);
  await page.getByRole("button", { name: "Create an account", exact: true }).click();
  await page.getByLabel("Username", { exact: true }).fill(username);
  await actionResponse("/auth/register/finish", 204, () =>
    page.getByRole("button", { name: "Create a passkey", exact: true }).click(),
  );
  await expect(page).toHaveURL(`${origin}/app`);
  await expect(
    page.getByRole("heading", { name: "Your feed", exact: true, level: 1 }),
  ).toBeVisible();
  await expect(
    page.getByRole("heading", { name: "No subscriptions", exact: true, level: 2 }),
  ).toBeVisible();
  const { credentials } = await cdp.send("WebAuthn.getCredentials", { authenticatorId });
  assert.equal(credentials.length, 1, "signup did not create a real authenticator credential");
  assert.equal(
    credentials[0].isResidentCredential,
    true,
    "signup did not create a discoverable passkey",
  );
  await metadata("Your feed", "New videos and podcast episodes from your subscriptions.", "/app");

  // Exercise SPA navigation, browser history and server-served direct links.
  await page.getByRole("link", { name: "Watch Later", exact: true }).click();
  await expect(page.getByRole("heading", { name: "Watch Later", exact: true })).toBeVisible();
  await metadata("Watch Later", "Your saved videos and podcast episodes.", "/app/watch-later");
  await page.getByRole("link", { name: "Recently played", exact: true }).click();
  await expect(page.getByRole("heading", { name: "Recently played", exact: true })).toBeVisible();
  await expect(
    page.getByRole("heading", { name: "No history", exact: true, level: 2 }),
  ).toBeVisible();
  await metadata(
    "Recently played",
    "Your recently played videos and podcast episodes.",
    "/app/recent",
  );
  await page.goBack();
  await expect(page.getByRole("heading", { name: "Watch Later", exact: true })).toBeVisible();
  await metadata("Watch Later", "Your saved videos and podcast episodes.", "/app/watch-later");
  await page.reload();
  await expect(
    page.getByRole("heading", { name: "No saved items", exact: true, level: 2 }),
  ).toBeVisible();
  await metadata("Watch Later", "Your saved videos and podcast episodes.", "/app/watch-later");
  await page.goto("/app/settings/account");
  await expect(page.getByRole("heading", { name: "Passkeys", exact: true })).toBeVisible();
  await expect(page.getByRole("button", { name: "Remove", exact: true })).toBeDisabled();
  const originalKeys = (await (await context.request.get("/api/v1/settings")).json()).passkeys;
  // Register a separate physical security key, rather than asking a platform
  // authenticator to create an excluded duplicate credential for the account.
  await cdp.send("WebAuthn.removeVirtualAuthenticator", { authenticatorId });
  ({ authenticatorId } = await cdp.send("WebAuthn.addVirtualAuthenticator", {
    options: { ...authenticatorOptions, transport: "usb" },
  }));
  await actionResponse("/passkeys/add/finish", 204, () =>
    page.getByRole("button", { name: "Add passkey", exact: true }).click(),
  );
  const remove = page.getByRole("button", { name: "Remove", exact: true });
  await expect(remove).toHaveCount(2);
  const newKeys = (await (await context.request.get("/api/v1/settings")).json()).passkeys;
  const addedIndex = newKeys.findIndex((key) => !originalKeys.some((old) => old.id === key.id));
  assert.ok(addedIndex >= 0, "adding a passkey did not persist a new key");
  await expect(remove.nth(addedIndex)).toBeEnabled();
  await remove.nth(addedIndex).click();
  const dialog = page.getByRole("dialog", { name: "Remove this passkey?" });
  await expect(dialog.getByRole("heading", { name: "Remove this passkey?" })).toBeVisible();
  const deletion = page.waitForResponse(
    (response) =>
      new URL(response.url()).pathname.startsWith("/api/v1/passkeys/") &&
      response.request().method() === "DELETE",
  );
  await dialog.getByRole("button", { name: "Delete", exact: true }).click();
  assert.equal((await deletion).status(), 204);
  await expect(dialog).toHaveCount(0);
  await expect(page.getByRole("heading", { name: "Settings", exact: true })).toBeFocused();
  await expect(remove).toHaveCount(1);
  await expect(remove).toBeDisabled();
  await cdp.send("WebAuthn.removeVirtualAuthenticator", { authenticatorId });
  ({ authenticatorId } = await cdp.send("WebAuthn.addVirtualAuthenticator", {
    options: authenticatorOptions,
  }));
  await cdp.send("WebAuthn.addCredential", { authenticatorId, credential: credentials[0] });
  const beforeLogout = (await context.cookies()).find((cookie) => cookie.name === "session_id");
  assert.ok(beforeLogout, "registration did not persist a session cookie");
  await actionResponse("/logout", 204, () =>
    page.getByRole("button", { name: "Sign out", exact: true }).click(),
  );
  await expect(page).toHaveURL(`${origin}/login`);
  await expect(page.getByRole("heading", { name: "Welcome to Feedlr" })).toBeVisible();
  assert.equal(
    (await context.cookies()).some((cookie) => cookie.name === "session_id"),
    false,
    "logout did not clear the browser cookie",
  );
  // Replay the previous cookie independently; browser logout must revoke it
  // server-side rather than merely remove the cookie from this browser.
  const replay = await context.request.get("/api/v1/watch-later", {
    headers: { Cookie: `session_id=${beforeLogout.value}` },
  });
  assert.equal(replay.status(), 401, "a logged-out cookie still authorized private data");
  assert.equal((await replay.json()).error.code, "unauthenticated");
  await expect(page.getByLabel("Username", { exact: true })).toHaveCount(0);
  await cdp.send("WebAuthn.setAutomaticPresenceSimulation", { authenticatorId, enabled: false });
  const conditional = await page.evaluate(async () =>
    typeof PublicKeyCredential.isConditionalMediationAvailable === "function"
      ? PublicKeyCredential.isConditionalMediationAvailable()
      : false,
  );
  if (conditional) {
    const begin = await actionResponse("/auth/login/begin", 200, () =>
      page.getByRole("button", { name: "Use a username instead", exact: true }).click(),
    );
    assert.deepEqual(begin.request().postDataJSON(), {});
    await expect(page.getByLabel("Username", { exact: true })).toHaveAttribute(
      "autocomplete",
      "username webauthn",
    );
    await page.getByRole("button", { name: "Back to sign in", exact: true }).click();
    await expect(page.getByLabel("Username", { exact: true })).toHaveCount(0);
  }
  const begin = await actionResponse("/auth/login/begin", 200, () =>
    page.getByRole("button", { name: "Sign in with a passkey", exact: true }).click(),
  );
  assert.deepEqual(begin.request().postDataJSON(), {}, "primary sign-in sent a username");
  await page.getByRole("button", { name: "Cancel", exact: true }).click();
  await expect(page.getByRole("button", { name: "Cancel", exact: true })).toHaveCount(0);
  await expect(
    page.getByText("Passkey request canceled. You can try again when you’re ready."),
  ).toBeVisible();
  await expect(
    page.getByRole("button", { name: "Sign in with a passkey", exact: true }),
  ).toBeFocused();
  await cdp.send("WebAuthn.setAutomaticPresenceSimulation", { authenticatorId, enabled: true });
  const missing = await context.request.post("/api/v1/auth/login/finish", { data: {} });
  assert.equal(missing.status(), 400, "a missing credential authenticated");
  for (const [name, change] of [
    [
      "missing handle",
      (credential) => {
        delete credential.response.userHandle;
      },
    ],
    [
      "unknown handle",
      (credential) => {
        credential.response.userHandle = Buffer.from("missing-account").toString("base64url");
      },
    ],
    [
      "wrong account handle",
      (credential) => {
        credential.response.userHandle = Buffer.from("viewer").toString("base64url");
      },
    ],
    [
      "unknown credential",
      (credential) => {
        credential.id = Buffer.from("unknown-key").toString("base64url");
        credential.rawId = credential.id;
      },
    ],
    [
      "wrong origin",
      (credential) => {
        const data = JSON.parse(
          Buffer.from(credential.response.clientDataJSON, "base64url").toString(),
        );
        data.origin = "https://evil.example";
        credential.response.clientDataJSON = Buffer.from(JSON.stringify(data)).toString(
          "base64url",
        );
      },
    ],
    [
      "no user verification",
      (credential) => {
        const data = Buffer.from(credential.response.authenticatorData, "base64url");
        data[32] &= ~4;
        credential.response.authenticatorData = data.toString("base64url");
      },
    ],
  ]) {
    const credential = await nativeAssertion();
    change(credential);
    const rejected = await context.request.post("/api/v1/auth/login/finish", {
      data: credential,
      headers: { Origin: origin },
    });
    assert.equal(rejected.status(), 401, `${name} authenticated`);
    const repeated = await context.request.post("/api/v1/auth/login/finish", {
      data: credential,
      headers: { Origin: origin },
    });
    assert.equal(repeated.status(), 401, `${name} retained a reusable challenge`);
  }
  const assertion = await nativeAssertion();
  const anonymous = (await context.cookies()).find((cookie) => cookie.name === "session_id");
  assert.ok(anonymous);
  const accepted = await context.request.post("/api/v1/auth/login/finish", {
    data: assertion,
    headers: { Origin: origin },
  });
  assert.equal(accepted.status(), 204, "a valid native assertion failed");
  const reused = await context.request.post("/api/v1/auth/login/finish", {
    data: assertion,
    headers: { Origin: origin, Cookie: `session_id=${anonymous.value}` },
  });
  assert.equal(reused.status(), 401, "a completed native assertion could be replayed");
  assert.equal((await context.request.post("/api/v1/logout")).status(), 204);
  await actionResponse("/auth/login/finish", 204, () =>
    page.getByRole("button", { name: "Sign in with a passkey", exact: true }).click(),
  );
  await expect(page).toHaveURL(`${origin}/app`);
  await expect(
    page.getByRole("heading", { name: "Your feed", exact: true, level: 1 }),
  ).toBeVisible();
  await expect(
    page.getByRole("heading", { name: "No subscriptions", exact: true, level: 2 }),
  ).toBeVisible();
  const session = await context.request.get("/api/v1/session");
  assert.equal(session.status(), 200);
  const identity = await session.json();
  assert.equal(identity.authenticated, true);
  assert.equal(identity.username, username);
  assert.ok(identity.userId);
  assert.deepEqual(externalRequests, [], "the SPA attempted a nonloopback request");
  assert.deepEqual(browserErrors, [], "the real production SPA emitted browser errors");
  assert.ok(apiResponses.length > 10, "browser did not exercise the real JSON API");
  assert.equal(
    apiResponses.every((response) => response.status >= 200 && response.status < 300),
    true,
    JSON.stringify(apiResponses),
  );
  console.log(
    `Browser auth passed: discoverable registration, platform/security-key addition/removal, logout revocation, username-free login, conditional=${conditional}, cancellation/retry, native credential/handle/origin/UV/replay rejection, direct links/history/current metadata; ${apiResponses.length} JSON responses; no browser errors or external requests.`,
  );
} catch (error) {
  console.error(
    "Overlay layout:",
    JSON.stringify(
      await page.locator('[role="dialog"]').evaluateAll((dialogs) =>
        dialogs.map((dialog) => {
          const ancestors = [];
          for (let element = dialog; element; element = element.parentElement) {
            const style = getComputedStyle(element);
            const box = element.getBoundingClientRect();
            ancestors.push({
              tag: element.tagName,
              className: element.className,
              display: style.display,
              visibility: style.visibility,
              position: style.position,
              opacity: style.opacity,
              width: box.width,
              height: box.height,
              inert: element.inert,
            });
          }
          return ancestors;
        }),
      ),
    ),
  );
  if (process.env.FEEDLR_E2E_OUTPUT) {
    await page.screenshot({ path: join(process.env.FEEDLR_E2E_OUTPUT, "auth-failure.png") });
  }
  console.error("API responses:", JSON.stringify(apiResponses));
  console.error("Browser errors:", JSON.stringify(browserErrors));
  throw error;
} finally {
  await browser.close();
}
