import {
  isArgumentElement,
  isLiteralElement,
} from "@formatjs/icu-messageformat-parser";
import { parseMessage } from "./format.ts";
import { type Language, languages } from "./language.ts";
import { catalogs, type MessageKey } from "./messages.ts";

/** Each of a host's own string names with its message's text in every language. */
export type ExportedMessages = Record<string, Record<Language, string>>;

export interface ExportOptions {
  /** Admits messages with `{name}` placeholders, for a host that fills them itself. */
  readonly values?: boolean;
}

function isMessageKey(key: unknown): key is MessageKey {
  return typeof key === "string" && key in catalogs.en;
}

/** plainText writes an ICU message as literal text with `{name}` placeholders, null when it holds a plural, select or format a host cannot fill. */
function plainText(message: string): { text: string; values: boolean } | null {
  let text = "";
  let values = false;
  for (const element of parseMessage(message)) {
    if (isLiteralElement(element)) {
      text += element.value;
    } else if (isArgumentElement(element)) {
      text += `{${element.value}}`;
      values = true;
    } else {
      return null;
    }
  }
  return { text, values };
}

/** exportMessages returns the messages a host names, by the host's own names. It fails for a key the catalogs lack, for a message a host cannot fill and, unless options allow values, for a message with placeholders. */
export function exportMessages(
  names: Readonly<Record<string, unknown>>,
  options: ExportOptions = {},
): ExportedMessages {
  const exported: ExportedMessages = {};
  for (const [name, key] of Object.entries(names)) {
    if (!isMessageKey(key)) {
      throw new Error(`${name} names ${String(key)}, which no catalog holds.`);
    }
    const texts = {} as Record<Language, string>;
    for (const language of languages) {
      const plain = plainText(catalogs[language][key]);
      if (!plain) {
        throw new Error(
          `${name} names ${key}, whose ${language} message a host cannot fill.`,
        );
      }
      if (plain.values && !options.values) {
        throw new Error(`${name} names ${key}, whose message takes values.`);
      }
      texts[language] = plain.text;
    }
    exported[name] = texts;
  }
  return exported;
}
