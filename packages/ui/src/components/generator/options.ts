import type { GeneratorOptions } from "../../vault-api.ts";

/** The options set with a number. */
export type NumericOption = "length" | "minNumbers" | "minSymbols" | "words";

/** The limits the Go generator accepts. */
export const generatorLimits: Record<
  NumericOption,
  { min: number; max: number }
> = {
  length: { min: 5, max: 128 },
  minNumbers: { min: 0, max: 9 },
  minSymbols: { min: 0, max: 9 },
  words: { min: 3, max: 20 },
};

const clamp = (value: number, min: number, max: number) =>
  Math.min(max, Math.max(min, Math.round(value)));

/**
 * clampOption sets one number within its limits and keeps the password's guaranteed characters within its length:
 * a longer minimum lengthens the password, a shorter length lowers the minimums.
 */
export function clampOption(
  options: GeneratorOptions,
  option: NumericOption,
  value: number,
): GeneratorOptions {
  const limits = generatorLimits[option];
  const next = { ...options, [option]: clamp(value, limits.min, limits.max) };
  const required = (candidate: GeneratorOptions) =>
    (candidate.uppercase ? 1 : 0) +
    (candidate.lowercase ? 1 : 0) +
    (candidate.numbers ? Math.max(1, candidate.minNumbers) : 0) +
    (candidate.symbols ? Math.max(1, candidate.minSymbols) : 0);
  if (option === "length") {
    while (required(next) > next.length) {
      if (next.symbols && next.minSymbols > 1) next.minSymbols--;
      else if (next.numbers && next.minNumbers > 1) next.minNumbers--;
      else break;
    }
  } else if (option === "minNumbers" || option === "minSymbols") {
    next.length = Math.max(next.length, required(next));
  }
  return next;
}

export type CharacterStyle = "letter" | "number" | "symbol";

/** styledCharacters splits a value into runs of letters, numbers and symbols, so each can be told apart. */
export function styledCharacters(
  value: string,
): { text: string; style: CharacterStyle }[] {
  const parts: { text: string; style: CharacterStyle }[] = [];
  for (const character of value) {
    const style: CharacterStyle = /\p{L}/u.test(character)
      ? "letter"
      : /\p{N}/u.test(character)
        ? "number"
        : "symbol";
    const last = parts.at(-1);
    if (last?.style === style) last.text += character;
    else parts.push({ text: character, style });
  }
  return parts;
}
