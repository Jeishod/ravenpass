import { useSyncExternalStore } from "react";

function subscribe(onChange: () => void) {
  document.addEventListener("visibilitychange", onChange);
  return () => document.removeEventListener("visibilitychange", onChange);
}

function visibleNow() {
  return document.visibilityState === "visible";
}

export function usePageVisible(): boolean {
  return useSyncExternalStore(subscribe, visibleNow, () => true);
}
