import { BookmarkIcon } from "@heroicons/react/24/outline";
import type { Meta, StoryObj } from "@storybook/react-vite";
import { toast } from "sonner";
import { expect, userEvent, waitFor, within } from "storybook/test";
import { Notifications } from "@/app/Notifications";
import { Badge, Button, EmptyState, Progress, Skeleton } from "./index";

function Feedback() {
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
        <section className="flex flex-wrap items-start gap-3" aria-label="Notification examples">
          <Button
            onClick={() =>
              toast.success("Playlist updated", {
                description: "Your episode is ready for later.",
                duration: Infinity,
              })
            }
          >
            Show success notification
          </Button>
          <Button
            onClick={() =>
              toast.error("Could not save this change. Try again.", { duration: Infinity })
            }
          >
            Show error notification
          </Button>
          <Button
            onClick={() =>
              toast.info("Podcast catalog search is unavailable. Add an RSS URL instead.", {
                duration: Infinity,
              })
            }
          >
            Show info notification
          </Button>
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
      <Notifications />
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
    const document = within(canvasElement.ownerDocument.body);
    await userEvent.click(canvas.getByRole("button", { name: "Show success notification" }));
    await waitFor(() =>
      expect(document.getByText("Your episode is ready for later.")).toBeVisible(),
    );
    await userEvent.click(document.getByRole("button", { name: "Dismiss notification" }));
    await waitFor(() =>
      expect(document.queryByText("Your episode is ready for later.")).not.toBeInTheDocument(),
    );
    await userEvent.click(canvas.getByRole("button", { name: "Show error notification" }));
    await waitFor(() =>
      expect(
        canvasElement.ownerDocument.querySelector('[data-sonner-toast][data-type="error"]'),
      ).toHaveTextContent("Could not save this change. Try again."),
    );
  },
};
