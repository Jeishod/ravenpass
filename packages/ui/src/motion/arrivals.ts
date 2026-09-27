/** Runs a task once, after the current frame. */
export type FrameScheduler = (task: () => void) => void;

/** RowArrivals staggers rows that first come into view in the same frame; a row seen before appears at once. */
export class RowArrivals {
  private readonly seen = new Set<string>();
  private readonly nextFrame: FrameScheduler;
  private arriving = 0;
  private frameOpen = false;

  constructor(
    nextFrame: FrameScheduler = (task) => {
      requestAnimationFrame(task);
    },
  ) {
    this.nextFrame = nextFrame;
  }

  seenBefore(id: string): boolean {
    return this.seen.has(id);
  }

  /** arrive returns the row's zero-based place among the rows arriving in the same frame. */
  arrive(id: string): number {
    this.seen.add(id);
    if (!this.frameOpen) {
      this.frameOpen = true;
      this.nextFrame(() => {
        this.arriving = 0;
        this.frameOpen = false;
      });
    }
    return this.arriving++;
  }
}
