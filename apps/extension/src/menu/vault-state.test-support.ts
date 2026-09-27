import type { VaultState } from "../messages.ts";
import type { VaultListener } from "./vault-state.ts";

/** A vault state watch the test pushes states through. */
export class FakeVault {
  private listener: VaultListener | null = null;

  readonly watch = (listener: VaultListener): (() => void) => {
    this.listener = listener;
    return () => {
      if (this.listener === listener) this.listener = null;
    };
  };

  watching(): boolean {
    return this.listener !== null;
  }

  push(state: VaultState): void {
    this.listener?.(state);
  }
}

export function settle(): Promise<void> {
  return new Promise((resolve) => setImmediate(resolve));
}
