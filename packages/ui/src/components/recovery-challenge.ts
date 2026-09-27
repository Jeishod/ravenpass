import { normalizedWord } from "../seeds/phrase.ts";

/** How many words a recovery key has. */
export const recoveryKeyWords = 24;

/** How many words a challenge asks for, fewer only when the phrase is shorter. */
const challengeCount = 3;

const RANDOM_RANGE = 0x1_0000_0000;

function secureRandomWord(): number {
  const [value = 0] = crypto.getRandomValues(new Uint32Array(1));
  return value;
}

function askedCount(wordCount: number): number {
  return Math.min(challengeCount, wordCount);
}

/** chooseWordPositions picks distinct sorted positions among `wordCount` words, free of modulo bias. */
export function chooseWordPositions(
  wordCount: number,
  randomWord: () => number = secureRandomWord,
): number[] {
  if (!Number.isInteger(wordCount) || wordCount <= 0) return [];
  const unbiasedLimit = Math.floor(RANDOM_RANGE / wordCount) * wordCount;
  const wanted = askedCount(wordCount);
  const positions = new Set<number>();
  while (positions.size < wanted) {
    const value = randomWord();
    if (value < 0 || !Number.isInteger(value) || value >= unbiasedLimit) {
      continue;
    }
    positions.add(value % wordCount);
  }
  return [...positions].sort((first, second) => first - second);
}

/** matchesWordChallenge compares answers ignoring case, surrounding spaces and Unicode form. */
export function matchesWordChallenge(
  words: readonly string[],
  positions: readonly number[],
  answers: readonly string[],
): boolean {
  const wanted = askedCount(words.length);
  if (
    wanted === 0 ||
    positions.length !== wanted ||
    answers.length !== wanted ||
    new Set(positions).size !== wanted
  ) {
    return false;
  }
  return positions.every((position, index) => {
    const word = words[position];
    const answer = answers[index];
    return (
      word !== undefined &&
      answer !== undefined &&
      normalizedWord(answer) === normalizedWord(word)
    );
  });
}
