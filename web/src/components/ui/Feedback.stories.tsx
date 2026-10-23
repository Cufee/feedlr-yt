import { BookmarkIcon } from "@heroicons/react/24/outline";
import type { Meta, StoryObj } from "@storybook/react-vite";
import { useState } from "react";
import { expect, userEvent, within } from "storybook/test";
import { Badge, Button, EmptyState, Progress, Skeleton, Toast } from "./index";

function Feedback() {
  const [visible, setVisible] = useState(true);
  return (
    <main className="mx-auto max-w-5xl space-y-8 p-2">
      <h1 className="text-2xl font-semibold">Useful feedback, without the noise.</h1>
      <div className="flex flex-wrap gap-2">
        {(["neutral", "accent", "success", "warning", "danger"] as const).map((tone) => (
          <Badge key={tone} tone={tone}>
            {tone}
          </Badge>
        ))}
      </div>
      <div className="grid gap-6 md:grid-cols-2">
        <section className="space-y-4" aria-label="Notifications">
          {visible && (
            <Toast title="Playlist updated" tone="success" onDismiss={() => setVisible(false)}>
              Your episode is ready for later.
            </Toast>
          )}
          <Toast tone="danger">Could not save this change. Try again.</Toast>
          <Toast tone="info">Podcast catalog search is unavailable. Add an RSS URL instead.</Toast>
        </section>
        <section className="panel space-y-6 p-5" aria-label="Loading feedback">
          <Skeleton className="aspect-video" />
          <div>
            <p className="ui-help mb-2">Episode progress</p>
            <Progress value={37} label="Episode progress" />
          </div>
          <div>
            <p className="ui-help mb-2">Loading your collection</p>
            <Progress label="Loading your collection" />
          </div>
        </section>
      </div>
      <div className="panel">
        <EmptyState
          title="A little room for later"
          description="Save a video or episode and it will be waiting here."
          icon={<BookmarkIcon />}
          action={<Button variant="primary">Explore subscriptions</Button>}
        />
      </div>
    </main>
  );
}
const meta = {
  title: "Foundation/Feedback",
  component: Feedback,
  tags: ["autodocs"],
} satisfies Meta<typeof Feedback>;
export default meta;
type Story = StoryObj<typeof meta>;
export const Desktop: Story = { globals: { viewport: { value: "desktop" } } };
export const Mobile: Story = { globals: { viewport: { value: "phone" } } };
export const DismissNotice: Story = {
  play: async ({ canvasElement }) => {
    const canvas = within(canvasElement);
    await userEvent.click(canvas.getByRole("button", { name: "Dismiss notification" }));
    await expect(canvas.queryByText("Your episode is ready for later.")).not.toBeInTheDocument();
    await expect(canvas.getByRole("alert")).toHaveTextContent(
      "Could not save this change. Try again.",
    );
  },
};
