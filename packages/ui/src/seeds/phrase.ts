import type { MessageKey } from "../i18n/messages.ts";
import type {
  BackupCode,
  SeedChecksum,
  SeedFormat,
  SeedSummary,
} from "../vault-api.ts";

/** In the order the editor offers them. */
export const seedFormats: readonly SeedFormat[] = ["phrase", "key", "codes"];

export function isSeedFormat(value: string): value is SeedFormat {
  return (seedFormats as readonly string[]).includes(value);
}

const seedChecksums: readonly SeedChecksum[] = [
  "valid",
  "invalid",
  "unknown",
  "",
];

export function isSeedChecksum(value: string): value is SeedChecksum {
  return (seedChecksums as readonly string[]).includes(value);
}

/** Inclusive. */
const lowCodesLeft = 3;

/** A phrase grid takes four columns above this many words. */
const threeColumnWords = 12;

const completionLimit = 5;

export function splitWords(text: string): string[] {
  return text.split(/\s+/u).filter(Boolean);
}

/** One trimmed backup code per line; blank lines are left out. */
export function codeLines(text: string): string[] {
  return text
    .split(/\r?\n/u)
    .map((line) => line.trim())
    .filter(Boolean);
}

/** `start` and `end` are offsets in the text. */
export interface WordAtCaret {
  word: string;
  start: number;
  end: number;
}

function isSpace(text: string, index: number): boolean {
  return /\s/u.test(text.charAt(index));
}

/** Between two spaces the word is empty. */
export function wordAt(text: string, caret: number): WordAtCaret {
  const at = Math.min(Math.max(caret, 0), text.length);
  let start = at;
  while (start > 0 && !isSpace(text, start - 1)) start -= 1;
  let end = at;
  while (end < text.length && !isSpace(text, end)) end += 1;
  return { word: text.slice(start, end), start, end };
}

/** Replaces the word at the caret and adds one space; the new caret follows that space. */
export function completeWord(
  text: string,
  caret: number,
  word: string,
): { text: string; caret: number } {
  const { start, end } = wordAt(text, caret);
  const after = text.slice(end).replace(/^\s+/u, "");
  const next = `${text.slice(0, start)}${word} ${after}`;
  return { text: next, caret: start + word.length + 1 };
}

/** Unicode NFKD, lower case, as the vault stores words. */
export function normalizedWord(word: string): string {
  return word.trim().normalize("NFKD").toLowerCase();
}

/** In list order; a prefix that already is the only match offers nothing. */
export function completions(
  prefix: string,
  words: readonly string[],
  limit = completionLimit,
): string[] {
  const start = normalizedWord(prefix);
  if (!start) return [];
  const found: string[] = [];
  for (const word of words) {
    if (!word.startsWith(start)) continue;
    found.push(word);
    if (found.length === limit) break;
  }
  return found.length === 1 && found[0] === start ? [] : found;
}

export function phraseColumns(words: number): 3 | 4 {
  return words > threeColumnWords ? 4 : 3;
}

/** -1 when every code is used. */
export function nextUnusedCode(codes: readonly BackupCode[]): number {
  return codes.findIndex((code) => !code.used);
}

/** Each code keeps the used mark recorded for its value. */
export function keptCodes(
  lines: readonly string[],
  used: ReadonlySet<string>,
): BackupCode[] {
  return lines.map((value) => ({ value, used: used.has(value) }));
}

export interface SeedLabel {
  key: MessageKey;
  values?: Record<string, number>;
  warning: boolean;
}

export function summaryLabel(
  summary: Pick<SeedSummary, "format" | "total" | "used">,
): SeedLabel {
  switch (summary.format) {
    case "phrase":
      return {
        key: "seed.summary.words",
        values: { count: summary.total },
        warning: false,
      };
    case "codes": {
      const left = summary.total - summary.used;
      return {
        key: "seed.summary.codes",
        values: { left, total: summary.total },
        warning: left <= lowCodesLeft,
      };
    }
    case "key":
      return { key: "seed.summary.key", warning: false };
  }
}

export function checksumLabel(checksum: Exclude<SeedChecksum, "">): SeedLabel {
  switch (checksum) {
    case "valid":
      return { key: "seed.checksum.valid", warning: false };
    case "invalid":
      return { key: "seed.checksum.invalid", warning: true };
    case "unknown":
      return { key: "seed.checksum.unknown", warning: false };
  }
}
