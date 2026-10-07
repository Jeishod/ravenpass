/** Words joined into a phrase, or characters drawn one by one. */
export type GeneratorMode = "words" | "characters";

export type WordSeparator = "-" | "." | "_" | " ";

export const wordSeparators: readonly WordSeparator[] = ["-", ".", "_", " "];

export interface GeneratorOptions {
  mode: GeneratorMode;
  words: number;
  separator: WordSeparator;
  /** Starts every word with a capital. */
  capitalize: boolean;
  /** Adds one digit to the end of one word. */
  digit: boolean;
  length: number;
  uppercase: boolean;
  digits: boolean;
  symbols: boolean;
  /** How many digits a password holds at least, when digits are on. */
  minDigits: number;
  /** How many symbols a password holds at least, when symbols are on. */
  minSymbols: number;
  /** Leaves out the characters easily taken for one another: I, O, l, 0 and 1. */
  avoidAmbiguous: boolean;
}

export const generatorBounds = {
  words: { min: 3, max: 10 },
  length: { min: 8, max: 64 },
  minimum: { min: 1, max: 9 },
} as const;

export const defaultGeneratorOptions: GeneratorOptions = {
  mode: "words",
  words: 5,
  separator: "-",
  capitalize: true,
  digit: true,
  length: 20,
  uppercase: true,
  digits: true,
  symbols: true,
  minDigits: 1,
  minSymbols: 1,
  avoidAmbiguous: false,
};

/** How hard a password is to guess, from its entropy. */
export type Strength = "weak" | "fair" | "strong" | "very-strong";

export interface GeneratedPassword {
  password: string;
  mode: GeneratorMode;
  /** Entropy in bits, given the options and the list it was drawn from. */
  bits: number;
}

const lowercase = "abcdefghijklmnopqrstuvwxyz";
const uppercase = lowercase.toUpperCase();
const digits = "0123456789";
const symbols = "!#$%&*+-=?@^_";
const ambiguous = /[IOl01]/g;

/** Draws an integer from 0 up to bound, exclusive, uniformly. */
export type RandomIndex = (bound: number) => number;

const uint32Range = 2 ** 32;

/** randomIndex draws with the platform's cryptographic source, rejecting draws past the last whole multiple of bound. */
export function randomIndex(bound: number): number {
  if (!Number.isInteger(bound) || bound < 1 || bound > uint32Range) {
    throw new RangeError(`cannot draw below ${bound}`);
  }
  const limit = uint32Range - (uint32Range % bound);
  const draw = new Uint32Array(1);
  for (;;) {
    crypto.getRandomValues(draw);
    const value = draw[0] ?? 0;
    if (value < limit) return value % bound;
  }
}

/** PasswordGenerator makes passwords from a word list or from character classes. */
export class PasswordGenerator {
  readonly #words: readonly string[];
  readonly #random: RandomIndex;

  constructor(words: readonly string[], random: RandomIndex = randomIndex) {
    if (words.length < 2) throw new RangeError("the word list is too short");
    this.#words = words;
    this.#random = random;
  }

  generate(options: GeneratorOptions): GeneratedPassword {
    return options.mode === "words"
      ? this.#phrase(options)
      : this.#characters(options);
  }

  static strength(bits: number): Strength {
    if (bits < 40) return "weak";
    if (bits < 60) return "fair";
    if (bits < 80) return "strong";
    return "very-strong";
  }

  #phrase(options: GeneratorOptions): GeneratedPassword {
    const count = clamp(options.words, generatorBounds.words);
    const words = Array.from({ length: count }, () => {
      const word = this.#pick(this.#words);
      return options.capitalize ? capitalized(word) : word;
    });
    let bits = count * Math.log2(this.#words.length);
    if (options.digit) {
      const at = this.#random(count);
      words[at] = `${words[at]}${this.#pick(digits)}`;
      bits += Math.log2(digits.length * count);
    }
    return { password: words.join(options.separator), mode: "words", bits };
  }

  // Each guaranteed character is drawn from its class and the rest from every chosen class, then all are shuffled.
  // The bits count the draws and not the shuffle, so the strength never overstates.
  #characters(options: GeneratorOptions): GeneratedPassword {
    const fitted = fitGeneratorOptions(
      { ...options, length: clamp(options.length, generatorBounds.length) },
      "length",
    );
    const classes = characterClasses(fitted.avoidAmbiguous);
    const chosen = [classes.lowercase];
    const guaranteed = [classes.lowercase];
    if (fitted.uppercase) {
      chosen.push(classes.uppercase);
      guaranteed.push(classes.uppercase);
    }
    if (fitted.digits) {
      chosen.push(classes.digits);
      guaranteed.push(...Array(fitted.minDigits).fill(classes.digits));
    }
    if (fitted.symbols) {
      chosen.push(symbols);
      guaranteed.push(...Array(fitted.minSymbols).fill(symbols));
    }
    const pool = chosen.join("");
    const sets = [
      ...guaranteed,
      ...Array(fitted.length - guaranteed.length).fill(pool),
    ];
    const characters = sets.map((set) => this.#pick(set));
    for (let i = characters.length - 1; i > 0; i--) {
      const j = this.#random(i + 1);
      [characters[i], characters[j]] = [
        characters[j] ?? "",
        characters[i] ?? "",
      ];
    }
    const bits = sets.reduce((sum, set) => sum + Math.log2(set.length), 0);
    return { password: characters.join(""), mode: "characters", bits };
  }

  #pick(from: string | readonly string[]): string {
    return from[this.#random(from.length)] ?? "";
  }
}

function characterClasses(avoidAmbiguous: boolean) {
  const kept = (set: string) =>
    avoidAmbiguous ? set.replace(ambiguous, "") : set;
  return {
    lowercase: kept(lowercase),
    uppercase: kept(uppercase),
    digits: kept(digits),
  };
}

/** How many characters options make a password hold at least. */
function guaranteedCount(options: GeneratorOptions): number {
  return (
    1 +
    (options.uppercase ? 1 : 0) +
    (options.digits ? options.minDigits : 0) +
    (options.symbols ? options.minSymbols : 0)
  );
}

/**
 * fitGeneratorOptions keeps the characters a password must hold within its length after `changed` changed: a longer
 * minimum lengthens the password, a shorter length lowers the minimums, symbols first.
 */
export function fitGeneratorOptions(
  options: GeneratorOptions,
  changed: "length" | "minDigits" | "minSymbols",
): GeneratorOptions {
  const next = {
    ...options,
    minDigits: clamp(options.minDigits, generatorBounds.minimum),
    minSymbols: clamp(options.minSymbols, generatorBounds.minimum),
  };
  if (changed !== "length") {
    next.length = Math.max(next.length, guaranteedCount(next));
    return next;
  }
  while (guaranteedCount(next) > next.length) {
    if (next.symbols && next.minSymbols > 1) next.minSymbols--;
    else if (next.digits && next.minDigits > 1) next.minDigits--;
    else break;
  }
  return next;
}

export type CharacterKind = "letter" | "digit" | "symbol";

/** characterRuns splits a password into runs of letters, digits and symbols, so each can be shown apart. */
export function characterRuns(
  password: string,
): { text: string; kind: CharacterKind }[] {
  const runs: { text: string; kind: CharacterKind }[] = [];
  for (const character of password) {
    const kind: CharacterKind = /\p{L}/u.test(character)
      ? "letter"
      : /\p{N}/u.test(character)
        ? "digit"
        : "symbol";
    const last = runs.at(-1);
    if (last?.kind === kind) last.text += character;
    else runs.push({ text: character, kind });
  }
  return runs;
}

function capitalized(word: string): string {
  return word.charAt(0).toUpperCase() + word.slice(1);
}

function clamp(value: number, bounds: { min: number; max: number }): number {
  return Math.min(bounds.max, Math.max(bounds.min, Math.round(value)));
}

/** readGeneratorOptions reads stored options, keeping each valid one and the default for the rest. */
export function readGeneratorOptions(stored: string | null): GeneratorOptions {
  let parsed: unknown;
  try {
    parsed = JSON.parse(stored ?? "null");
  } catch {
    return defaultGeneratorOptions;
  }
  if (typeof parsed !== "object" || parsed === null) {
    return defaultGeneratorOptions;
  }
  const value = parsed as Record<string, unknown>;
  const defaults = defaultGeneratorOptions;
  const flag = (key: keyof GeneratorOptions, fallback: boolean) =>
    typeof value[key] === "boolean" ? value[key] : fallback;
  const count = (
    key: "words" | "length" | "minDigits" | "minSymbols",
    bounds: { min: number; max: number },
  ) => {
    const number = value[key];
    return typeof number === "number" &&
      Number.isInteger(number) &&
      number >= bounds.min &&
      number <= bounds.max
      ? number
      : defaults[key];
  };
  const separator = wordSeparators.find((item) => item === value.separator);
  return fitGeneratorOptions(
    {
      mode:
        value.mode === "words" || value.mode === "characters"
          ? value.mode
          : defaults.mode,
      words: count("words", generatorBounds.words),
      separator: separator ?? defaults.separator,
      capitalize: flag("capitalize", defaults.capitalize),
      digit: flag("digit", defaults.digit),
      length: count("length", generatorBounds.length),
      uppercase: flag("uppercase", defaults.uppercase),
      digits: flag("digits", defaults.digits),
      symbols: flag("symbols", defaults.symbols),
      minDigits: count("minDigits", generatorBounds.minimum),
      minSymbols: count("minSymbols", generatorBounds.minimum),
      avoidAmbiguous: flag("avoidAmbiguous", defaults.avoidAmbiguous),
    },
    "length",
  );
}
