import type { KeyboardEvent } from "react";

/** A list position a key moves to: an item's index, or above the first item. */
export type ListTarget = number | "before";

/** listTarget returns where ArrowDown, ArrowUp, Home or End moves from `index` (-1 for none) in a list of `count`, null for other keys. */
export function listTarget(
  key: string,
  index: number,
  count: number,
  loop: boolean,
): ListTarget | null {
  if (count === 0) return null;
  switch (key) {
    case "Home":
      return 0;
    case "End":
      return count - 1;
    case "ArrowDown":
      if (index < count - 1) return index + 1;
      return loop ? 0 : count - 1;
    case "ArrowUp":
      if (index > 0) return index - 1;
      if (loop && index === 0) return count - 1;
      return "before";
    default:
      return null;
  }
}

export interface RovingFocusOptions {
  count: number;
  /** Moves focus to an item; a virtualized list brings it into view first. */
  focus: (index: number) => void;
  loop?: boolean;
  /** Takes focus when ArrowUp leaves the first item; without it focus stays. */
  onBefore?: () => void;
}

/** useRovingFocus returns the keydown handler that moves focus between a list's items by index. */
export function useRovingFocus({
  count,
  focus,
  loop = false,
  onBefore,
}: RovingFocusOptions): (event: KeyboardEvent, index: number) => void {
  return (event, index) => {
    const target = listTarget(event.key, index, count, loop);
    if (target === null) return;
    event.preventDefault();
    if (target === "before") onBefore?.();
    else focus(target);
  };
}
