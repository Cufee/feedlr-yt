import {
  Description,
  Checkbox as HeadlessCheckbox,
  Field as HeadlessField,
  Switch as HeadlessSwitch,
  Label,
} from "@headlessui/react";
import { CheckIcon } from "@heroicons/react/20/solid";
import type {
  ButtonHTMLAttributes,
  InputHTMLAttributes,
  ReactNode,
  Ref,
  SelectHTMLAttributes,
  TextareaHTMLAttributes,
} from "react";
import { createContext, useContext, useId } from "react";

const FieldContext = createContext<{ id: string; descriptionId?: string; invalid: boolean } | null>(
  null,
);
export type FieldProps = {
  label: ReactNode;
  htmlFor?: string;
  description?: ReactNode;
  error?: ReactNode;
  required?: boolean;
  children: ReactNode;
  className?: string;
};
export function Field({
  label,
  htmlFor,
  description,
  error,
  required,
  children,
  className = "",
}: FieldProps) {
  const generatedId = useId();
  const id = htmlFor ?? generatedId;
  const descriptionId = description
    ? error
      ? `${id}-description ${id}-error`
      : `${id}-description`
    : error
      ? `${id}-error`
      : undefined;
  return (
    <FieldContext value={{ id, descriptionId, invalid: Boolean(error) }}>
      <div className={`ui-field ${className}`}>
        <label htmlFor={id} className="ui-label">
          {label}
          {required && (
            <span aria-hidden="true" className="text-danger">
              {" "}
              *
            </span>
          )}
        </label>
        {children}
        {description && (
          <p id={`${id}-description`} className="ui-help">
            {description}
          </p>
        )}
        {error && (
          <p id={`${id}-error`} className="ui-error" role="alert">
            {error}
          </p>
        )}
      </div>
    </FieldContext>
  );
}

export type InputProps = InputHTMLAttributes<HTMLInputElement> & { ref?: Ref<HTMLInputElement> };
export function Input({ className = "", id, ...props }: InputProps) {
  const field = useContext(FieldContext);
  return (
    <input
      {...props}
      id={id ?? field?.id}
      aria-invalid={props["aria-invalid"] ?? (field?.invalid || undefined)}
      aria-describedby={props["aria-describedby"] ?? field?.descriptionId}
      className={`ui-input ${className}`}
    />
  );
}
export type TextareaProps = TextareaHTMLAttributes<HTMLTextAreaElement> & {
  ref?: Ref<HTMLTextAreaElement>;
};
export function Textarea({ className = "", id, ...props }: TextareaProps) {
  const field = useContext(FieldContext);
  return (
    <textarea
      {...props}
      id={id ?? field?.id}
      aria-invalid={props["aria-invalid"] ?? (field?.invalid || undefined)}
      aria-describedby={props["aria-describedby"] ?? field?.descriptionId}
      className={`ui-input ui-textarea ${className}`}
    />
  );
}
export type SelectProps = SelectHTMLAttributes<HTMLSelectElement> & {
  ref?: Ref<HTMLSelectElement>;
};
export function Select({ className = "", id, ...props }: SelectProps) {
  const field = useContext(FieldContext);
  return (
    <select
      {...props}
      id={id ?? field?.id}
      aria-invalid={props["aria-invalid"] ?? (field?.invalid || undefined)}
      aria-describedby={props["aria-describedby"] ?? field?.descriptionId}
      className={`ui-input ui-select ${className}`}
    />
  );
}

export type ChoiceProps = Omit<
  ButtonHTMLAttributes<HTMLButtonElement>,
  "onChange" | "onClick" | "value"
> & {
  value?: string;
  checked: boolean;
  onChange: (checked: boolean) => void;
  label: ReactNode;
  description?: ReactNode;
  name?: string;
  ref?: Ref<HTMLButtonElement>;
};
export function Switch({ label, description, className = "", disabled, ...props }: ChoiceProps) {
  return (
    <HeadlessField disabled={disabled} className={`ui-choice ${className}`}>
      <div className="min-w-0 flex-1">
        <Label className="ui-label">{label}</Label>
        {description && <Description className="ui-help mt-1">{description}</Description>}
      </div>
      <HeadlessSwitch {...props} disabled={disabled} className="ui-switch">
        <span className="ui-switch-thumb" aria-hidden="true" />
      </HeadlessSwitch>
    </HeadlessField>
  );
}
export function Checkbox({ label, description, className = "", disabled, ...props }: ChoiceProps) {
  return (
    <HeadlessField disabled={disabled} className={`ui-choice ui-checkbox-field ${className}`}>
      <HeadlessCheckbox {...props} disabled={disabled} className="ui-checkbox">
        <CheckIcon
          className="ui-checkbox-mark size-4"
          data-checked={props.checked || undefined}
          aria-hidden="true"
        />
      </HeadlessCheckbox>
      <div className="min-w-0 flex-1">
        <Label className="ui-label">{label}</Label>
        {description && <Description className="ui-help mt-1">{description}</Description>}
      </div>
    </HeadlessField>
  );
}
