/** `system` follows the system's light or dark mode. */
const appearances = ["system", "light", "dark"] as const;

export type Appearance = (typeof appearances)[number];

export function isAppearance(value: unknown): value is Appearance {
  return appearances.some((appearance) => appearance === value);
}

/** An unknown value reads as `system`. */
export function appearanceOf(value: unknown): Appearance {
  return isAppearance(value) ? value : "system";
}

/** The attribute the stylesheet's `dark` variant reads; without it the page follows the system. */
export function appearanceAttribute(appearance: Appearance): string | null {
  return appearance === "system" ? null : appearance;
}

export function showAppearance(appearance: Appearance) {
  const attribute = appearanceAttribute(appearance);
  const root = document.documentElement;
  if (attribute) root.dataset.appearance = attribute;
  else delete root.dataset.appearance;
}
