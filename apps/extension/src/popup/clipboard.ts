import { ConnectionKey } from "../link/key.ts";

const clipboardRead: chrome.permissions.Permissions = {
  permissions: ["clipboardRead"],
};

/** Reads a connection key from the clipboard under the optional clipboardRead permission. */
export class ClipboardKeys {
  async granted(): Promise<string | null> {
    if (!(await chrome.permissions.contains(clipboardRead))) return null;
    return this.read();
  }

  /** Chrome asks only during a click and may close the popup to ask; the next opening reads through `granted`. */
  async request(): Promise<string | null> {
    if (!(await chrome.permissions.request(clipboardRead))) {
      throw new Error("Clipboard access was not granted.");
    }
    return this.read();
  }

  async release(): Promise<void> {
    await chrome.permissions.remove(clipboardRead);
  }

  private async read(): Promise<string | null> {
    const text = (await navigator.clipboard.readText()).trim();
    try {
      await ConnectionKey.parse(text);
      return text;
    } catch {
      return null;
    }
  }
}
