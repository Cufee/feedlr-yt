import { useNavigate } from "@tanstack/react-router";
import { type FormEvent, useState } from "react";
import { Button, Dialog, Field, Input } from "@/components/ui";

export function youtubeId(value: string): string | null {
  if (/^[A-Za-z0-9_-]{11}$/.test(value.trim())) {
    return value.trim();
  }
  try {
    const url = new URL(value.trim());
    if (!["http:", "https:"].includes(url.protocol)) {
      return null;
    }
    if (
      url.hostname !== "youtu.be" &&
      url.hostname !== "youtube.com" &&
      !url.hostname.endsWith(".youtube.com")
    ) {
      return null;
    }
    const id =
      url.hostname === "youtu.be"
        ? url.pathname.split("/")[1]
        : (url.searchParams.get("v") ?? url.pathname.split("/")[2]);
    return id && /^[A-Za-z0-9_-]{11}$/.test(id) ? id : null;
  } catch {
    return null;
  }
}
export function OpenMediaDialog({ open, onClose }: { open: boolean; onClose: () => void }) {
  const [url, setUrl] = useState("");
  const [error, setError] = useState("");
  const navigate = useNavigate();
  function submit(event: FormEvent) {
    event.preventDefault();
    const id = youtubeId(url);
    if (!id) {
      setError("Enter a YouTube video link or video ID.");
      return;
    }
    onClose();
    void navigate({ to: "/video/$mediaId", params: { mediaId: id }, search: { return: "/app" } });
  }
  return (
    <Dialog open={open} onClose={onClose} title="Open a video">
      <form className="form-stack" onSubmit={submit}>
        <Field label="YouTube link" htmlFor="open-media-url" error={error || undefined}>
          <Input
            id="open-media-url"
            autoFocus
            type="text"
            inputMode="url"
            value={url}
            onChange={(event) => {
              setUrl(event.target.value);
              setError("");
            }}
            placeholder="https://youtube.com/watch?v=…"
            required
            aria-invalid={!!error}
          />
        </Field>
        <div className="form-actions">
          <Button onClick={onClose} variant="ghost">
            Cancel
          </Button>
          <Button type="submit" variant="primary">
            Open video
          </Button>
        </div>
      </form>
    </Dialog>
  );
}
