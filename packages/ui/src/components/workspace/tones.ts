import type { Validity } from "../../identities/dates.ts";

/** How strongly a value asks for attention: not at all, soon, or now. */
export type Tone = "default" | "warning" | "destructive";

export const toneText: Record<Tone, string> = {
  default: "text-muted-foreground",
  warning: "text-warning",
  destructive: "text-destructive",
};

/** validityTone warns while a validity runs out and alarms once it has. */
export function validityTone(validity: Validity | null, today: Date): Tone {
  switch (validity?.state(today)) {
    case "expired":
      return "destructive";
    case "expiring":
      return "warning";
    default:
      return "default";
  }
}
