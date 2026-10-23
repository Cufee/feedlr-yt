import { BookmarkIcon, PlusIcon } from "@heroicons/react/20/solid";
import type { Meta, StoryObj } from "@storybook/react-vite";
import { useState } from "react";
import { expect, userEvent, waitFor, within } from "storybook/test";
import {
  Badge,
  Button,
  Checkbox,
  Field,
  IconButton,
  Input,
  Link,
  Select,
  Switch,
  Textarea,
} from "./index";

function Controls({
  invalid = false,
  disabled = false,
}: {
  invalid?: boolean;
  disabled?: boolean;
}) {
  const [skip, setSkip] = useState(true);
  const [stream, setStream] = useState(false);
  const [saved, setSaved] = useState(false);
  const [pending, setPending] = useState(false);
  return (
    <main className="mx-auto max-w-5xl space-y-8 p-2">
      <header>
        <Badge tone="accent">Feedlr foundations</Badge>
        <h1 className="mt-3 text-2xl font-semibold tracking-tight">
          Small details. A calmer feed.
        </h1>
        <p className="mt-2 text-text-muted">
          Consistent controls, clear state and room to breathe.
        </p>
      </header>
      <div className="grid gap-8 md:grid-cols-2">
        <section className="panel space-y-5 p-5" aria-label="Form controls">
          <h2 className="text-lg font-semibold">Your collection</h2>
          <Field
            label="Playlist name"
            description="A short, memorable name."
            error={invalid ? "Enter a playlist name." : undefined}
            required
          >
            <Input required placeholder="Weekend listening" disabled={disabled} />
          </Field>
          <Field label="Description">
            <Textarea placeholder="What belongs in this playlist?" disabled={disabled} />
          </Field>
          <Field label="Source">
            <Select disabled={disabled} defaultValue="all">
              <option value="all">All media</option>
              <option value="youtube">YouTube</option>
              <option value="podcasts">Podcasts</option>
            </Select>
          </Field>
        </section>
        <section className="panel space-y-6 p-5" aria-label="Actions and preferences">
          <h2 className="text-lg font-semibold">Ready when you are</h2>
          <div className="flex flex-wrap gap-3">
            <Button variant="primary" disabled={disabled}>
              <PlusIcon aria-hidden="true" />
              Create playlist
            </Button>
            <Button disabled={disabled}>Cancel</Button>
            <Button variant="ghost" disabled={disabled}>
              Preview
            </Button>
            <Button variant="danger" disabled={disabled}>
              Delete
            </Button>
          </div>
          <div className="flex items-center gap-3">
            <IconButton
              label={saved ? "Remove from Watch Later" : "Save to Watch Later"}
              aria-pressed={saved}
              disabled={disabled}
              onClick={() => setSaved(!saved)}
            >
              <BookmarkIcon aria-hidden="true" />
            </IconButton>
            <Link href="#controls-help">Keyboard shortcuts</Link>
          </div>
          <Switch
            label="Smart skipping"
            description="Skip selected moments in podcast episodes."
            checked={skip}
            onChange={setSkip}
            disabled={disabled}
          />
          <Checkbox
            label="Include live streams"
            description="Show streams alongside uploaded videos."
            checked={stream}
            onChange={setStream}
            disabled={disabled}
          />
          <Button pending={pending} onClick={() => setPending(true)}>
            Save preferences
          </Button>
          <p id="controls-help" className="ui-help">
            Every action supports the keyboard. Saving keeps its focused control in place.
          </p>
        </section>
      </div>
    </main>
  );
}

const meta = {
  title: "Foundation/Controls",
  component: Controls,
  tags: ["autodocs"],
} satisfies Meta<typeof Controls>;
export default meta;
type Story = StoryObj<typeof meta>;
export const Desktop: Story = { globals: { viewport: { value: "desktop" } } };
export const Mobile: Story = { globals: { viewport: { value: "phone" } } };
export const FormErrors: Story = { args: { invalid: true } };
export const Disabled: Story = { args: { disabled: true } };
export const KeyboardAndPending: Story = {
  play: async ({ canvasElement }) => {
    const canvas = within(canvasElement);
    const name = canvas.getByRole("textbox", { name: "Playlist name" });
    await userEvent.type(name, "Long walks");
    await expect(name).toHaveAccessibleDescription("A short, memorable name.");
    await userEvent.selectOptions(canvas.getByRole("combobox", { name: "Source" }), "podcasts");
    await expect(canvas.getByRole("combobox", { name: "Source" })).toHaveValue("podcasts");
    const skip = canvas.getByRole("switch", { name: "Smart skipping" });
    skip.focus();
    await userEvent.keyboard(" ");
    await expect(skip).toHaveAttribute("aria-checked", "false");
    const checkbox = canvas.getByRole("checkbox", { name: "Include live streams" });
    await userEvent.click(checkbox);
    await expect(checkbox).toHaveAttribute("aria-checked", "true");
    const save = canvas.getByRole("button", { name: "Save preferences" });
    const before = save.getBoundingClientRect();
    await userEvent.click(save);
    const label = canvas.getByText("Save preferences");
    await expect(label).toHaveStyle({ opacity: "1" });
    await expect(save).toHaveFocus();
    await expect(save).toHaveAttribute("aria-busy", "true");
    await expect(save.getBoundingClientRect().width).toBe(before.width);
    await waitFor(() => expect(label).toHaveStyle({ opacity: "0" }));
    await userEvent.click(save);
    await expect(canvas.getByRole("button", { name: "Save preferences" })).toBe(save);
  },
};
