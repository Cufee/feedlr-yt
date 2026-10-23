import { QueryClientProvider } from "@tanstack/react-query";
import { RouterProvider } from "@tanstack/react-router";
import { StrictMode } from "react";
import { createRoot } from "react-dom/client";
import { NotificationProvider } from "@/app/Notifications";
import { router } from "@/app/router";
import { queryClient } from "@/lib/api/queries";
import "@/styles/index.css";
import "@/app/app.css";

const root = document.getElementById("root");
if (!root) {
  throw new Error("Feedlr could not find its application root.");
}
// Initial document metadata serves crawlers and link previews. React 19 owns
// these tags after boot and updates them declaratively as SPA routes change.
for (const tag of document.head.querySelectorAll("[data-feedlr-head]")) {
  tag.remove();
}
createRoot(root).render(
  <StrictMode>
    <QueryClientProvider client={queryClient}>
      <NotificationProvider>
        <RouterProvider router={router} />
      </NotificationProvider>
    </QueryClientProvider>
  </StrictMode>,
);
