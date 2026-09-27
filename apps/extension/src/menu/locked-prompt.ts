import { LockedWatch, type VaultWatcher } from "./vault-state.ts";

/** A card's state that listeners subscribe to and that reviews itself once Ravenpass leaves the locked state. */
export abstract class LockedPrompt<View> {
  private readonly listeners = new Set<() => void>();
  private readonly lockedWatch: LockedWatch;
  private shown: View;
  /** An answer that arrives after a start or stop since it was asked is dropped. */
  private runs = 0;
  private started = false;

  protected constructor(watchVault: VaultWatcher, initial: View) {
    this.lockedWatch = new LockedWatch(watchVault, () => {
      void this.review();
    });
    this.shown = initial;
  }

  readonly subscribe = (listener: () => void): (() => void) => {
    this.listeners.add(listener);
    return () => this.listeners.delete(listener);
  };

  readonly view = (): View => this.shown;

  start(): void {
    if (this.started) return;
    this.started = true;
    this.runs += 1;
    this.syncWatch();
  }

  stop(): void {
    this.started = false;
    this.runs += 1;
    this.syncWatch();
  }

  protected get current(): View {
    return this.shown;
  }

  protected get run(): number {
    return this.runs;
  }

  protected get running(): boolean {
    return this.started;
  }

  protected update(view: View): void {
    this.shown = view;
    this.syncWatch();
    for (const listener of this.listeners) listener();
  }

  /** Whether `view` shows Ravenpass locked, which holds the vault state watch while the card runs. */
  protected abstract showsLocked(view: View): boolean;

  /** Asks again for what the card shows, after Ravenpass leaves the locked state. */
  protected abstract review(): Promise<void>;

  private syncWatch(): void {
    this.lockedWatch.sync(this.started && this.showsLocked(this.shown));
  }
}
