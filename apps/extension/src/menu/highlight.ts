import { useState } from "react";

/** Selects the elements the arrow keys move between. */
export const itemSelector = "[data-menu-item]";

/** When the pointer leaves the list, the highlight returns to the focused row. */
export function useHighlight() {
  const [highlighted, setHighlighted] = useState<string | null>(null);
  return {
    highlighted,
    list: { onPointerLeave: () => setHighlighted(focusedRow()) },
    row: (key: string) => ({
      "data-menu-item": true,
      "data-row": key,
      onPointerEnter: () => setHighlighted(key),
      onFocus: () => setHighlighted(key),
      onBlur: () => setHighlighted(null),
    }),
  };
}

function focusedRow(): string | null {
  const focused = document.activeElement;
  return focused instanceof HTMLElement ? (focused.dataset.row ?? null) : null;
}
