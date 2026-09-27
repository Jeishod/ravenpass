import type { LanguageArea } from "./language.ts";

type Listener = (changes: Record<string, unknown>) => void;

/** `chrome.storage.local` in memory: it tells its listeners which keys a write changed. */
export class MemoryLocalArea implements LanguageArea {
  readonly items = new Map<string, unknown>();
  readonly listeners = new Set<Listener>();
  readonly onChanged = {
    addListener: (listener: Listener) => {
      this.listeners.add(listener);
    },
    removeListener: (listener: Listener) => {
      this.listeners.delete(listener);
    },
  };

  async get(keys: string[]): Promise<Record<string, unknown>> {
    return Object.fromEntries(
      keys
        .filter((key) => this.items.has(key))
        .map((key) => [key, this.items.get(key)]),
    );
  }

  async set(items: Record<string, unknown>): Promise<void> {
    const changes: Record<string, unknown> = {};
    for (const [key, value] of Object.entries(items)) {
      changes[key] = { oldValue: this.items.get(key), newValue: value };
      this.items.set(key, value);
    }
    this.notify(changes);
  }

  async remove(key: string): Promise<void> {
    if (!this.items.has(key)) return;
    const oldValue = this.items.get(key);
    this.items.delete(key);
    this.notify({ [key]: { oldValue } });
  }

  private notify(changes: Record<string, unknown>): void {
    for (const listener of this.listeners) listener(changes);
  }
}
