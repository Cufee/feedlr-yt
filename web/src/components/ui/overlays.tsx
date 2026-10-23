import {
  Description,
  DialogBackdrop,
  DialogPanel,
  DialogTitle,
  Dialog as HeadlessDialog,
  Menu as HeadlessMenu,
  MenuButton,
  MenuItem,
  MenuItems,
  Portal,
} from "@headlessui/react";
import {
  CheckIcon,
  ChevronDownIcon,
  EllipsisHorizontalIcon,
  XMarkIcon,
} from "@heroicons/react/20/solid";
import type { ButtonHTMLAttributes, HTMLAttributes, ReactNode, Ref, RefObject } from "react";
import { useOverlayPortal } from "@/lib/browser/overlay-portal";
import { IconButton } from "./actions";

export type DialogProps = Omit<HTMLAttributes<HTMLDivElement>, "title" | "role"> & {
  role?: "dialog" | "alertdialog";
  open: boolean;
  onClose: () => void;
  title: ReactNode;
  description?: ReactNode;
  footer?: ReactNode;
  portalTarget?: HTMLElement | null;
  initialFocus?: RefObject<HTMLElement | null>;
  ref?: Ref<HTMLDivElement>;
};
export function Dialog({
  open,
  onClose,
  title,
  description,
  children,
  footer,
  portalTarget,
  initialFocus,
  className = "",
  ...props
}: DialogProps) {
  useOverlayPortal(open, portalTarget);
  return (
    <HeadlessDialog
      {...props}
      open={open}
      onClose={onClose}
      initialFocus={initialFocus}
      className={`ui-dialog ${className}`}
    >
      <DialogBackdrop transition className="ui-dialog-backdrop" />
      <div className="ui-dialog-position">
        <DialogPanel transition className="ui-dialog-panel">
          <div className="ui-dialog-header">
            <div>
              <DialogTitle className="text-lg font-semibold tracking-tight">{title}</DialogTitle>
              {description && (
                <Description className="mt-1 text-sm leading-6 text-text-muted">
                  {description}
                </Description>
              )}
            </div>
            <IconButton label="Close dialog" variant="ghost" onClick={onClose} data-autofocus>
              <XMarkIcon className="size-5" />
            </IconButton>
          </div>
          <div className="ui-dialog-content">{children}</div>
          {footer && <div className="ui-dialog-footer">{footer}</div>}
        </DialogPanel>
      </div>
    </HeadlessDialog>
  );
}

export type MenuAction = {
  id: string;
  label: ReactNode;
  onSelect?: () => void;
  href?: string;
  disabled?: boolean;
  danger?: boolean;
  icon?: ReactNode;
  checked?: boolean;
  pending?: boolean;
};
export type MenuProps = {
  label: ReactNode;
  items: MenuAction[];
  className?: string;
  buttonClassName?: string;
  iconOnly?: boolean;
  portalTarget?: HTMLElement | null;
  ref?: Ref<HTMLButtonElement>;
};
function MenuCommand({
  checked,
  ...props
}: ButtonHTMLAttributes<HTMLButtonElement> & {
  checked?: boolean;
  ref?: Ref<HTMLButtonElement>;
}) {
  if (checked === undefined) {
    return <button {...props} role="menuitem" />;
  }
  return <button {...props} role="menuitemcheckbox" aria-checked={checked} />;
}
export function Menu({
  label,
  items,
  className = "",
  buttonClassName = "",
  iconOnly = false,
  portalTarget,
  ref,
}: MenuProps) {
  return (
    <Portal.Group target={{ current: portalTarget ?? document.body }}>
      <HeadlessMenu as="div" className={`relative inline-block ${className}`}>
        <MenuButton
          ref={ref}
          aria-label={iconOnly && typeof label === "string" ? label : undefined}
          className={`ui-button ui-button-secondary ui-button-md ${iconOnly ? "ui-icon-button" : ""} ${buttonClassName}`}
        >
          {iconOnly ? (
            <EllipsisHorizontalIcon aria-hidden="true" />
          ) : (
            <>
              {label}
              <ChevronDownIcon className="size-4 text-text-muted" aria-hidden="true" />
            </>
          )}
        </MenuButton>
        <MenuItems
          transition
          modal={false}
          portal={portalTarget !== undefined}
          anchor={portalTarget !== undefined ? { to: "bottom end", gap: 8 } : undefined}
          className={`ui-menu${portalTarget !== undefined ? " ui-menu-floating" : ""}`}
        >
          {items.map((item) => (
            <MenuItem key={item.id} disabled={item.disabled || item.pending}>
              {item.href ? (
                <a
                  href={item.href}
                  className={`ui-menu-item ${item.danger ? "ui-menu-item-danger" : ""}`}
                  onClick={item.onSelect}
                >
                  {item.icon}
                  {item.label}
                </a>
              ) : (
                <MenuCommand
                  type="button"
                  checked={item.checked}
                  aria-busy={item.pending || undefined}
                  className={`ui-menu-item ${item.danger ? "ui-menu-item-danger" : ""}`}
                  onClick={item.onSelect}
                >
                  {item.icon}
                  {item.label}
                  {item.pending ? (
                    <span className="ui-spinner ui-pending" aria-hidden="true" />
                  ) : item.checked ? (
                    <CheckIcon aria-hidden="true" />
                  ) : null}
                </MenuCommand>
              )}
            </MenuItem>
          ))}
        </MenuItems>
      </HeadlessMenu>
    </Portal.Group>
  );
}
