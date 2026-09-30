import type { ColorScheme } from "../toolbar/icon.ts";

/** A colour in sRGB, each channel and alpha from 0 to 1. */
export interface Rgba {
  readonly red: number;
  readonly green: number;
  readonly blue: number;
  readonly alpha: number;
}

/** How the menu's frost mixes Ravenpass's popover colour with the blurred page. */
export interface Frosting {
  /** The share of the popover colour the menu page lays over the blurred page. */
  readonly tint: number;
  /** What a page's own background adds to the menu's colour. */
  readonly backgroundLight: number;
  /** The most a white spot behind the menu adds. */
  readonly maxLight: number;
  /** The opacity of the dark ring that sets the menu off the page. */
  readonly ring: number;
}

export const frostings: Readonly<Record<ColorScheme, Frosting>> = {
  // Above about 0.35 of added light, muted text loses contrast.
  dark: { tint: 0.35, backgroundLight: 0.085, maxLight: 0.32, ring: 0.4 },
  // No brightness turns a dark page light, so the popover colour carries the light; 0.78 keeps muted text at 4.5:1 over black.
  light: { tint: 0.78, backgroundLight: 0.2, maxLight: 0.22, ring: 0.1 },
};

/** Chrome's light or dark mode, which the menu page's stylesheet follows. */
export function preferredScheme(): ColorScheme {
  return matchMedia("(prefers-color-scheme: dark)").matches ? "dark" : "light";
}

/** Chrome's canvas for a document in a dark colour scheme with no background of its own. */
const darkCanvas: Rgba = {
  red: 0x12 / 255,
  green: 0x12 / 255,
  blue: 0x12 / 255,
  alpha: 1,
};
const lightCanvas: Rgba = { red: 1, green: 1, blue: 1, alpha: 1 };

/** CSS filters are affine: a light page and a dark page need different brightness. */
export function backdropFilter(
  luminance: number,
  { tint, backgroundLight, maxLight }: Frosting,
): string {
  const light = Math.min(backgroundLight / Math.max(luminance, 0.01), maxLight);
  const brightness = light / (1 - tint);
  return `blur(14px) saturate(1.6) brightness(${brightness.toFixed(3)})`;
}

/** The luminance of sRGB values, as `brightness()` scales them. */
export function luminance({ red, green, blue }: Rgba): number {
  return 0.2126 * red + 0.7152 * green + 0.0722 * blue;
}

/** Layers colours over an opaque base, the lowest first. */
export function composite(base: Rgba, layers: readonly Rgba[]): Rgba {
  return layers.reduce<Rgba>(
    (under, { red, green, blue, alpha }) => ({
      red: red * alpha + under.red * (1 - alpha),
      green: green * alpha + under.green * (1 - alpha),
      blue: blue * alpha + under.blue * (1 - alpha),
      alpha: 1,
    }),
    base,
  );
}

/** Reads background colours only; background images and gradients are ignored. */
export function pageLuminanceBehind(host: Element): number {
  const document = host.ownerDocument;
  const { left, top, width, height } = host.getBoundingClientRect();
  const layers: Rgba[] = [];
  for (
    let element: Element | null | undefined = document
      .elementsFromPoint(left + width / 2, top + height / 2)
      .find((hit) => hit !== host);
    element;
    element = element.parentElement
  ) {
    const color = parseColor(getComputedStyle(element).backgroundColor);
    if (!color || color.alpha === 0) continue;
    layers.unshift(color);
    if (color.alpha === 1) break;
  }
  return luminance(composite(canvasOf(document), layers));
}

function canvasOf(document: Document): Rgba {
  const scheme = getComputedStyle(document.documentElement).colorScheme;
  const dark = scheme.includes("dark")
    ? !scheme.includes("light") ||
      matchMedia("(prefers-color-scheme: dark)").matches
    : false;
  return dark ? darkCanvas : lightCanvas;
}

let context: OffscreenCanvasRenderingContext2D | null = null;

/** Computed colours keep their authored space, such as `oklch()`; a canvas converts them to sRGB. */
function parseColor(color: string): Rgba | null {
  context ??= new OffscreenCanvas(1, 1).getContext("2d", {
    willReadFrequently: true,
  });
  if (!context) return null;
  context.clearRect(0, 0, 1, 1);
  context.fillStyle = color;
  context.fillRect(0, 0, 1, 1);
  const [red = 0, green = 0, blue = 0, alpha = 0] = context.getImageData(
    0,
    0,
    1,
    1,
  ).data;
  return {
    red: red / 255,
    green: green / 255,
    blue: blue / 255,
    alpha: alpha / 255,
  };
}
