import { type Language, languages } from "@ravenpass/ui/i18n/language.ts";
import type {
  LanguageSettings,
  LanguageSource,
} from "@ravenpass/ui/vault-api.ts";

const chosenKey = "language";
const desktopKey = "desktopLanguage";
const keys = [desktopKey, chosenKey];

type Changes = Record<string, unknown>;

/** `chrome.storage.local` as far as the extension's language uses it. */
export interface LanguageArea {
  get(keys: string[]): Promise<Record<string, unknown>>;
  set(items: Record<string, unknown>): Promise<void>;
  remove(keys: string): Promise<void>;
  readonly onChanged: {
    addListener(listener: (changes: Changes) => void): void;
    removeListener(listener: (changes: Changes) => void): void;
  };
}

/** Where the link client keeps the language the desktop app reports while linked. */
export interface DesktopLanguageStore {
  keepDesktopLanguage(language: Language): Promise<void>;
  forgetDesktopLanguage(): Promise<void>;
}

export interface StoredLanguageDependencies {
  readonly area?: LanguageArea;
  readonly browserLanguage?: () => string;
}

/** The desktop app's language while linked, else the one chosen in the popup, else the browser's. */
export class StoredLanguage implements LanguageSource, DesktopLanguageStore {
  private readonly area: LanguageArea;
  private readonly browserLanguage: () => string;

  constructor({
    area = chrome.storage.local,
    browserLanguage = () => chrome.i18n.getUILanguage(),
  }: StoredLanguageDependencies = {}) {
    this.area = area;
    this.browserLanguage = browserLanguage;
  }

  async getLanguage(): Promise<LanguageSettings> {
    const stored = await this.area.get(keys);
    const language = keys
      .map((key) => stored[key])
      .find((value) => typeof value === "string");
    if (typeof language === "string") {
      return { languages: [...languages], language, chosen: true };
    }
    return {
      languages: [...languages],
      language: this.browserLanguage(),
      chosen: false,
    };
  }

  async setLanguage(language: string): Promise<void> {
    await this.area.set({ [chosenKey]: language });
  }

  async keepDesktopLanguage(language: Language): Promise<void> {
    await this.area.set({ [desktopKey]: language });
  }

  async forgetDesktopLanguage(): Promise<void> {
    await this.area.remove(desktopKey);
  }

  watchLanguage(onChange: () => void): () => void {
    const listener = (changes: Changes) => {
      if (keys.some((key) => key in changes)) onChange();
    };
    this.area.onChanged.addListener(listener);
    return () => this.area.onChanged.removeListener(listener);
  }
}
