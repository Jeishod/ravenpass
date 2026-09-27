/** The colour scheme Chrome draws its toolbar in. */
export type ColorScheme = "light" | "dark";

/** Icon paths keyed by pixel size, for 1x and 2x displays. */
export type IconPaths = Readonly<Record<16 | 32, string>>;

// Root-absolute: a service worker's setIcon resolves paths against its own script; the dark-toolbar icons ship from public/.
const icons: Readonly<Record<ColorScheme, IconPaths>> = {
  light: {
    16: "/icons/toolbar-on-light-16.png",
    32: "/icons/toolbar-on-light-32.png",
  },
  dark: {
    16: "/icons/toolbar-on-dark-16.png",
    32: "/icons/toolbar-on-dark-32.png",
  },
};

export function toolbarIcon(scheme: ColorScheme): IconPaths {
  return icons[scheme];
}
