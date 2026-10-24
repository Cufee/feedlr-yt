import type { Meta, StoryObj } from "@storybook/react-vite";
import { StrictMode, useState } from "react";
import { toast } from "sonner";
import { expect, userEvent, waitFor, within } from "storybook/test";
import { Notifications } from "@/app/Notifications";
import { Button, Dialog, Field, Input, Menu, Tabs } from "./index";

function OverlayScenario({ nested = false }: { nested?: boolean }) {
  const [open, setOpen] = useState(false);
  const [target, setTarget] = useState<HTMLDivElement | null>(null);
  return (
    <div ref={setTarget} className="panel mx-auto max-w-2xl space-y-5 p-6" data-fullscreen-host>
      <h1 className="text-xl font-semibold">A focused player action</h1>
      <div data-background-controls>
        <Button onClick={() => setOpen(true)}>Open playlist dialog</Button>
      </div>
      <Dialog
        open={open}
        onClose={() => setOpen(false)}
        title="Save to playlist"
        description="Choose where this episode belongs."
        portalTarget={nested ? target : undefined}
        footer={
          <Button
            variant="primary"
            onClick={() => {
              toast.success("Episode saved.");
              setOpen(false);
            }}
          >
            Save episode
          </Button>
        }
      >
        <Field label="Playlist">
          <Input defaultValue="Long walks" />
        </Field>
      </Dialog>
    </div>
  );
}

const meta = {
  title: "Foundation/Overlays",
  component: OverlayScenario,
  tags: ["autodocs"],
  decorators: [
    (Story) => (
      <StrictMode>
        <Story />
        <Notifications />
      </StrictMode>
    ),
  ],
} satisfies Meta<typeof OverlayScenario>;
export default meta;
type Story = StoryObj<typeof meta>;
export const DialogFocusAndDismiss: Story = {
  play: async ({ canvasElement }) => {
    const canvas = within(canvasElement);
    const trigger = canvas.getByRole("button", { name: "Open playlist dialog" });
    await userEvent.click(trigger);
    const dialog = await within(document.body).findByRole("dialog", { name: "Save to playlist" });
    await expect(dialog).toHaveAccessibleDescription("Choose where this episode belongs.");
    await waitFor(() =>
      expect(within(dialog).getByRole("button", { name: "Close dialog" })).toHaveFocus(),
    );
    await userEvent.keyboard("{Escape}");
    await waitFor(() => expect(trigger).toHaveFocus());
    await expect(within(document.body).queryByRole("dialog")).not.toBeInTheDocument();
  },
};
export const FullscreenIsolationRegression: Story = {
  args: { nested: true },
  play: async ({ canvasElement }) => {
    const canvas = within(canvasElement);
    const trigger = canvas.getByRole("button", { name: "Open playlist dialog" });
    await userEvent.click(trigger);
    const dialog = await within(document.body).findByRole("dialog", { name: "Save to playlist" });
    await waitFor(() => {
      for (let ancestor: HTMLElement | null = dialog; ancestor; ancestor = ancestor.parentElement) {
        expect(ancestor.inert).toBe(false);
        expect(ancestor).not.toHaveAttribute("aria-hidden", "true");
      }
      expect(canvasElement.querySelector("[data-background-controls]")).toHaveAttribute("inert");
    });
    await userEvent.type(within(dialog).getByRole("textbox", { name: "Playlist" }), " and trails");
    await userEvent.click(within(dialog).getByRole("button", { name: "Save episode" }));
    await waitFor(() => expect(within(document.body).getByText("Episode saved.")).toBeVisible());
    await waitFor(() => expect(trigger).toHaveFocus());
    await expect(canvasElement.querySelector("[data-background-controls]")).not.toHaveAttribute(
      "inert",
    );
    await userEvent.click(trigger);
    await expect(
      await within(document.body).findByRole("dialog", { name: "Save to playlist" }),
    ).toBeVisible();
    await userEvent.keyboard("{Escape}");
    await waitFor(() => expect(trigger).toHaveFocus());
  },
};

function MenuScenario() {
  const [message, setMessage] = useState("No action selected");
  return (
    <div className="flex min-h-64 items-start justify-center gap-8">
      <Menu
        label="Episode actions"
        items={[
          { id: "save", label: "Save episode", onSelect: () => setMessage("Episode saved") },
          { id: "move", label: "Move episode", disabled: true },
          {
            id: "remove",
            label: "Remove episode",
            danger: true,
            onSelect: () => setMessage("Episode removed"),
          },
        ]}
      />
      <p role="status">{message}</p>
    </div>
  );
}
export const MenuKeyboard: Story = {
  render: () => <MenuScenario />,
  play: async ({ canvasElement }) => {
    const canvas = within(canvasElement);
    const trigger = canvas.getByRole("button", { name: "Episode actions" });
    await userEvent.click(trigger);
    await userEvent.keyboard("{ArrowDown}{ArrowDown}{Enter}");
    await expect(canvas.getByRole("status")).toHaveTextContent("Episode removed");
    await waitFor(() => expect(trigger).toHaveFocus());
  },
};

function PersistentMenuDialog() {
  const [open, setOpen] = useState(false);
  const [target, setTarget] = useState<HTMLDivElement | null>(null);
  return (
    <>
      <Menu
        label="Playlist options"
        items={[{ id: "edit", label: "Edit playlist", onSelect: () => setOpen(true) }]}
      />
      <Dialog ref={setTarget} open={open} onClose={() => setOpen(false)} title="Playlist settings">
        <Menu
          portalTarget={target}
          label="View options"
          items={[{ id: "keep", label: "Keep editing" }]}
        />
        <Button onClick={() => setOpen(false)}>Done</Button>
      </Dialog>
    </>
  );
}

export const PersistentDialogMenu: Story = { render: () => <PersistentMenuDialog /> };

export const TabsKeyboard: Story = {
  render: () => (
    <Tabs
      label="Episode details"
      items={[
        { id: "notes", label: "Episode notes", content: "Thoughtfully edited notes." },
        { id: "disabled", label: "Unavailable", content: "Hidden", disabled: true },
        { id: "segments", label: "Episode segments", content: "Sponsor · 2:10–2:45" },
      ]}
    />
  ),
  play: async ({ canvasElement }) => {
    const canvas = within(canvasElement);
    await userEvent.click(canvas.getByRole("tab", { name: "Episode notes" }));
    await userEvent.keyboard("{ArrowRight}");
    await expect(canvas.getByRole("tab", { name: "Episode segments" })).toHaveAttribute(
      "aria-selected",
      "true",
    );
    await expect(canvas.getByRole("tabpanel")).toHaveTextContent("Sponsor · 2:10–2:45");
  },
};

function CheckedMenuScenario() {
  const [target, setTarget] = useState<HTMLDivElement | null>(null);
  const [saved, setSaved] = useState(false);
  return (
    <div ref={setTarget} className="panel min-h-64 p-6" data-menu-host>
      <Menu
        label="Playback actions"
        portalTarget={target}
        items={[
          { id: "saved", label: "Watch Later", checked: saved, onSelect: () => setSaved(!saved) },
          { id: "pending", label: "Sending to TV", pending: true },
        ]}
      />
    </div>
  );
}

export const CheckedMenuInFullscreenHost: Story = {
  render: () => <CheckedMenuScenario />,
  play: async ({ canvasElement }) => {
    const canvas = within(canvasElement);
    const trigger = canvas.getByRole("button", { name: "Playback actions" });
    await userEvent.click(trigger);
    const saved = await canvas.findByRole("menuitemcheckbox", { name: "Watch Later" });
    await expect(saved).toHaveAttribute("aria-checked", "false");
    await expect(canvas.getByRole("menuitem", { name: "Sending to TV" })).toHaveAttribute(
      "aria-disabled",
      "true",
    );
    await userEvent.keyboard("{ArrowDown}{Enter}");
    await waitFor(() => expect(trigger).toHaveFocus());
    await userEvent.click(trigger);
    await expect(
      await canvas.findByRole("menuitemcheckbox", { name: "Watch Later" }),
    ).toHaveAttribute("aria-checked", "true");
    await userEvent.keyboard("{Escape}");
    await waitFor(() => expect(trigger).toHaveFocus());
  },
};
