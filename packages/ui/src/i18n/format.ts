import {
  type MessageFormatElement,
  parse,
} from "@formatjs/icu-messageformat-parser";
import { IntlMessageFormat } from "intl-messageformat";
import type { Language } from "./language.ts";
import { catalogs, type MessageKey } from "./messages.ts";

export type MessageValues = Readonly<Record<string, string | number>>;

/** parseMessage reads a catalog message as ICU MessageFormat, with `<` and `>` as plain text. */
export function parseMessage(message: string): MessageFormatElement[] {
  return parse(message, { ignoreTag: true });
}

/** MessageFormatter formats catalog messages as ICU MessageFormat, parsing each message once per language. */
export class MessageFormatter {
  readonly #formats = new Map<string, IntlMessageFormat>();

  /** Throws when the message uses a value that `values` lacks. */
  format(language: Language, key: MessageKey, values?: MessageValues): string {
    const cacheKey = `${language}:${key}`;
    let format = this.#formats.get(cacheKey);
    if (!format) {
      format = new IntlMessageFormat(
        parseMessage(catalogs[language][key]),
        language,
      );
      this.#formats.set(cacheKey, format);
    }
    return String(format.format(values));
  }
}

export const messageFormatter = new MessageFormatter();
