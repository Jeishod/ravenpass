import {
  isVaultStateMessage,
  type VaultState,
  vaultStatePort,
} from "../messages.ts";

/** A `chrome.runtime.Port` as far as the watcher uses it. */
export interface StatePort {
  readonly onMessage: {
    addListener(listener: (message: unknown) => void): void;
  };
  readonly onDisconnect: {
    addListener(listener: () => void): void;
  };
  disconnect(): void;
}

export type VaultListener = (state: VaultState) => void;

/** Reopens the port whenever the service worker drops it, as when Chrome stops the worker. */
export function watchVaultState(
  listener: VaultListener,
  connect: () => StatePort = () =>
    chrome.runtime.connect({ name: vaultStatePort }),
): () => void {
  let watching = true;
  let port: StatePort | null = null;
  const open = () => {
    if (!watching) return;
    port = connect();
    port.onMessage.addListener((message) => {
      if (watching && isVaultStateMessage(message)) listener(message.state);
    });
    port.onDisconnect.addListener(open);
  };
  open();
  return () => {
    watching = false;
    port?.disconnect();
  };
}

export type VaultWatcher = (listener: VaultListener) => () => void;

/** Holds one vault state watch while a card shows Ravenpass locked and calls `onChange` for every other state. */
export class LockedWatch {
  private readonly watch: VaultWatcher;
  private readonly onChange: () => void;
  private unwatch: (() => void) | null = null;

  constructor(watch: VaultWatcher, onChange: () => void) {
    this.watch = watch;
    this.onChange = onChange;
  }

  sync(locked: boolean): void {
    if (locked && !this.unwatch) {
      this.unwatch = this.watch((state) => {
        if (state !== "locked") this.onChange();
      });
    } else if (!locked && this.unwatch) {
      this.unwatch();
      this.unwatch = null;
    }
  }
}
