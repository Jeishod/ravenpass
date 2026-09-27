import { useSyncExternalStore } from "react";

/** Must match Tailwind's `sm` breakpoint, which CSS uses as `max-sm:`. */
const compactQuery = "(width < 40rem)";

function subscribe(onChange: () => void) {
  const query = window.matchMedia(compactQuery);
  query.addEventListener("change", onChange);
  return () => query.removeEventListener("change", onChange);
}

function compactNow() {
  return window.matchMedia(compactQuery).matches;
}

export function useCompactLayout(): boolean {
  return useSyncExternalStore(subscribe, compactNow, () => false);
}
