import type { Suggestion } from "../link/client.ts";
import type { SessionArea } from "./sessions.ts";

const windowMs = 10 * 60_000;
const keyPrefix = "filled:";

/** `at` is in Unix milliseconds. */
interface Fill {
  readonly credential: string;
  readonly at: number;
}

export interface RecentFillsDependencies {
  readonly area: SessionArea;
  readonly now?: () => number;
}

/** The one-time code a site asks for next usually belongs to the credential last filled in its tab. */
export class RecentFills {
  private readonly area: SessionArea;
  private readonly now: () => number;

  constructor({ area, now = Date.now }: RecentFillsDependencies) {
    this.area = area;
    this.now = now;
  }

  async remember(tabId: number, credential: string): Promise<void> {
    const fill: Fill = { credential, at: this.now() };
    await this.area.set({ [keyPrefix + tabId]: fill });
  }

  /** The credential filled within `windowMs` comes first; the others keep their order. */
  async order<Credential extends Suggestion>(
    tabId: number,
    credentials: readonly Credential[],
  ): Promise<Credential[]> {
    const recent = await this.recent(tabId);
    const index = credentials.findIndex(({ id }) => id === recent);
    const filled = credentials[index];
    if (index === 0 || !filled) return [...credentials];
    return [
      filled,
      ...credentials.slice(0, index),
      ...credentials.slice(index + 1),
    ];
  }

  async forget(tabId: number): Promise<void> {
    await this.area.remove(keyPrefix + tabId);
  }

  private async recent(tabId: number): Promise<string | null> {
    const key = keyPrefix + tabId;
    const fill = (await this.area.get(key))[key] as Fill | undefined;
    if (!fill) return null;
    if (fill.at + windowMs <= this.now()) {
      await this.area.remove(key);
      return null;
    }
    return fill.credential;
  }
}
