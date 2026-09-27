import type { SessionArea } from "./sessions.ts";

const lifetimeMs = 60_000;
const keyPrefix = "sign-in:";

/** Holds a credential id, never its values; `expiresAt` is in Unix milliseconds. */
interface PendingSignIn {
  readonly credential: string;
  readonly origin: string;
  readonly expiresAt: number;
}

export interface PendingSignInsDependencies {
  readonly area: SessionArea;
  readonly now?: () => number;
}

/** One pending sign-in per tab; it ends once taken, after `lifetimeMs`, with its tab, or when replaced. */
export class PendingSignIns {
  private readonly area: SessionArea;
  private readonly now: () => number;

  constructor({ area, now = Date.now }: PendingSignInsDependencies) {
    this.area = area;
    this.now = now;
  }

  async record(
    tabId: number,
    credential: string,
    origin: string,
  ): Promise<void> {
    const pending: PendingSignIn = {
      credential,
      origin,
      expiresAt: this.now() + lifetimeMs,
    };
    await this.area.set({ [keyPrefix + tabId]: pending });
  }

  /** A sign-in pending for another origin stays pending. */
  async take(tabId: number, origin: string): Promise<string | null> {
    const key = keyPrefix + tabId;
    const pending = (await this.area.get(key))[key] as
      | PendingSignIn
      | undefined;
    if (!pending) return null;
    if (pending.expiresAt <= this.now()) {
      await this.area.remove(key);
      return null;
    }
    if (pending.origin !== origin) return null;
    await this.area.remove(key);
    return pending.credential;
  }

  async forget(tabId: number): Promise<void> {
    await this.area.remove(keyPrefix + tabId);
  }
}
