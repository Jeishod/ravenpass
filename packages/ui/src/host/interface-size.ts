/** In percent. */
export const naturalInterfaceSize = 100;

/** Lays the page out `100 / percent` times wider than `width`, the CSS width at scale 1, and scales it back to fit. */
export function scaledViewport(percent: number, width: number): string {
  const scale = percent / 100;
  return [
    `width=${Math.round(width / scale)}`,
    `initial-scale=${scale}`,
    `minimum-scale=${scale}`,
    `maximum-scale=${scale}`,
    "user-scalable=no",
    "viewport-fit=cover",
  ].join(", ");
}

export interface PageViewport {
  /** The viewport tag's content. */
  content: string;
  /** CSS pixels at scale 1; zero while the page has no size. */
  readonly width: number;
  onResize(listener: () => void): void;
}

/** Scales through the viewport tag: a root CSS zoom misplaces floating menus and overflows `100dvh`. */
export class InterfaceScale {
  readonly #viewport: PageViewport;
  readonly #natural: string;
  #percent = naturalInterfaceSize;

  constructor(viewport: PageViewport) {
    this.#viewport = viewport;
    this.#natural = viewport.content;
    viewport.onResize(() => this.#fit());
  }

  /** Keeps the size as the screen changes. */
  show(percent: number) {
    this.#percent = percent;
    this.#fit();
  }

  #fit() {
    let content = this.#natural;
    if (this.#percent !== naturalInterfaceSize) {
      const width = this.#viewport.width;
      if (width <= 0) return;
      content = scaledViewport(this.#percent, width);
    }
    if (content !== this.#viewport.content) this.#viewport.content = content;
  }
}

let pageScale: InterfaceScale | undefined;

function scaleOfPage(): InterfaceScale | undefined {
  const tag = document.querySelector<HTMLMetaElement>('meta[name="viewport"]');
  const viewport = window.visualViewport;
  if (!tag || !viewport) return undefined;
  return new InterfaceScale({
    get content() {
      return tag.content;
    },
    set content(content) {
      tag.content = content;
    },
    // The visual viewport shrinks as the scale grows, so their product stays the width at scale 1.
    get width() {
      return viewport.width * viewport.scale;
    },
    onResize: (listener) => viewport.addEventListener("resize", listener),
  });
}

export function showInterfaceSize(percent: number) {
  pageScale ??= scaleOfPage();
  pageScale?.show(percent);
}
