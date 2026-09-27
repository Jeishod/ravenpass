import { splitWords } from "../seeds/phrase.ts";

/** What typing or pasting into a field made of the entry, and the field to go on in. */
export interface PhraseEdit {
  entry: PhraseEntry;
  focus: number;
}

/** PhraseEntry holds one lower-case word per linked field; several words fill on from the field they entered. */
export class PhraseEntry {
  readonly words: readonly string[];

  private constructor(words: readonly string[]) {
    this.words = words;
  }

  static empty(count: number): PhraseEntry {
    return new PhraseEntry(Array.from({ length: count }, () => ""));
  }

  /** Whether every field holds a word. */
  get complete(): boolean {
    return this.words.every(Boolean);
  }

  /** The first field without a word, or -1 once every field holds one. */
  get firstEmpty(): number {
    return this.words.indexOf("");
  }

  /** The words as one phrase, separated by single spaces. */
  get phrase(): string {
    return this.words.join(" ");
  }

  /** enter replaces field `index` with `text`; words past the last field are dropped. */
  enter(index: number, text: string): PhraseEdit {
    const typed = splitWords(text.toLowerCase());
    const words = [...this.words];
    const fitting = typed.slice(0, words.length - index);
    words[index] = "";
    fitting.forEach((word, offset) => {
      words[index + offset] = word;
    });
    const movesOn = typed.length > 1 || (typed.length > 0 && /\s$/u.test(text));
    return {
      entry: new PhraseEntry(words),
      focus: movesOn
        ? Math.min(index + fitting.length, words.length - 1)
        : index,
    };
  }

  /** paste fills on from field `index` for several words, and answers null for one. */
  paste(index: number, text: string): PhraseEdit | null {
    return splitWords(text).length > 1 ? this.enter(index, text) : null;
  }

  /** keyMove answers the field a key moves to, or null where the key acts as usual. */
  keyMove(index: number, key: string): number | null {
    const last = this.words.length - 1;
    if (key === " ") return Math.min(index + 1, last);
    if (key === "Enter") return index < last ? index + 1 : null;
    if (key === "Backspace" && index > 0 && this.words[index] === "") {
      return index - 1;
    }
    return null;
  }
}
