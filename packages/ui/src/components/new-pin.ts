export type NewPinCheck = "short" | "repeating" | "matched" | "mismatched";

/** checkNewPin reads the repeat as mismatched only once it is as long as the PIN. */
export function checkNewPin(
  pin: string,
  repeat: string,
  minimum: number,
): NewPinCheck {
  if (pin.length < minimum) return "short";
  if (pin === repeat) return "matched";
  return repeat.length >= pin.length ? "mismatched" : "repeating";
}
