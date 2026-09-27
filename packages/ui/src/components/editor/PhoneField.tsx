import { useMaskito } from "@maskito/react";
import { cn } from "cn";
import { phoneMask } from "../../identities/phone.ts";
import { Input } from "../ui/input.tsx";
import { bareField } from "./EditorFields.tsx";

/** PhoneField formats an international number as typed; its value is the text as shown. */
export function PhoneField({
  id,
  value,
  maxLength,
  disabled,
  onChange,
}: {
  id: string;
  value: string;
  maxLength: number | undefined;
  disabled: boolean;
  onChange: (value: string) => void;
}) {
  const maskRef = useMaskito({ options: phoneMask });

  return (
    <Input
      id={id}
      ref={maskRef}
      type="tel"
      className={cn(bareField, "tabular-nums")}
      value={value}
      onInput={(event) => onChange(event.currentTarget.value)}
      placeholder="+1 212 555 0100"
      maxLength={maxLength}
      autoComplete="off"
      spellCheck={false}
      disabled={disabled}
    />
  );
}
