import assert from "node:assert/strict";
import test from "node:test";
import { splitWords } from "../seeds/phrase.ts";
import {
  chooseWordPositions,
  matchesWordChallenge,
} from "./recovery-challenge.ts";

function phraseOf(count: number): string[] {
  return Array.from({ length: count }, (_, index) => `word${index + 1}`);
}

/** Hands out `values` in order, then throws once they run out. */
function drawing(values: readonly number[]): () => number {
  const queue = [...values];
  return () => {
    const value = queue.shift();
    if (value === undefined) throw new Error("No random value left.");
    return value;
  };
}

const phrase = phraseOf(24);

test("repeated random values still select three distinct word positions", () => {
  const positions = chooseWordPositions(
    24,
    drawing([2, 2, 0xffff_ffff, 25, 7]),
  );
  assert.deepEqual(positions, [1, 2, 7]);
});

test("positions over twelve words stay in range and skip biased values", () => {
  // 0xffff_fffc is the first value at or above the largest multiple of 12 below 2^32.
  const positions = chooseWordPositions(
    12,
    drawing([0xffff_fffc, 13, 13, 11, 30]),
  );
  assert.deepEqual(positions, [1, 6, 11]);
});

test("a phrase shorter than the challenge asks for every word", () => {
  assert.deepEqual(chooseWordPositions(2, drawing([1, 0, 1])), [0, 1]);
  assert.deepEqual(chooseWordPositions(0), []);
});

test("secure positions are distinct and within the phrase", () => {
  for (const count of [12, 24]) {
    const positions = chooseWordPositions(count);
    assert.equal(positions.length, 3);
    assert.equal(new Set(positions).size, 3);
    assert.ok(positions.every((position) => position >= 0 && position < count));
  }
});

test("missing or incorrect recovery words cannot pass confirmation", () => {
  const positions = [1, 8, 21];
  assert.equal(
    matchesWordChallenge(phrase, positions, ["WORD2", "word9", "word22"]),
    true,
  );
  assert.equal(
    matchesWordChallenge(phrase, positions, ["word2", "", "word22"]),
    false,
  );
  assert.equal(
    matchesWordChallenge(phrase, positions, ["word2", "word8", "word22"]),
    false,
  );
  assert.equal(
    matchesWordChallenge(phrase, [1, 1, 21], ["word2", "word2", "word22"]),
    false,
  );
});

test("a twelve-word challenge compares against its own words", () => {
  const twelve = phraseOf(12);
  assert.equal(
    matchesWordChallenge(twelve, [0, 5, 11], [" word1 ", "Word6", "word12"]),
    true,
  );
  assert.equal(
    matchesWordChallenge(twelve, [0, 5, 12], ["word1", "word6", "word13"]),
    false,
  );
  assert.equal(
    matchesWordChallenge(twelve, [-1, 5, 1.5], ["word1", "word6", "word2"]),
    false,
  );
  assert.equal(
    matchesWordChallenge(splitWords(" a  b\nc "), [0, 1, 2], ["A", "b", "c"]),
    true,
  );
});
