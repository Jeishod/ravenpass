import type { LanguageSource } from "../vault-api.ts";
import { type Language, matchLanguage } from "./language.ts";

/** LanguageFollower reads the language a source reports, and again after each change a watchable source reports. */
export class LanguageFollower {
  private readonly source: LanguageSource;

  constructor(source: LanguageSource) {
    this.source = source;
  }

  /** The language in use now, English when the source cannot be read. */
  async current(): Promise<Language> {
    const settings = await this.source.getLanguage().catch(() => null);
    return matchLanguage(settings?.language ?? "");
  }

  /** Reports until the returned function is called; a read that a later one overtakes reports nothing. */
  follow(onLanguage: (language: Language) => void): () => void {
    let latest = 0;
    let stopped = false;
    const read = async () => {
      latest += 1;
      const current = latest;
      const settings = await this.source.getLanguage().catch(() => null);
      if (!settings || stopped || current !== latest) return;
      onLanguage(matchLanguage(settings.language));
    };
    const unwatch = this.source.watchLanguage?.(() => void read());
    void read();
    return () => {
      stopped = true;
      unwatch?.();
    };
  }
}
