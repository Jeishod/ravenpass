/** Waits until the host's count of some change passes `seen`, then resolves with the count. */
export type AwaitCount = (seen: number, signal: AbortSignal) => Promise<number>;

/** Reports each change a host counts, such as an extension's save or a language chosen elsewhere. */
export class CountWatcher {
  private readonly awaitCount: AwaitCount;

  constructor(awaitCount: AwaitCount) {
    this.awaitCount = awaitCount;
  }

  /** The host fails a wait only when it is canceled, so a failed wait ends the watch. */
  watch(onChange: () => void): () => void {
    const controller = new AbortController();
    void this.run(controller.signal, onChange);
    return () => controller.abort();
  }

  private async run(signal: AbortSignal, onChange: () => void): Promise<void> {
    let seen = 0;
    while (!signal.aborted) {
      try {
        seen = await this.awaitCount(seen, signal);
      } catch {
        return;
      }
      if (!signal.aborted) onChange();
    }
  }
}
