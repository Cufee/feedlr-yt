import { Button, Dialog } from "@/components/ui";

export function ConfirmDialog({
  open,
  onClose,
  title,
  description,
  confirmLabel = "Delete",
  pending = false,
  error,
  onConfirm,
}: {
  open: boolean;
  onClose: () => void;
  title: string;
  description: string;
  confirmLabel?: string;
  pending?: boolean;
  error?: Error | null;
  onConfirm: () => void;
}) {
  return (
    <Dialog
      open={open}
      onClose={pending ? () => undefined : onClose}
      title={title}
      description={description}
      footer={
        <>
          <Button onClick={onClose} disabled={pending} autoFocus>
            Cancel
          </Button>
          <Button variant="danger" pending={pending} onClick={onConfirm}>
            {confirmLabel}
          </Button>
        </>
      }
    >
      {error && (
        <p className="form-error" role="alert">
          {error.message}
        </p>
      )}
    </Dialog>
  );
}
