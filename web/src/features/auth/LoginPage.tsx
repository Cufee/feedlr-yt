import { ArrowLeftIcon, FingerPrintIcon } from "@heroicons/react/24/outline";
import { type SyntheticEvent, useRef, useState } from "react";
import { Metadata } from "@/app/Metadata";
import { Button, Field, Input, Link } from "@/components/ui";
import type { PasskeySnapshot } from "@/lib/browser/passkey-session";
import { usePasskeys } from "@/lib/browser/usePasskeys";
import { safeReturn } from "./passkeys";

export type AuthMode = "signin" | "register" | "legacy";
export interface LoginPageViewProps {
  mode: AuthMode;
  username: string;
  snapshot: Readonly<PasskeySnapshot>;
  onMode: (mode: AuthMode) => void;
  onUsername: (username: string) => void;
  onSignIn: (username?: string) => void;
  onRegister: (username: string) => void;
  onCancel: () => void;
}

export function LoginPageView({
  mode,
  username,
  snapshot,
  onMode,
  onUsername,
  onSignIn,
  onRegister,
  onCancel,
}: LoginPageViewProps) {
  const primary = useRef<HTMLButtonElement>(null);
  const pending = snapshot.phase !== "idle";
  const completing = snapshot.phase === "verifying" || snapshot.phase === "success";
  const registration = mode === "register";
  const status =
    snapshot.phase === "preparing"
      ? "Getting your passkey request ready…"
      : snapshot.phase === "prompt"
        ? registration
          ? "Follow your browser’s steps to create your passkey."
          : "Choose a passkey in your browser to continue."
        : snapshot.phase === "verifying"
          ? registration
            ? "Creating your account…"
            : "Signing you in…"
          : snapshot.phase === "success"
            ? "Signed in…"
            : snapshot.notice;

  function submit(event: SyntheticEvent<HTMLFormElement>) {
    event.preventDefault();
    if (pending || !snapshot.supported) {
      return;
    }
    if (registration) {
      onRegister(username.trim());
    } else {
      onSignIn(username.trim());
    }
  }
  function cancel() {
    onCancel();
    primary.current?.focus({ preventScroll: true });
  }

  const feedback = (
    <>
      {!snapshot.supported && (
        <p className="form-error" role="alert">
          Passkeys need a secure, up-to-date browser. Open Feedlr directly in your browser to
          continue.
        </p>
      )}
      {snapshot.error && (
        <p className="form-error" role="alert">
          {snapshot.error}
        </p>
      )}
      <p className="auth-note" role="status" aria-atomic="true">
        {status}
      </p>
      {pending && !completing && (
        <Button variant="ghost" onClick={cancel}>
          Cancel
        </Button>
      )}
    </>
  );

  return (
    <div className="auth-page">
      <div className="auth-intro">
        <span className="auth-icon" aria-hidden="true">
          <FingerPrintIcon />
        </span>
        <h1>Welcome to Feedlr</h1>
      </div>
      {mode === "signin" ? (
        <section className="panel auth-form form-stack" aria-labelledby="auth-title">
          <div>
            <h2 id="auth-title">Sign in</h2>
          </div>
          <Button
            ref={primary}
            variant="primary"
            pending={pending}
            disabled={!snapshot.supported}
            onClick={() => onSignIn()}
          >
            <FingerPrintIcon aria-hidden="true" />
            Sign in with a passkey
          </Button>
          {feedback}
          <div className="form-stack">
            <Button disabled={pending} onClick={() => onMode("register")}>
              Create an account
            </Button>
            <Button variant="ghost" disabled={pending} onClick={() => onMode("legacy")}>
              Use a username instead
            </Button>
          </div>
          <p className="auth-note">
            Use your screen lock, a saved passkey, or a security key.
            {snapshot.hybrid !== "unsupported" &&
              " Your browser may also offer a passkey from another device."}
          </p>
        </section>
      ) : (
        <form className="panel auth-form form-stack" onSubmit={submit} aria-labelledby="auth-title">
          <div>
            <h2 id="auth-title">{registration ? "Create your account" : "Use your username"}</h2>
            <p className="muted">
              {registration
                ? "Choose a name, then save a passkey to get started."
                : "Older passkeys may need your username to find your account."}
            </p>
          </div>
          <Field
            label="Username"
            htmlFor="auth-username"
            description={
              registration ? "5–18 characters." : "Enter the username you chose when you joined."
            }
            error={snapshot.usernameError || undefined}
          >
            <Input
              id="auth-username"
              name="username"
              autoComplete={registration ? "username" : "username webauthn"}
              value={username}
              onChange={(event) => onUsername(event.target.value)}
              minLength={5}
              maxLength={18}
              required
              readOnly={pending}
              autoFocus
              autoCapitalize="none"
              spellCheck={false}
            />
          </Field>
          {!registration && snapshot.conditional && (
            <p className="auth-note">
              You can also choose a saved passkey from your browser’s suggestions.
            </p>
          )}
          <Button
            ref={primary}
            variant="primary"
            type="submit"
            pending={pending}
            disabled={!snapshot.supported}
          >
            <FingerPrintIcon aria-hidden="true" />
            {registration ? "Create a passkey" : "Continue with a passkey"}
          </Button>
          {feedback}
          <Button variant="ghost" disabled={pending} onClick={() => onMode("signin")}>
            <ArrowLeftIcon aria-hidden="true" />
            Back to sign in
          </Button>
          {registration && (
            <p className="auth-note">
              Your device or passkey provider will help you save it. There’s no password to
              remember.
            </p>
          )}
        </form>
      )}
      <p className="auth-legal">
        By continuing, you agree to the <Link href="/legal/terms-of-service">Terms of Service</Link>{" "}
        and <Link href="/legal/privacy-policy">Privacy Policy</Link>.
      </p>
    </div>
  );
}

export function LoginPage({ returnTo }: { returnTo?: string }) {
  const [mode, setMode] = useState<AuthMode>("signin");
  const [username, setUsername] = useState("");
  const auth = usePasskeys(mode === "legacy", () => window.location.assign(safeReturn(returnTo)));
  function changeMode(next: AuthMode) {
    auth.reset();
    setMode(next);
  }
  return (
    <>
      <Metadata title="Welcome" />
      <LoginPageView
        mode={mode}
        username={username}
        snapshot={auth.snapshot}
        onMode={changeMode}
        onUsername={setUsername}
        onSignIn={auth.login}
        onRegister={auth.register}
        onCancel={auth.cancel}
      />
    </>
  );
}
