import { type FormEvent, useState } from "react";
import { Button, Dialog, Field, Input } from "@/components/ui";
import { useAction } from "@/lib/api/queries";

export function URLDialog({
  open,
  onClose,
  title,
  description,
  label,
  submitLabel,
  onSubmit,
  onSuccess,
}: {
  open: boolean;
  onClose: () => void;
  title: string;
  description?: string;
  label: string;
  submitLabel: string;
  onSubmit: (url: string) => Promise<unknown>;
  onSuccess?: () => void;
}) {
  const [url, setUrl] = useState("");
  const mutation = useAction(onSubmit, {
    onSuccess: () => {
      onSuccess?.();
      onClose();
    },
  });
  function submit(event: FormEvent) {
    event.preventDefault();
    mutation.mutate(url.trim());
  }
  return (
    <Dialog open={open} onClose={onClose} title={title} description={description}>
      <form className="form-stack" onSubmit={submit}>
        <Field label={label} htmlFor="media-url" error={mutation.error?.message}>
          <Input
            id="media-url"
            type="url"
            inputMode="url"
            value={url}
            onChange={(event) => setUrl(event.target.value)}
            placeholder="https://youtube.com/…"
            required
            autoFocus
          />
        </Field>
        <div className="form-actions">
          <Button variant="ghost" onClick={onClose}>
            Cancel
          </Button>
          <Button type="submit" variant="primary" pending={mutation.isPending}>
            {submitLabel}
          </Button>
        </div>
      </form>
    </Dialog>
  );
}
