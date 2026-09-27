export interface Styled {
  readonly style: Pick<CSSStyleDeclaration, "setProperty" | "removeProperty">;
}

/** While the surface animates it carries the blur: its opacity or transform cuts off a child's.
 * At rest the layer carries it: Chrome reports a frame under a backdrop filter as not visible. */
export class Frost {
  private readonly surface: Styled;
  private readonly layer: Styled;
  private readonly filter: string;

  constructor(surface: Styled, layer: Styled, filter: string) {
    this.surface = surface;
    this.layer = layer;
    this.filter = filter;
  }

  moving(): void {
    this.move(this.layer, this.surface);
  }

  settled(): void {
    this.move(this.surface, this.layer);
  }

  private move(from: Styled, to: Styled): void {
    to.style.setProperty("backdrop-filter", this.filter, "important");
    from.style.removeProperty("backdrop-filter");
  }
}
