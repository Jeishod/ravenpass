import { cn } from "cn";
import { useTranslator } from "../i18n/translator.tsx";
import { AccessField } from "./AccessField.tsx";
import { PinAttemptsWarning } from "./PinAttemptsWarning.tsx";

/** PinField takes a masked PIN on the numeric keyboard and drops anything but digits as typed. */
export function PinField({
  id,
  value,
  onChange,
  maxLength,
  attemptsLeft,
  disabled,
  label,
  autoFocus = true,
  invalid = false,
  className,
}: {
  id: string;
  value: string;
  onChange: (pin: string) => void;
  maxLength?: number;
  /** Left out while the attempts are not known. */
  attemptsLeft?: number;
  disabled: boolean;
  /** The label and placeholder of a PIN being chosen; left out for the vault's PIN. */
  label?: string;
  /** Unset for a field that follows another in its form. */
  autoFocus?: boolean;
  invalid?: boolean;
  /** Sizes the field beyond the access field's own height. */
  className?: string;
}) {
  const { t } = useTranslator();
  return (
    <>
      <label className="sr-only" htmlFor={id}>
        {label ?? t("unlock.pin.label")}
      </label>
      <AccessField
        id={id}
        className={cn("font-mono text-[19px] tracking-[0.4em]", className)}
        type="password"
        inputMode="numeric"
        autoComplete="off"
        autoFocus={autoFocus}
        value={value}
        placeholder={label ?? t("unlock.pin.placeholder")}
        maxLength={maxLength}
        disabled={disabled}
        aria-invalid={invalid || undefined}
        onChange={(event) => onChange(event.target.value.replace(/\D/gu, ""))}
      />
      {attemptsLeft !== undefined && (
        <PinAttemptsWarning attemptsLeft={attemptsLeft} />
      )}
    </>
  );
}
