/** SelectionQueue serializes closes of the host's open item so a late close never drops its successor. */
export class SelectionQueue {
  private closing: Promise<void> = Promise.resolve();
  private readonly clear: () => Promise<void>;

  constructor(clear: () => Promise<void>) {
    this.clear = clear;
  }

  /** close asks the host to close the open item, after any close still running. */
  close(): Promise<void> {
    // An earlier failed close was already rejected to its own caller.
    this.closing = this.closing.catch(() => {}).then(() => this.clear());
    return this.closing;
  }

  /** settled waits for every close so far and rejects when the last one failed. */
  settled(): Promise<void> {
    return this.closing;
  }
}
