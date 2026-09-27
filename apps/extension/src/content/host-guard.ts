/** The attributes of a menu's host element whose change can hide, move or restyle the menu. */
export const guardedAttributes = ["style", "class", "popover"] as const;

export interface GuardedHost {
  getAttribute(name: string): string | null;
}

/** `settle` must follow every change Ravenpass makes; any other difference is the page's. */
export class HostGuard {
  private readonly host: GuardedHost;
  private readonly onTampered: () => void;
  private settled: readonly (string | null)[] = [];

  constructor(host: GuardedHost, onTampered: () => void) {
    this.host = host;
    this.onTampered = onTampered;
    this.settle();
  }

  settle(): void {
    this.settled = this.read();
  }

  review(): void {
    const current = this.read();
    if (current.some((value, index) => value !== this.settled[index])) {
      this.onTampered();
    }
  }

  private read(): (string | null)[] {
    return guardedAttributes.map((name) => this.host.getAttribute(name));
  }
}
