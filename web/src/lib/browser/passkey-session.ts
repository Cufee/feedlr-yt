import {
  base64URLStringToBuffer,
  browserSupportsWebAuthn,
  browserSupportsWebAuthnAutofill,
  bufferToBase64URLString,
  getBrowserCapabilities,
  startAuthentication,
  startRegistration,
  WebAuthnAbortService,
} from "@simplewebauthn/browser";
import { ApiError, request } from "@/lib/api/client";

export type AuthenticationOptions = Parameters<typeof startAuthentication>[0]["optionsJSON"];
export type RegistrationOptions = Parameters<typeof startRegistration>[0]["optionsJSON"];
type AuthenticationResponse = Awaited<ReturnType<typeof startAuthentication>>;
type RegistrationResponse = Awaited<ReturnType<typeof startRegistration>>;
export type PasskeyIntent =
  | { kind: "login"; username?: string; conditional?: boolean }
  | { kind: "register"; username: string }
  | { kind: "add" };
export type PasskeyPhase = "idle" | "preparing" | "prompt" | "verifying" | "success";
type Capability = "supported" | "unsupported" | "unknown";
export interface PasskeyCapabilities {
  supported: boolean;
  conditional: boolean;
  hybrid: Capability;
}
export interface PasskeyEnvironment {
  supported: () => boolean;
  capabilities: () => Promise<PasskeyCapabilities>;
  loginOptions: (username?: string) => Promise<AuthenticationOptions>;
  registrationOptions: (
    kind: "register" | "add",
    username?: string,
  ) => Promise<RegistrationOptions>;
  authenticate: (
    options: AuthenticationOptions,
    conditional: boolean,
    signal: AbortSignal,
  ) => Promise<AuthenticationResponse>;
  register: (options: RegistrationOptions) => Promise<RegistrationResponse>;
  verify: (
    kind: PasskeyIntent["kind"],
    credential: AuthenticationResponse | RegistrationResponse,
  ) => Promise<void>;
  cancelNative: () => void;
}

const supported = () =>
  globalThis.isSecureContext === true &&
  browserSupportsWebAuthn() &&
  typeof navigator.credentials?.get === "function" &&
  typeof navigator.credentials?.create === "function";

async function capabilities(): Promise<PasskeyCapabilities> {
  if (!supported()) {
    return { supported: false, conditional: false, hybrid: "unknown" };
  }
  let conditional = false;
  let hybrid: Capability = "unknown";
  try {
    const features = await getBrowserCapabilities();
    conditional = features.conditionalGet === "supported";
    hybrid = features.hybridTransport;
  } catch {
    // An optional feature probe must not disable ordinary passkey sign-in.
  }
  if (!conditional) {
    try {
      conditional = await browserSupportsWebAuthnAutofill();
    } catch {
      conditional = false;
    }
  }
  return { supported: true, conditional, hybrid };
}

async function conditionalAuthentication(
  options: AuthenticationOptions,
  signal: AbortSignal,
): Promise<AuthenticationResponse> {
  signal.throwIfAborted();
  const input = document.querySelector<HTMLInputElement>('input[autocomplete~="webauthn"]');
  if (
    !input ||
    input.disabled ||
    input.type === "hidden" ||
    input.getClientRects().length === 0 ||
    getComputedStyle(input).visibility === "hidden"
  ) {
    throw canceled();
  }
  // v14's autofill helper probes capabilities asynchronously before installing
  // its AbortService signal. Use an owned signal here so cancellation during
  // that gap cannot leave a future conditional request running after unmount.
  // The native L3 parser is optional; the fallback supports older autofill UAs.
  const publicKey =
    typeof PublicKeyCredential.parseRequestOptionsFromJSON === "function"
      ? PublicKeyCredential.parseRequestOptionsFromJSON({
          ...options,
          allowCredentials: [],
        } as PublicKeyCredentialRequestOptionsJSON)
      : ({
          ...options,
          challenge: base64URLStringToBuffer(options.challenge),
          allowCredentials: [],
        } as PublicKeyCredentialRequestOptions);
  const result = await navigator.credentials.get({ publicKey, mediation: "conditional", signal });
  if (result?.type !== "public-key") {
    throw new Error("The passkey request was not completed");
  }
  const credential = result as PublicKeyCredential;
  const response = credential.response as AuthenticatorAssertionResponse;
  return {
    id: credential.id,
    rawId: bufferToBase64URLString(credential.rawId),
    type: "public-key",
    response: {
      authenticatorData: bufferToBase64URLString(response.authenticatorData),
      clientDataJSON: bufferToBase64URLString(response.clientDataJSON),
      signature: bufferToBase64URLString(response.signature),
      ...(response.userHandle ? { userHandle: bufferToBase64URLString(response.userHandle) } : {}),
    },
    clientExtensionResults: credential.getClientExtensionResults(),
    ...(credential.authenticatorAttachment === "platform" ||
    credential.authenticatorAttachment === "cross-platform"
      ? { authenticatorAttachment: credential.authenticatorAttachment }
      : {}),
  };
}

const environment: PasskeyEnvironment = {
  supported,
  capabilities,
  loginOptions: async (username) =>
    (
      await request<{ publicKey: AuthenticationOptions }>("/auth/login/begin", {
        method: "POST",
        body: JSON.stringify(username ? { username } : {}),
      })
    ).publicKey,
  registrationOptions: async (kind, username) =>
    (
      await request<{ publicKey: RegistrationOptions }>(
        kind === "add" ? "/passkeys/add/begin" : "/auth/register/begin",
        { method: "POST", body: JSON.stringify(kind === "add" ? {} : { username }) },
      )
    ).publicKey,
  authenticate: (optionsJSON, conditional, signal) =>
    conditional
      ? conditionalAuthentication(optionsJSON, signal)
      : startAuthentication({ optionsJSON }),
  register: (optionsJSON) => startRegistration({ optionsJSON }),
  verify: (kind, credential) =>
    request(kind === "add" ? "/passkeys/add/finish" : `/auth/${kind}/finish`, {
      method: "POST",
      body: JSON.stringify(credential),
    }),
  cancelNative: () => WebAuthnAbortService.cancelCeremony(),
};

const canceled = () => new DOMException("Passkey request canceled", "AbortError");
interface Attempt {
  stopped: boolean;
  phase: PasskeyPhase;
  abort: AbortController;
}
export interface PasskeyOperation {
  promise: Promise<void>;
  cancel: () => void;
}

// One queue covers login, account creation, and adding a key. Canceled begin
// requests drain before a replacement begins, preserving challenge/cookie order.
// Native ownership prevents stale cleanup from aborting a newer ceremony.
export class PasskeyCoordinator {
  private tail: Promise<void> = Promise.resolve();
  private active?: Attempt;
  private nativeOwner?: Attempt;
  constructor(private readonly browser: PasskeyEnvironment = environment) {}

  start(
    intent: PasskeyIntent,
    onPhase: (phase: PasskeyPhase) => void = () => undefined,
  ): PasskeyOperation {
    if (this.active) {
      this.stop(this.active);
    }
    const attempt: Attempt = { stopped: false, phase: "preparing", abort: new AbortController() };
    this.active = attempt;
    onPhase("preparing");
    const promise = this.tail
      .then(async () => {
        this.check(attempt);
        if (!this.browser.supported()) {
          throw new Error("passkeys_unsupported");
        }
        let credential: AuthenticationResponse | RegistrationResponse;
        if (intent.kind === "login") {
          const options = await this.browser.loginOptions(intent.username);
          this.check(attempt);
          attempt.phase = "prompt";
          onPhase("prompt");
          this.nativeOwner = attempt;
          try {
            credential = await this.browser.authenticate(
              options,
              intent.conditional === true,
              attempt.abort.signal,
            );
          } finally {
            if (this.nativeOwner === attempt) {
              this.nativeOwner = undefined;
            }
          }
        } else {
          const options = await this.browser.registrationOptions(
            intent.kind,
            intent.kind === "register" ? intent.username : undefined,
          );
          this.check(attempt);
          attempt.phase = "prompt";
          onPhase("prompt");
          this.nativeOwner = attempt;
          try {
            credential = await this.browser.register(options);
          } finally {
            if (this.nativeOwner === attempt) {
              this.nativeOwner = undefined;
            }
          }
        }
        this.check(attempt);
        attempt.phase = "verifying";
        onPhase("verifying");
        // A submitted finish may have committed on the server. Do not abort that
        // mutation or retry it; await its result and suppress stale UI completion.
        await this.browser.verify(intent.kind, credential);
        this.check(attempt);
        attempt.phase = "success";
        onPhase("success");
      })
      .catch((error: unknown) => {
        if (attempt.stopped || this.active !== attempt) {
          throw canceled();
        }
        throw error;
      })
      .finally(() => {
        if (this.active === attempt) {
          this.active = undefined;
        }
      });
    this.tail = promise.catch(() => undefined);
    return { promise, cancel: () => this.stop(attempt) };
  }

  private check(attempt: Attempt) {
    if (attempt.stopped || this.active !== attempt) {
      throw canceled();
    }
  }
  private stop(attempt: Attempt) {
    if (attempt.stopped) {
      return;
    }
    attempt.stopped = true;
    attempt.abort.abort();
    if (this.nativeOwner === attempt) {
      this.browser.cancelNative();
    }
  }
}

const coordinator = new PasskeyCoordinator();
export interface PasskeySnapshot extends PasskeyCapabilities {
  phase: PasskeyPhase;
  autofillActive: boolean;
  error: string;
  usernameError: string;
  notice: string;
}
export function passkeyError(error: unknown): { error: string; usernameError: string } {
  if (error instanceof Error && error.message === "passkeys_unsupported") {
    return {
      error: "Passkeys aren’t available here. Open Feedlr in a secure, up-to-date browser.",
      usernameError: "",
    };
  }
  if (error instanceof ApiError) {
    if (error.fields?.username) {
      return { error: "", usernameError: error.fields.username };
    }
    if (error.status === 429) {
      return { error: "Too many attempts. Wait a moment before trying again.", usernameError: "" };
    }
    if (/expired|challenge|session/i.test(error.code)) {
      return {
        error: "That sign-in request expired. Try again to start a fresh request.",
        usernameError: "",
      };
    }
    if (error.status === 401) {
      return {
        error: "We couldn’t use that passkey. Try another, or use your username for an older key.",
        usernameError: "",
      };
    }
    return {
      error:
        error.status >= 500
          ? "Feedlr couldn’t be reached. Check your connection and try again."
          : error.message,
      usernameError: "",
    };
  }
  const name = error !== null && typeof error === "object" && "name" in error ? error.name : "";
  const message =
    name === "NotAllowedError" || name === "AbortError"
      ? "That passkey request wasn’t completed. Try again, or choose another sign-in option."
      : name === "InvalidStateError"
        ? "That passkey is already registered. Try signing in with it instead."
        : name === "SecurityError"
          ? "Passkeys aren’t available from this page. Open Feedlr directly in a secure browser tab and try again."
          : name === "ConstraintError" || name === "NotSupportedError"
            ? "This device couldn’t use that passkey. Try a different passkey provider or security key."
            : "We couldn’t finish that passkey request. Check your connection and try again.";
  return { error: message, usernameError: "" };
}

export class PasskeySession {
  private snapshot: Readonly<PasskeySnapshot>;
  private listeners = new Set<() => void>();
  private connection?: { autofill: boolean; success: () => void };
  private operation?: PasskeyOperation;
  private capabilityProbe?: Promise<PasskeyCapabilities>;
  private intention = 0;
  constructor(
    private readonly queue = coordinator,
    private readonly browser: Pick<PasskeyEnvironment, "supported" | "capabilities"> = environment,
  ) {
    this.snapshot = Object.freeze({
      supported: browser.supported(),
      conditional: false,
      hybrid: "unknown",
      phase: "idle",
      autofillActive: false,
      error: "",
      usernameError: "",
      notice: "",
    });
  }
  readonly subscribe = (listener: () => void) => {
    this.listeners.add(listener);
    return () => this.listeners.delete(listener);
  };
  readonly getSnapshot = () => this.snapshot;
  private update(patch: Partial<PasskeySnapshot>) {
    if (
      Object.entries(patch).every(
        ([key, value]) => this.snapshot[key as keyof PasskeySnapshot] === value,
      )
    ) {
      return;
    }
    this.snapshot = Object.freeze({ ...this.snapshot, ...patch });
    for (const listener of this.listeners) {
      listener();
    }
  }
  connect(autofill: boolean, success: () => void) {
    const connection = { autofill, success };
    const intention = this.intention;
    this.connection = connection;
    this.capabilityProbe ??= this.browser.capabilities().catch(() => ({
      supported: this.browser.supported(),
      conditional: false,
      hybrid: "unknown" as const,
    }));
    void this.capabilityProbe.then((features) => {
      if (this.connection !== connection) {
        return;
      }
      this.update(features);
      if (autofill && features.conditional && !this.operation && this.intention === intention) {
        this.begin({ kind: "login", conditional: true });
      }
    });
    return () => {
      if (this.connection === connection) {
        this.connection = undefined;
        this.intention += 1;
        this.operation?.cancel();
        this.operation = undefined;
        this.update({ phase: "idle", autofillActive: false });
      }
    };
  }
  readonly login = (username?: string) =>
    this.begin({ kind: "login", ...(username ? { username } : {}) });
  readonly register = (username: string) => this.begin({ kind: "register", username });
  readonly cancel = () => {
    if (this.snapshot.phase === "verifying" || this.snapshot.phase === "success") {
      return;
    }
    this.intention += 1;
    this.operation?.cancel();
    this.operation = undefined;
    this.update({
      phase: "idle",
      autofillActive: false,
      error: "",
      usernameError: "",
      notice: "Passkey request canceled. You can try again when you’re ready.",
    });
  };
  readonly reset = () => {
    this.intention += 1;
    this.operation?.cancel();
    this.operation = undefined;
    this.update({ phase: "idle", autofillActive: false, error: "", usernameError: "", notice: "" });
  };
  readonly resumeAutofill = () => {
    if (this.connection?.autofill && this.snapshot.conditional && !this.operation) {
      this.begin({ kind: "login", conditional: true });
    }
  };
  private begin(intent: PasskeyIntent) {
    const connection = this.connection;
    if (
      !connection ||
      !this.snapshot.supported ||
      (this.operation && !this.snapshot.autofillActive)
    ) {
      return;
    }
    this.intention += 1;
    const background = intent.kind === "login" && intent.conditional === true;
    this.operation?.cancel();
    this.update({ error: "", usernameError: "", notice: "", autofillActive: background });
    const operation = this.queue.start(intent, (phase) => {
      if (this.connection === connection) {
        this.update({
          phase: background && phase !== "verifying" && phase !== "success" ? "idle" : phase,
          autofillActive: background && phase !== "verifying" && phase !== "success",
        });
      }
    });
    this.operation = operation;
    void operation.promise
      .then(() => {
        if (this.operation === operation && this.connection === connection) {
          this.operation = undefined;
          connection.success();
        }
      })
      .catch((error: unknown) => {
        if (this.operation !== operation || this.connection !== connection) {
          return;
        }
        this.operation = undefined;
        const report = !background || this.snapshot.phase === "verifying";
        this.update({
          phase: "idle",
          autofillActive: false,
          ...(report ? passkeyError(error) : {}),
        });
      });
  }
}

export class AddPasskeySession {
  private operation?: PasskeyOperation;
  constructor(private readonly queue = coordinator) {}
  readonly cancel = () => {
    this.operation?.cancel();
    this.operation = undefined;
  };
  readonly add = () => {
    if (this.operation) {
      return this.operation.promise;
    }
    const operation = this.queue.start({ kind: "add" });
    const promise = operation.promise
      .catch((error: unknown) => {
        if (error instanceof DOMException && error.name === "AbortError") {
          throw error;
        }
        throw new Error(passkeyError(error).error);
      })
      .finally(() => {
        if (this.operation === owned) {
          this.operation = undefined;
        }
      });
    const owned = { ...operation, promise };
    this.operation = owned;
    return promise;
  };
}
