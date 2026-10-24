import { useEffect } from "react";
import { toast } from "sonner";

export function usePlaybackNotifications({
  notice,
  noticeID,
  noticeTone,
  tvSending,
}: {
  notice: string;
  noticeID: number;
  noticeTone: "success" | "error" | "info";
  tvSending: boolean;
}) {
  useEffect(() => {
    if (!notice || noticeID === 0) {
      return;
    }
    const id = toast[noticeTone](notice, {
      duration: noticeTone === "error" ? 10_000 : 6_000,
    });
    return () => {
      toast.dismiss(id);
    };
  }, [notice, noticeID, noticeTone]);
  useEffect(() => {
    if (!tvSending) {
      return;
    }
    const id = toast.loading("Sending to TV…");
    return () => {
      toast.dismiss(id);
    };
  }, [tvSending]);
}
