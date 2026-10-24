// Establish the global layer order before router imports can load feature styles.
import "@/styles/index.css";
import "@/app/app.css";
import { QueryClientProvider } from "@tanstack/react-query";
import { RouterProvider } from "@tanstack/react-router";
import { StrictMode } from "react";
import { createRoot } from "react-dom/client";
import { Notifications } from "@/app/Notifications";
import { router } from "@/app/router";
import { queryClient } from "@/lib/api/queries";

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
      <Notifications />
      <RouterProvider router={router} />
    </QueryClientProvider>
  </StrictMode>,
);
