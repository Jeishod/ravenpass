import type { PageChanges } from "./page-changes.ts";

/** How long a click or Enter that may have sent a form keeps it armed. */
export const armMs = 10_000;

/** Reports a form once when it leaves the page; gives up after `forMs`. */
export class Leaving {
  private readonly left: () => boolean;
  private readonly onLeft: () => void;
  private readonly unobserve: () => void;
  private readonly giveUp: ReturnType<typeof setTimeout>;
  private stopped = false;

  constructor(
    left: () => boolean,
    onLeft: () => void,
    forMs: number,
    changes: PageChanges,
  ) {
    this.left = left;
    this.onLeft = onLeft;
    this.unobserve = changes(this.check);
    this.giveUp = setTimeout(() => this.stop(), forMs);
  }

  get watching(): boolean {
    return !this.stopped;
  }

  stop(): void {
    this.stopped = true;
    this.unobserve();
    clearTimeout(this.giveUp);
  }

  private readonly check = (): void => {
    if (this.stopped || !this.left()) return;
    this.stop();
    this.onLeft();
  };
}

export interface ArmingDependencies<Box> {
  readonly left: (box: Box) => boolean;
  /** `left` is true once the box left the page, false when its page is hidden first. */
  readonly capture: (box: Box, left: boolean) => void;
  readonly changes: (box: Box) => PageChanges;
}

/** Holds the one form a click or Enter may have sent without a `submit` event. */
export class Arming<Box> {
  private readonly left: (box: Box) => boolean;
  private readonly capture: (box: Box, left: boolean) => void;
  private readonly changes: (box: Box) => PageChanges;
  private armed: { readonly box: Box; readonly watch: Leaving } | null = null;

  constructor({ left, capture, changes }: ArmingDependencies<Box>) {
    this.left = left;
    this.capture = capture;
    this.changes = changes;
  }

  arm(box: Box): void {
    this.disarm();
    const watch = new Leaving(
      () => this.left(box),
      () => {
        this.armed = null;
        this.capture(box, true);
      },
      armMs,
      this.changes(box),
    );
    this.armed = { box, watch };
  }

  /** Lets the armed form go, or only one `which` picks. */
  disarm(which: (box: Box) => boolean = () => true): void {
    const armed = this.armed;
    if (!armed || !which(armed.box)) return;
    armed.watch.stop();
    this.armed = null;
  }

  pageHidden(): void {
    const armed = this.armed;
    const watching = armed?.watch.watching ?? false;
    this.disarm();
    if (armed && watching) this.capture(armed.box, false);
  }
}
