import { createPortal } from "react-dom";
import { Toaster } from "sonner";
import { useFullscreenPortal } from "@/lib/browser/useFullscreenPortal";

export function Notifications() {
  const portal = useFullscreenPortal();
  return createPortal(
    <Toaster
      theme="dark"
      richColors
      closeButton
      duration={6_000}
      mobileOffset={{ bottom: "calc(88px + env(safe-area-inset-bottom))", left: 16, right: 16 }}
      toastOptions={{ closeButtonAriaLabel: "Dismiss notification" }}
    />,
    portal,
  );
}
