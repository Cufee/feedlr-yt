import { describe, expect, it, vi } from "vitest";
import { ApiError } from "@/lib/api/client";
import {
  AddPasskeySession,
  type AuthenticationOptions,
  PasskeyCoordinator,
  type PasskeyEnvironment,
  PasskeySession,
  passkeyError,
  type RegistrationOptions,
} from "./passkey-session";

const options: AuthenticationOptions = {
  challenge: "cHJldmlldy1jaGFsbGVuZ2U",
  rpId: "feedlr.test",
  userVerification: "required",
};
const registration: RegistrationOptions = {
  challenge: options.challenge,
  rp: { id: "feedlr.test", name: "Feedlr" },
  user: { id: "cmVhZGVy", name: "reader", displayName: "reader" },
  pubKeyCredParams: [{ type: "public-key", alg: -7 }],
  authenticatorSelection: { residentKey: "required", userVerification: "required" },
};
const credential = {
  id: "credential",
  rawId: "Y3JlZGVudGlhbA",
  type: "public-key" as const,
  response: { clientDataJSON: "e30", authenticatorData: "YXV0aA", signature: "c2ln" },
  clientExtensionResults: {},
};
const registered = {
  ...credential,
  response: { clientDataJSON: "e30", attestationObject: "YXR0ZXN0" },
};
function deferred<T>() {
  let resolve: (value: T) => void = () => undefined;
  let reject: (reason: unknown) => void = () => undefined;
  const promise = new Promise<T>((accept, decline) => {
    resolve = accept;
    reject = decline;
  });
  return { promise, resolve, reject };
}
function browser(): PasskeyEnvironment {
  return {
    supported: vi.fn(() => true),
    capabilities: vi.fn(async () => ({
      supported: true,
      conditional: true,
      hybrid: "supported" as const,
    })),
    loginOptions: vi.fn(async () => options),
    registrationOptions: vi.fn(async () => registration),
    authenticate: vi.fn(async () => credential),
    register: vi.fn(async () => registered),
    verify: vi.fn(async () => undefined),
    cancelNative: vi.fn(),
  };
}

describe("PasskeyCoordinator", () => {
  it("drains a canceled begin before replacement and never opens its stale prompt", async () => {
    const runtime = browser();
    const oldBegin = deferred<AuthenticationOptions>();
    vi.mocked(runtime.loginOptions).mockReturnValueOnce(oldBegin.promise);
    const queue = new PasskeyCoordinator(runtime);
    const first = queue.start({ kind: "login", conditional: true });
    await vi.waitFor(() => expect(runtime.loginOptions).toHaveBeenCalledTimes(1));
    first.cancel();
    const second = queue.start({ kind: "login", username: "legacy-reader" });
    await Promise.resolve();
    expect(runtime.loginOptions).toHaveBeenCalledTimes(1);
    expect(runtime.authenticate).not.toHaveBeenCalled();
    oldBegin.resolve(options);
    await expect(first.promise).rejects.toMatchObject({ name: "AbortError" });
    await second.promise;
    expect(runtime.loginOptions).toHaveBeenNthCalledWith(2, "legacy-reader");
    expect(runtime.authenticate).toHaveBeenCalledTimes(1);
    expect(runtime.authenticate).toHaveBeenCalledWith(options, false, expect.any(AbortSignal));
    expect(runtime.verify).toHaveBeenCalledTimes(1);
  });

  it("does not let an older owner's cleanup abort a newer native ceremony", async () => {
    const runtime = browser();
    const oldPrompt = deferred<typeof credential>();
    const newPrompt = deferred<typeof credential>();
    vi.mocked(runtime.authenticate)
      .mockReturnValueOnce(oldPrompt.promise)
      .mockReturnValueOnce(newPrompt.promise);
    vi.mocked(runtime.cancelNative).mockImplementationOnce(() =>
      oldPrompt.reject(new DOMException("Canceled", "AbortError")),
    );
    const queue = new PasskeyCoordinator(runtime);
    const first = queue.start({ kind: "login" });
    await vi.waitFor(() => expect(runtime.authenticate).toHaveBeenCalledTimes(1));
    const second = queue.start({ kind: "login" });
    await expect(first.promise).rejects.toMatchObject({ name: "AbortError" });
    await vi.waitFor(() => expect(runtime.authenticate).toHaveBeenCalledTimes(2));
    first.cancel();
    expect(runtime.cancelNative).toHaveBeenCalledTimes(1);
    newPrompt.resolve(credential);
    await second.promise;
    expect(runtime.verify).toHaveBeenCalledTimes(1);
  });

  it("waits for an issued finish without retry and suppresses its stale success", async () => {
    const runtime = browser();
    const finish = deferred<void>();
    vi.mocked(runtime.verify).mockReturnValueOnce(finish.promise);
    const phases = vi.fn();
    const queue = new PasskeyCoordinator(runtime);
    const first = queue.start({ kind: "login" }, phases);
    await vi.waitFor(() => expect(runtime.verify).toHaveBeenCalledTimes(1));
    first.cancel();
    const second = queue.start({ kind: "register", username: "new-reader" });
    await Promise.resolve();
    expect(runtime.registrationOptions).not.toHaveBeenCalled();
    finish.resolve();
    await expect(first.promise).rejects.toMatchObject({ name: "AbortError" });
    await second.promise;
    expect(phases).not.toHaveBeenCalledWith("success");
    expect(runtime.verify).toHaveBeenCalledTimes(2);
    expect(runtime.verify).toHaveBeenNthCalledWith(2, "register", registered);
  });

  it("shares ordering and cancellation with adding a passkey", async () => {
    const runtime = browser();
    const creation = deferred<typeof registered>();
    vi.mocked(runtime.register).mockReturnValueOnce(creation.promise);
    vi.mocked(runtime.cancelNative).mockImplementationOnce(() =>
      creation.reject(
        Object.assign(new Error("Browser implementation details"), { name: "AbortError" }),
      ),
    );
    const queue = new PasskeyCoordinator(runtime);
    const account = new AddPasskeySession(queue);
    const added = account.add();
    expect(account.add()).toBe(added);
    await vi.waitFor(() => expect(runtime.register).toHaveBeenCalledTimes(1));
    account.cancel();
    await expect(added).rejects.toMatchObject({ name: "AbortError" });
    await queue.start({ kind: "login" }).promise;
    expect(runtime.registrationOptions).toHaveBeenCalledWith("add", undefined);
    expect(runtime.verify).toHaveBeenCalledTimes(1);
    expect(runtime.verify).toHaveBeenCalledWith("login", credential);
  });
});

describe("PasskeySession", () => {
  it("keeps immutable cached snapshots and survives StrictMode cleanup/remount", async () => {
    const runtime = browser();
    const probe = deferred<Awaited<ReturnType<PasskeyEnvironment["capabilities"]>>>();
    vi.mocked(runtime.capabilities).mockReturnValue(probe.promise);
    const prompt = deferred<typeof credential>();
    vi.mocked(runtime.authenticate).mockReturnValue(prompt.promise);
    vi.mocked(runtime.cancelNative).mockImplementation(() =>
      prompt.reject(new DOMException("Canceled", "AbortError")),
    );
    const session = new PasskeySession(new PasskeyCoordinator(runtime), runtime);
    const oldSuccess = vi.fn();
    const success = vi.fn();
    const initial = session.getSnapshot();
    expect(session.getSnapshot()).toBe(initial);
    expect(Object.isFrozen(initial)).toBe(true);
    const oldCleanup = session.connect(true, oldSuccess);
    oldCleanup();
    const cleanup = session.connect(true, success);
    probe.resolve({ supported: true, conditional: true, hybrid: "supported" });
    await vi.waitFor(() => expect(runtime.authenticate).toHaveBeenCalledTimes(1));
    expect(session.getSnapshot().autofillActive).toBe(true);
    expect(session.getSnapshot().phase).toBe("idle");
    oldCleanup();
    expect(runtime.cancelNative).not.toHaveBeenCalled();
    prompt.resolve(credential);
    await vi.waitFor(() => expect(success).toHaveBeenCalledTimes(1));
    expect(oldSuccess).not.toHaveBeenCalled();
    expect(runtime.capabilities).toHaveBeenCalledTimes(1);
    expect(session.getSnapshot()).not.toBe(initial);
    cleanup();
  });

  it("cancels a pending prompt, reports a calm status, and starts a fresh retry", async () => {
    const runtime = browser();
    const prompt = deferred<typeof credential>();
    vi.mocked(runtime.authenticate).mockReturnValueOnce(prompt.promise);
    vi.mocked(runtime.cancelNative).mockImplementationOnce(() =>
      prompt.reject(new DOMException("Canceled", "AbortError")),
    );
    const session = new PasskeySession(new PasskeyCoordinator(runtime), runtime);
    const success = vi.fn();
    const cleanup = session.connect(false, success);
    session.login();
    await vi.waitFor(() => expect(session.getSnapshot().phase).toBe("prompt"));
    session.cancel();
    expect(session.getSnapshot().phase).toBe("idle");
    expect(session.getSnapshot().error).toBe("");
    expect(session.getSnapshot().notice).toContain("canceled");
    session.login();
    await vi.waitFor(() => expect(success).toHaveBeenCalledTimes(1));
    expect(runtime.loginOptions).toHaveBeenCalledTimes(2);
    cleanup();
  });

  it("keeps optional capability failures from blocking ordinary sign-in", async () => {
    const runtime = browser();
    vi.mocked(runtime.capabilities).mockRejectedValue(new Error("Optional API unavailable"));
    const session = new PasskeySession(new PasskeyCoordinator(runtime), runtime);
    const success = vi.fn();
    const cleanup = session.connect(true, success);
    await vi.waitFor(() => expect(runtime.capabilities).toHaveBeenCalledTimes(1));
    session.login();
    await vi.waitFor(() => expect(success).toHaveBeenCalledTimes(1));
    expect(session.getSnapshot().supported).toBe(true);
    expect(session.getSnapshot().conditional).toBe(false);
    cleanup();
  });

  it("never reactivates autofill from a late capability result after cancellation", async () => {
    const runtime = browser();
    const probe = deferred<Awaited<ReturnType<PasskeyEnvironment["capabilities"]>>>();
    const prompt = deferred<typeof credential>();
    vi.mocked(runtime.capabilities).mockReturnValue(probe.promise);
    vi.mocked(runtime.authenticate).mockReturnValueOnce(prompt.promise);
    vi.mocked(runtime.cancelNative).mockImplementationOnce(() =>
      prompt.reject(new DOMException("Canceled", "AbortError")),
    );
    const session = new PasskeySession(new PasskeyCoordinator(runtime), runtime);
    const success = vi.fn();
    const cleanup = session.connect(true, success);
    session.login();
    await vi.waitFor(() => expect(session.getSnapshot().phase).toBe("prompt"));
    session.cancel();
    probe.resolve({ supported: true, conditional: true, hybrid: "supported" });
    await vi.waitFor(() => expect(session.getSnapshot().conditional).toBe(true));
    expect(runtime.loginOptions).toHaveBeenCalledTimes(1);
    expect(session.getSnapshot().phase).toBe("idle");
    expect(session.getSnapshot().notice).toContain("canceled");
    expect(success).not.toHaveBeenCalled();
    cleanup();
  });

  it("leaves verification in progress when cancel is pressed and completes once", async () => {
    const runtime = browser();
    const finish = deferred<void>();
    vi.mocked(runtime.verify).mockReturnValue(finish.promise);
    const session = new PasskeySession(new PasskeyCoordinator(runtime), runtime);
    const success = vi.fn();
    const cleanup = session.connect(false, success);
    session.login();
    await vi.waitFor(() => expect(session.getSnapshot().phase).toBe("verifying"));
    session.cancel();
    session.login();
    expect(session.getSnapshot().phase).toBe("verifying");
    expect(runtime.loginOptions).toHaveBeenCalledTimes(1);
    finish.resolve();
    await vi.waitFor(() => expect(success).toHaveBeenCalledTimes(1));
    expect(runtime.verify).toHaveBeenCalledTimes(1);
    cleanup();
  });

  it("never redirects from a late finish after leaving the login page", async () => {
    const runtime = browser();
    const finish = deferred<void>();
    vi.mocked(runtime.verify).mockReturnValue(finish.promise);
    const session = new PasskeySession(new PasskeyCoordinator(runtime), runtime);
    const success = vi.fn();
    const cleanup = session.connect(false, success);
    session.login();
    await vi.waitFor(() => expect(runtime.verify).toHaveBeenCalledTimes(1));
    cleanup();
    finish.resolve();
    await Promise.resolve();
    await Promise.resolve();
    expect(success).not.toHaveBeenCalled();
    expect(session.getSnapshot().phase).toBe("idle");
  });
});

describe("passkey errors", () => {
  it("preserves field validation but hides browser internals and distinguishes expiry", () => {
    expect(
      passkeyError(
        new ApiError(422, "invalid_username", "Invalid", { username: "Use 5–18 characters" }),
      ),
    ).toEqual({ error: "", usernameError: "Use 5–18 characters" });
    expect(
      passkeyError(
        new DOMException("Internal authenticator failure and RP identifier", "NotAllowedError"),
      ).error,
    ).toBe("That passkey request wasn’t completed. Try again, or choose another sign-in option.");
    expect(
      passkeyError(new ApiError(401, "challenge_expired", "Internal challenge metadata")).error,
    ).toContain("expired");
  });
});
