import type { ShareProgress } from "../link/client.ts";

/** What the desktop asks of the owner while a password or code waits for confirmation; progress with no request waiting is dropped. */
export class FillProgress {
  private waiting = 0;
  private current: ShareProgress | null = null;
  private readonly listeners = new Set<() => void>();

  readonly subscribe = (listener: () => void): (() => void) => {
    this.listeners.add(listener);
    return () => this.listeners.delete(listener);
  };

  readonly view = (): ShareProgress | null => this.current;

  /** Settles as `request` does; the progress clears once no request waits. */
  async track<Answer>(request: Promise<Answer>): Promise<Answer> {
    this.waiting += 1;
    try {
      return await request;
    } finally {
      this.waiting -= 1;
      if (this.waiting === 0) this.update(null);
    }
  }

  report(progress: ShareProgress): void {
    if (this.waiting > 0) this.update(progress);
  }

  private update(progress: ShareProgress | null): void {
    if (progress === this.current) return;
    this.current = progress;
    for (const listener of this.listeners) listener();
  }
}
