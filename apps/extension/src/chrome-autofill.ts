import type { LanguageArea } from "./language.ts";
import { signInStyleKey } from "./sign-in-style.ts";

const key = "chromeAutofillOff";

/** `chrome.storage.local` as far as the choice uses it. */
export type ChromeAutofillArea = Pick<
  LanguageArea,
  "get" | "set" | "onChanged"
>;

export type BooleanSetting = Pick<
  chrome.types.ChromeSetting<boolean>,
  "get" | "set" | "clear"
>;

export interface ChromeAutofillDependencies {
  readonly area?: ChromeAutofillArea;
  /** Chrome's address, payment card and password saving settings this build offers. */
  readonly settings?: () => readonly BooleanSetting[];
  /** Whether the extension is linked to the desktop app; an unlinked one offers nothing in Chrome's place. */
  readonly linked?: () => Promise<boolean>;
}

/** Chrome's own autofill, off while the extension is linked unless the owner turns it back on. */
export class ChromeAutofill {
  private readonly area: ChromeAutofillArea;
  private readonly settings: () => readonly BooleanSetting[];
  private readonly linked: () => Promise<boolean>;
  /** Each application waits for the one before, so the last choice is the one applied. */
  private applied: Promise<void> = Promise.resolve();

  constructor({
    area = chrome.storage.local,
    settings = chromeSettings,
    linked,
  }: ChromeAutofillDependencies = {}) {
    this.area = area;
    this.settings = settings;
    this.linked =
      linked ??
      (async () => (await area.get([signInStyleKey]))[signInStyleKey] != null);
  }

  async turnedOff(): Promise<boolean> {
    return (await this.area.get([key]))[key] !== false;
  }

  async turnOff(off: boolean): Promise<void> {
    await this.area.set({ [key]: off });
  }

  /**
   * Turns Chrome's autofill settings off, or releases them, as the choice and the link say; settings another extension
   * or a policy controls are left alone. Chrome restores what an extension controls once it is disabled or removed.
   */
  apply(): Promise<void> {
    const applying = this.applied.then(async () => {
      const off = (await this.turnedOff()) && (await this.linked());
      const results = await Promise.allSettled(
        this.settings().map((setting) => applyTo(setting, off)),
      );
      const failure = results.find((result) => result.status === "rejected");
      if (failure) throw failure.reason;
    });
    this.applied = applying.catch(() => {});
    return applying;
  }

  watch(onChange: () => void): () => void {
    const listener = (changes: Record<string, unknown>) => {
      if (key in changes || signInStyleKey in changes) onChange();
    };
    this.area.onChanged.addListener(listener);
    return () => this.area.onChanged.removeListener(listener);
  }
}

async function applyTo(setting: BooleanSetting, off: boolean): Promise<void> {
  const { levelOfControl } = await setting.get({});
  if (off) {
    if (
      levelOfControl === "controllable_by_this_extension" ||
      levelOfControl === "controlled_by_this_extension"
    ) {
      await setting.set({ value: false });
    }
  } else if (levelOfControl === "controlled_by_this_extension") {
    await setting.clear({});
  }
}

function chromeSettings(): BooleanSetting[] {
  const services = chrome.privacy?.services;
  return [
    services?.autofillAddressEnabled,
    services?.autofillCreditCardEnabled,
    services?.passwordSavingEnabled,
  ].filter((setting) => setting !== undefined);
}
