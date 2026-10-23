import { createContext, type ReactNode, useContext, useRef, useState } from "react";
import { Toast } from "@/components/ui";
import { useToastTimer } from "@/lib/browser/useToastTimer";

type Notice = { id: number; message: string; tone: "success" | "danger" | "info" };
type Notify = (message: string, tone?: Notice["tone"]) => void;
const Notifications = createContext<Notify>(() => undefined);

function Notification({ notice, dismiss }: { notice: Notice; dismiss: () => void }) {
  const lifetime = useToastTimer(notice.tone === "danger" ? 10_000 : 6_000, dismiss);
  return (
    <Toast {...lifetime} tone={notice.tone} onDismiss={dismiss}>
      {notice.message}
    </Toast>
  );
}

export function NotificationProvider({ children }: { children: ReactNode }) {
  const [notices, setNotices] = useState<Notice[]>([]);
  const nextID = useRef(0);
  function notify(message: string, tone: Notice["tone"] = "success") {
    const notice = { id: ++nextID.current, message, tone };
    setNotices((current) => [...current.slice(-2), notice]);
  }
  return (
    <Notifications value={notify}>
      {children}
      <section className="notification-stack" aria-label="Notifications">
        {notices.map((notice) => (
          <Notification
            key={notice.id}
            notice={notice}
            dismiss={() => setNotices((current) => current.filter((item) => item.id !== notice.id))}
          />
        ))}
      </section>
    </Notifications>
  );
}
export function useNotify() {
  return useContext(Notifications);
}
