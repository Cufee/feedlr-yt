import {
  CheckCircleIcon,
  ExclamationCircleIcon,
  InformationCircleIcon,
  XMarkIcon,
} from "@heroicons/react/20/solid";
import type { HTMLAttributes, ReactNode, Ref } from "react";
import { IconButton } from "./actions";

export type Tone = "neutral" | "info" | "accent" | "success" | "warning" | "danger";
export type BadgeProps = HTMLAttributes<HTMLSpanElement> & {
  tone?: Tone;
  ref?: Ref<HTMLSpanElement>;
};
export function Badge({ tone = "neutral", className = "", ...props }: BadgeProps) {
  return <span {...props} className={`ui-badge ui-tone-${tone} ${className}`} />;
}
export type ToastProps = HTMLAttributes<HTMLDivElement> & {
  tone?: Tone;
  title?: string;
  onDismiss?: () => void;
  ref?: Ref<HTMLDivElement>;
};
export function Toast({
  tone = "neutral",
  title,
  children,
  onDismiss,
  className = "",
  ...props
}: ToastProps) {
  const Icon =
    tone === "success"
      ? CheckCircleIcon
      : tone === "danger" || tone === "warning"
        ? ExclamationCircleIcon
        : InformationCircleIcon;
  return (
    <div
      {...props}
      role={props.role ?? (tone === "danger" ? "alert" : "status")}
      className={`ui-toast ui-tone-${tone} ${className}`}
    >
      <Icon className="ui-toast-icon" aria-hidden="true" />
      <div className="ui-toast-copy">
        {title && <p className="font-semibold">{title}</p>}
        <div>{children}</div>
      </div>
      {onDismiss && (
        <IconButton label="Dismiss notification" variant="ghost" size="sm" onClick={onDismiss}>
          <XMarkIcon className="size-4" />
        </IconButton>
      )}
    </div>
  );
}
export function Skeleton({ className = "", ...props }: HTMLAttributes<HTMLDivElement>) {
  return <div {...props} aria-hidden="true" className={`ui-skeleton ${className}`} />;
}
export type ProgressProps = Omit<HTMLAttributes<HTMLDivElement>, "children"> & {
  value?: number;
  max?: number;
  label: string;
};
export function Progress({ value, max = 100, label, className = "", ...props }: ProgressProps) {
  const safeMax = Number.isFinite(max) && max > 0 ? max : 100;
  const safeValue =
    value === undefined || !Number.isFinite(value)
      ? undefined
      : Math.max(0, Math.min(value, safeMax));
  return (
    <div
      {...props}
      role="progressbar"
      aria-label={label}
      aria-valuemin={0}
      aria-valuemax={safeMax}
      aria-valuenow={safeValue}
      className={`ui-progress ${className}`}
    >
      <span
        className={
          safeValue === undefined
            ? "ui-progress-fill ui-progress-indeterminate"
            : "ui-progress-fill"
        }
        style={safeValue === undefined ? undefined : { width: `${(safeValue / safeMax) * 100}%` }}
      />
    </div>
  );
}
export type EmptyStateProps = HTMLAttributes<HTMLDivElement> & {
  title: string;
  description?: ReactNode;
  icon?: ReactNode;
  action?: ReactNode;
};
export function EmptyState({
  title,
  description,
  icon,
  action,
  className = "",
  ...props
}: EmptyStateProps) {
  return (
    <div {...props} className={`ui-empty-state ${className}`}>
      {icon && (
        <div className="ui-empty-icon" aria-hidden="true">
          {icon}
        </div>
      )}
      <h2 className="text-lg font-semibold tracking-tight">{title}</h2>
      {description && (
        <p className="mt-2 max-w-md text-sm leading-6 text-text-muted">{description}</p>
      )}
      {action && <div className="mt-5">{action}</div>}
    </div>
  );
}
