import { isMenuMoved } from "../messages.ts";

// Intersection Observer v2, which Chrome ships and the DOM typings omit.
declare global {
  interface IntersectionObserverInit {
    trackVisibility?: boolean;
    delay?: number;
  }
  interface IntersectionObserverEntry {
    readonly isVisible: boolean;
  }
}

/** How long the menu must have been shown and kept its place before a click acts. */
export const stillMs = 500;

/** The shortest interval Chrome accepts between visibility reports. */
const visibilityDelayMs = 100;

export interface ClickGateDependencies {
  /** A monotonic clock in milliseconds. */
  readonly now?: () => number;
}

export interface GatedEvent {
  preventDefault(): void;
  stopImmediatePropagation(): void;
}

/** Accepts a click only while Chrome reports the frame visible and unobscured and it has kept its place for `stillMs`. */
export class ClickGate {
  private readonly now: () => number;
  private visible = false;
  private stillSince = 0;

  constructor({ now = () => performance.now() }: ClickGateDependencies = {}) {
    this.now = now;
  }

  visibility(visible: boolean): void {
    if (visible && !this.visible) this.stillSince = this.now();
    this.visible = visible;
  }

  /** The framing content script reports the end of the entrance and each move or resize. */
  moved(): void {
    this.stillSince = this.now();
  }

  accepts(): boolean {
    return this.visible && this.now() - this.stillSince >= stillMs;
  }

  /** Must run in the capture phase, ahead of the menu's own listeners. */
  readonly screen = (event: GatedEvent): void => {
    if (this.accepts()) return;
    event.preventDefault();
    event.stopImmediatePropagation();
  };

  start(root: Element): () => void {
    const observer = new IntersectionObserver(
      (entries) => {
        const latest = entries.at(-1);
        if (latest) this.visibility(latest.isVisible);
      },
      { trackVisibility: true, delay: visibilityDelayMs },
    );
    observer.observe(root);
    const onMessage = (event: MessageEvent) => {
      if (event.source === parent && isMenuMoved(event.data)) this.moved();
    };
    addEventListener("message", onMessage);
    addEventListener("click", this.screen, true);
    return () => {
      observer.disconnect();
      removeEventListener("message", onMessage);
      removeEventListener("click", this.screen, true);
    };
  }
}
