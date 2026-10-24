import type { Preview } from "@storybook/react-vite";
import { mswLoader } from "msw-storybook-addon/csf3";
import { toast } from "sonner";
import "../src/styles/index.css";

const preview: Preview = {
  beforeEach: () => {
    toast.dismiss();
    return () => {
      toast.dismiss();
    };
  },
  loaders: [
    mswLoader(async () => {
      const { setupWorker } = await import("msw/browser");
      const worker = setupWorker();
      await worker.start({
        quiet: true,
        serviceWorker: { url: "./mockServiceWorker.js" },
        onUnhandledFrame({ frame, defaults }) {
          const data = frame.data;
          if (
            data &&
            typeof data === "object" &&
            "request" in data &&
            data.request instanceof Request &&
            /^\/(api|login|register)\//.test(new URL(data.request.url).pathname)
          ) {
            defaults.error();
          }
        },
      });
      return worker;
    }),
  ],
  parameters: {
    layout: "padded",
    backgrounds: {
      options: {
        canvas: { name: "Canvas", value: "#101014" },
        surface: { name: "Surface", value: "#1c1c23" },
      },
    },
    a11y: { test: "error" },
    controls: { matchers: { color: /(background|color)$/i, date: /Date$/i } },
    viewport: {
      options: {
        phone: {
          name: "Phone · 390 × 844",
          styles: { width: "390px", height: "844px" },
          type: "mobile",
        },
        tablet: {
          name: "Tablet · 768 × 1024",
          styles: { width: "768px", height: "1024px" },
          type: "tablet",
        },
        desktop: {
          name: "Desktop · 1440 × 1000",
          styles: { width: "1440px", height: "1000px" },
          type: "desktop",
        },
      },
    },
  },
  initialGlobals: { backgrounds: { value: "canvas" } },
};
export default preview;
