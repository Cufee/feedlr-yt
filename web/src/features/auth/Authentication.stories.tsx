import type { Meta, StoryObj } from "@storybook/react-vite";
import { useState } from "react";
import { expect, userEvent, waitFor, within } from "storybook/test";
import type { PasskeyPhase, PasskeySnapshot } from "@/lib/browser/passkey-session";
import { type AuthMode, LoginPageView } from "./LoginPage";
import "@/app/app.css";

interface ScenarioProps {
  mode?: AuthMode;
  phase?: PasskeyPhase;
  supported?: boolean;
  error?: string;
  notice?: string;
  usernameError?: string;
}
function AuthenticationScenario({
  mode: initialMode = "signin",
  phase = "idle",
  supported = true,
  error = "",
  notice = "",
  usernameError = "",
}: ScenarioProps) {
  const [mode, setMode] = useState<AuthMode>(initialMode);
  const [username, setUsername] = useState("");
  const [snapshot, setSnapshot] = useState<PasskeySnapshot>({
    supported,
    conditional: true,
    hybrid: "supported",
    phase,
    autofillActive: false,
    error,
    notice,
    usernameError,
  });
  function changeMode(mode: AuthMode) {
    setMode(mode);
    setSnapshot((current) => ({
      ...current,
      phase: "idle",
      error: "",
      notice: "",
      usernameError: "",
    }));
  }
  function start() {
    setSnapshot((current) => ({
      ...current,
      phase: "prompt",
      error: "",
      notice: "",
      usernameError: "",
    }));
  }
  return (
    <main className="min-h-dvh px-5 py-8">
      <LoginPageView
        mode={mode}
        username={username}
        snapshot={snapshot}
        onMode={changeMode}
        onUsername={setUsername}
        onSignIn={start}
        onRegister={start}
        onCancel={() =>
          setSnapshot((current) => ({
            ...current,
            phase: "idle",
            error: "",
            notice: "Passkey request canceled. You can try again when you’re ready.",
          }))
        }
      />
    </main>
  );
}
const meta = {
  title: "Authentication/Passkeys",
  component: AuthenticationScenario,
  parameters: { layout: "fullscreen" },
} satisfies Meta<typeof AuthenticationScenario>;
export default meta;
type Story = StoryObj<typeof meta>;

export const SignInDesktop: Story = { globals: { viewport: { value: "desktop" } } };
export const SignInMobile: Story = { globals: { viewport: { value: "phone" } } };
export const CreateAccountDesktop: Story = {
  args: { mode: "register" },
  globals: { viewport: { value: "desktop" } },
};
export const CreateAccountMobile: Story = {
  args: { mode: "register" },
  globals: { viewport: { value: "phone" } },
};
export const LegacyUsernameDesktop: Story = {
  args: { mode: "legacy" },
  globals: { viewport: { value: "desktop" } },
};
export const LegacyUsernameMobile: Story = {
  args: { mode: "legacy" },
  globals: { viewport: { value: "phone" } },
};
export const ChoosePasskeyDesktop: Story = {
  args: { phase: "prompt" },
  globals: { viewport: { value: "desktop" } },
};
export const ChoosePasskeyMobile: Story = {
  args: { phase: "prompt" },
  globals: { viewport: { value: "phone" } },
};
export const CompletingDesktop: Story = {
  args: { phase: "verifying" },
  globals: { viewport: { value: "desktop" } },
};
export const CompletingMobile: Story = {
  args: { phase: "verifying" },
  globals: { viewport: { value: "phone" } },
};
export const CanceledDesktop: Story = {
  args: { notice: "Passkey request canceled. You can try again when you’re ready." },
  globals: { viewport: { value: "desktop" } },
};
export const CanceledMobile: Story = {
  args: { notice: "Passkey request canceled. You can try again when you’re ready." },
  globals: { viewport: { value: "phone" } },
};
export const TryAgainDesktop: Story = {
  args: {
    error: "That passkey request wasn’t completed. Try again, or choose another sign-in option.",
  },
  globals: { viewport: { value: "desktop" } },
};
export const TryAgainMobile: Story = {
  args: {
    error: "That passkey request wasn’t completed. Try again, or choose another sign-in option.",
  },
  globals: { viewport: { value: "phone" } },
};
export const UnsupportedDesktop: Story = {
  args: { supported: false },
  globals: { viewport: { value: "desktop" } },
};
export const UnsupportedMobile: Story = {
  args: { supported: false },
  globals: { viewport: { value: "phone" } },
};
export const UsernameErrorDesktop: Story = {
  args: { mode: "register", usernameError: "That username isn’t available. Choose another." },
  globals: { viewport: { value: "desktop" } },
};
export const UsernameErrorMobile: Story = {
  args: { mode: "register", usernameError: "That username isn’t available. Choose another." },
  globals: { viewport: { value: "phone" } },
};

export const OneButtonSignInAndCancel: Story = {
  globals: { viewport: { value: "phone" } },
  play: async ({ canvasElement }) => {
    const canvas = within(canvasElement);
    const primary = canvas.getByRole("button", { name: "Sign in with a passkey" });
    await expect(canvas.queryByRole("textbox")).not.toBeInTheDocument();
    await userEvent.click(primary);
    await expect(primary).toHaveAttribute("aria-busy", "true");
    await expect(canvas.getByRole("status")).toHaveTextContent("Choose a passkey in your browser");
    await userEvent.click(canvas.getByRole("button", { name: "Cancel" }));
    await expect(primary).toHaveFocus();
    await expect(primary).not.toHaveAttribute("aria-busy", "true");
    await expect(canvas.getByRole("status")).toHaveTextContent("Passkey request canceled");
  },
};
export const AccountCreationIsSeparate: Story = {
  globals: { viewport: { value: "desktop" } },
  play: async ({ canvasElement }) => {
    const canvas = within(canvasElement);
    await userEvent.click(canvas.getByRole("button", { name: "Create an account" }));
    const username = canvas.getByRole("textbox", { name: "Username" });
    await expect(username).toHaveFocus();
    await expect(username).toHaveAttribute("autocomplete", "username");
    await userEvent.type(username, "quiet-reader");
    const create = canvas.getByRole("button", { name: "Create a passkey" });
    await userEvent.click(create);
    await expect(username).toHaveAttribute("readonly");
    await userEvent.click(canvas.getByRole("button", { name: "Cancel" }));
    await expect(create).toHaveFocus();
    await userEvent.click(canvas.getByRole("button", { name: "Back to sign in" }));
    await expect(canvas.queryByRole("textbox")).not.toBeInTheDocument();
    await expect(canvas.getByRole("button", { name: "Sign in with a passkey" })).toBeVisible();
  },
};
export const LegacyAutofillUsesVisibleInput: Story = {
  globals: { viewport: { value: "phone" } },
  play: async ({ canvasElement }) => {
    const canvas = within(canvasElement);
    await userEvent.click(canvas.getByRole("button", { name: "Use a username instead" }));
    const username = canvas.getByRole("textbox", { name: "Username" });
    await expect(username).toBeVisible();
    await expect(username).toHaveAttribute("autocomplete", "username webauthn");
    await expect(username).toHaveFocus();
    await userEvent.type(username, "older-reader");
    await userEvent.click(canvas.getByRole("button", { name: "Continue with a passkey" }));
    await expect(canvas.getByRole("status")).toHaveTextContent("Choose a passkey in your browser");
    await userEvent.click(canvas.getByRole("button", { name: "Cancel" }));
    await expect(canvas.getByRole("button", { name: "Continue with a passkey" })).toHaveFocus();
  },
};
export const FailedRequestCanRetry: Story = {
  args: {
    error: "That passkey request wasn’t completed. Try again, or choose another sign-in option.",
  },
  globals: { viewport: { value: "desktop" } },
  play: async ({ canvasElement }) => {
    const canvas = within(canvasElement);
    await expect(canvas.getByRole("alert")).toBeVisible();
    await userEvent.click(canvas.getByRole("button", { name: "Sign in with a passkey" }));
    await waitFor(() => expect(canvas.queryByRole("alert")).not.toBeInTheDocument());
    await expect(canvas.getByRole("button", { name: "Cancel" })).toBeVisible();
  },
};
export const VerificationCompletesOnce: Story = {
  args: { phase: "verifying" },
  globals: { viewport: { value: "phone" } },
  play: async ({ canvasElement }) => {
    const canvas = within(canvasElement);
    await expect(canvas.queryByRole("button", { name: "Cancel" })).not.toBeInTheDocument();
    await expect(canvas.getByRole("button", { name: "Create an account" })).toBeDisabled();
    await expect(canvas.getByRole("button", { name: "Use a username instead" })).toBeDisabled();
    await expect(canvas.getByRole("button", { name: "Sign in with a passkey" })).toHaveAttribute(
      "aria-busy",
      "true",
    );
  },
};
