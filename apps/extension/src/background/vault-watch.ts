import type { LinkStatus } from "../link/client.ts";
import type { VaultState, VaultStateMessage } from "../messages.ts";

/** A `chrome.runtime.Port` as far as the watch uses it. */
export interface WatchPort {
  postMessage(message: VaultStateMessage): void;
  readonly onDisconnect: {
    addListener(listener: () => void): void;
  };
}

export interface VaultWatchDependencies {
  readonly status: () => Promise<LinkStatus>;
  readonly everyMs: number;
}

/** Polls the desktop app only while a menu port is connected. */
export class VaultWatch {
  private readonly status: () => Promise<LinkStatus>;
  private readonly everyMs: number;
  private readonly ports = new Set<WatchPort>();
  private state: VaultState | null = null;
  private timer: ReturnType<typeof setTimeout> | undefined;
  private polling = false;

  constructor({ status, everyMs }: VaultWatchDependencies) {
    this.status = status;
    this.everyMs = everyMs;
  }

  connect(port: WatchPort): void {
    this.ports.add(port);
    port.onDisconnect.addListener(() => this.disconnect(port));
    if (this.state !== null) {
      port.postMessage({ kind: "vault-state", state: this.state });
    }
    if (!this.polling) void this.poll();
  }

  private disconnect(port: WatchPort): void {
    this.ports.delete(port);
    if (this.ports.size > 0) return;
    clearTimeout(this.timer);
    this.state = null;
  }

  private async poll(): Promise<void> {
    clearTimeout(this.timer);
    this.polling = true;
    try {
      const state = vaultStateOf(await this.status());
      if (this.ports.size === 0 || state === this.state) return;
      this.state = state;
      for (const port of this.ports) {
        port.postMessage({ kind: "vault-state", state });
      }
    } finally {
      this.polling = false;
      if (this.ports.size > 0) {
        this.timer = setTimeout(() => void this.poll(), this.everyMs);
      }
    }
  }
}

function vaultStateOf(status: LinkStatus): VaultState {
  return status.linked ? status.desktop : "unlinked";
}
