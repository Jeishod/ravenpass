import { cn } from "cn";
import { useId } from "react";
import { Field, FieldError, FieldLabel } from "../ui/field.tsx";
import { Input } from "../ui/input.tsx";

/** OfferField edits the name or account of a new saved item; `dense` fits a browser card. */
export function OfferField({
  label,
  value,
  error,
  disabled,
  dense = false,
  onChange,
}: {
  label: string;
  value: string;
  /** Empty while the vault refused nothing. */
  error: string;
  disabled: boolean;
  dense?: boolean;
  onChange: (value: string) => void;
}) {
  const id = useId();
  return (
    <Field
      className={dense ? "gap-1" : "gap-1.5"}
      data-invalid={error ? true : undefined}
    >
      <FieldLabel
        htmlFor={id}
        className={cn(
          "font-normal text-muted-foreground",
          dense ? "text-[11px]" : "text-xs",
        )}
      >
        {label}
      </FieldLabel>
      <Input
        id={id}
        className={dense ? "h-7" : undefined}
        value={value}
        disabled={disabled}
        autoComplete="off"
        spellCheck={false}
        aria-invalid={error ? true : undefined}
        onChange={(event) => onChange(event.target.value)}
      />
      {error && <FieldError className="text-[11px]">{error}</FieldError>}
    </Field>
  );
}
