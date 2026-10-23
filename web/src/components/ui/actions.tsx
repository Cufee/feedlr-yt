import type { AnchorHTMLAttributes, ButtonHTMLAttributes, Ref } from "react";

export type ButtonVariant = "primary" | "secondary" | "ghost" | "danger";
export type ButtonSize = "sm" | "md" | "lg";
export function buttonClassName({
  variant = "secondary",
  size = "md",
}: {
  variant?: ButtonVariant;
  size?: ButtonSize;
} = {}) {
  return `ui-button ui-button-${variant} ui-button-${size}`;
}
export type ButtonProps = ButtonHTMLAttributes<HTMLButtonElement> & {
  ref?: Ref<HTMLButtonElement>;
  variant?: ButtonVariant;
  size?: ButtonSize;
  pending?: boolean;
};

export function Button({
  children,
  className = "",
  variant = "secondary",
  size = "md",
  pending = false,
  disabled,
  type = "button",
  onClick,
  ...props
}: ButtonProps) {
  return (
    <button
      {...props}
      type={type}
      disabled={disabled}
      aria-busy={pending || props["aria-busy"] || undefined}
      data-pending={pending || undefined}
      aria-disabled={pending || disabled || props["aria-disabled"] || undefined}
      onClick={(event) => {
        if (pending) {
          event.preventDefault();
          return;
        }
        onClick?.(event);
      }}
      className={`${buttonClassName({ variant, size })} ${className}`}
    >
      {pending && <span className="ui-spinner ui-pending" aria-hidden="true" />}
      <span className="ui-button-label">{children}</span>
    </button>
  );
}

export type IconButtonProps = Omit<ButtonProps, "aria-label"> & { label: string };
export function IconButton({ label, className = "", ...props }: IconButtonProps) {
  return (
    <Button
      {...props}
      className={`ui-icon-button ${className}`}
      aria-label={label}
      title={props.title ?? label}
    />
  );
}

export type LinkProps = AnchorHTMLAttributes<HTMLAnchorElement> & { ref?: Ref<HTMLAnchorElement> };
export function Link({ className = "", ...props }: LinkProps) {
  return <a {...props} className={`ui-link ${className}`} />;
}
