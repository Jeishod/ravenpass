/** The browser storage of this interface, or null where there is none or it refuses access. */
export function browserStorage(): Storage | null {
  try {
    return globalThis.localStorage ?? null;
  } catch {
    return null;
  }
}

/** quietStorage keeps interface preferences on the device; a failed read or write falls back to defaults. */
export const quietStorage = {
  getItem(key: string): string | null {
    try {
      return browserStorage()?.getItem(key) ?? null;
    } catch {
      return null;
    }
  },
  setItem(key: string, value: string): void {
    try {
      browserStorage()?.setItem(key, value);
    } catch {
      // The default comes back next time.
    }
  },
};
