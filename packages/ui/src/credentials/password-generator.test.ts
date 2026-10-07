import assert from "node:assert/strict";
import test from "node:test";
import {
  characterRuns,
  defaultGeneratorOptions,
  fitGeneratorOptions,
  PasswordGenerator,
  type RandomIndex,
  randomIndex,
  readGeneratorOptions,
} from "./password-generator.ts";

const words = ["orbit", "velvet", "canyon", "ladder"];

/** A source that returns the given draws in turn, each below its bound. */
function scripted(...draws: number[]): RandomIndex {
  let next = 0;
  return (bound) => {
    const value = draws[next++ % draws.length] ?? 0;
    assert.ok(value < bound, `draw ${value} is not below ${bound}`);
    return value;
  };
}

test("randomIndex stays below its bound and refuses an empty range", () => {
  for (const bound of [1, 2, 7, 2048]) {
    for (let i = 0; i < 200; i++) {
      const value = randomIndex(bound);
      assert.ok(Number.isInteger(value) && value >= 0 && value < bound);
    }
  }
  assert.throws(() => randomIndex(0), RangeError);
  assert.throws(() => randomIndex(1.5), RangeError);
});

test("the default is five capitalized words joined by hyphens with one digit", () => {
  // Words 0, 1, 2, 3, 0; the digit goes after word 2 and is 7.
  const generator = new PasswordGenerator(words, scripted(0, 1, 2, 3, 0, 2, 7));
  const { password, bits } = generator.generate(defaultGeneratorOptions);
  assert.equal(password, "Orbit-Velvet-Canyon7-Ladder-Orbit");
  assert.equal(bits, 5 * Math.log2(4) + Math.log2(10 * 5));
});

test("word options change the count, the separator and the capitals", () => {
  const generator = new PasswordGenerator(words, scripted(3, 2, 1));
  const { password, bits } = generator.generate({
    ...defaultGeneratorOptions,
    words: 3,
    separator: " ",
    capitalize: false,
    digit: false,
  });
  assert.equal(password, "ladder canyon velvet");
  assert.equal(bits, 3 * Math.log2(4));
});

test("an out-of-bounds count is brought within bounds", () => {
  const generator = new PasswordGenerator(words);
  const short = generator.generate({
    ...defaultGeneratorOptions,
    words: 1,
    digit: false,
  });
  assert.equal(short.password.split("-").length, 3);
  const long = generator.generate({
    ...defaultGeneratorOptions,
    mode: "characters",
    length: 500,
  });
  assert.equal(long.password.length, 64);
});

test("character passwords use every chosen class and no other", () => {
  const generator = new PasswordGenerator(words);
  for (let i = 0; i < 50; i++) {
    const { password } = generator.generate({
      ...defaultGeneratorOptions,
      mode: "characters",
      length: 8,
    });
    assert.match(password, /[a-z]/);
    assert.match(password, /[A-Z]/);
    assert.match(password, /[0-9]/);
    assert.match(password, /[^a-zA-Z0-9]/);
  }
  const { password, bits } = generator.generate({
    ...defaultGeneratorOptions,
    mode: "characters",
    length: 20,
    uppercase: false,
    digits: false,
    symbols: false,
  });
  assert.match(password, /^[a-z]{20}$/);
  assert.ok(Math.abs(bits - 20 * Math.log2(26)) < 1e-9, `${bits} bits`);
});

test("character passwords hold at least the minimum digits and symbols", () => {
  const generator = new PasswordGenerator(words);
  const options = {
    ...defaultGeneratorOptions,
    mode: "characters" as const,
    length: 12,
    minDigits: 4,
    minSymbols: 3,
  };
  for (let i = 0; i < 50; i++) {
    const { password } = generator.generate(options);
    assert.equal(password.length, 12);
    assert.ok((password.match(/[0-9]/g) ?? []).length >= 4, password);
    assert.ok((password.match(/[^a-zA-Z0-9]/g) ?? []).length >= 3, password);
  }
});

test("avoiding ambiguous characters leaves out I, O, l, 0 and 1", () => {
  const generator = new PasswordGenerator(words);
  for (let i = 0; i < 50; i++) {
    const { password } = generator.generate({
      ...defaultGeneratorOptions,
      mode: "characters",
      length: 64,
      avoidAmbiguous: true,
    });
    assert.doesNotMatch(password, /[IOl01]/);
  }
});

test("the bits count every draw from its own class", () => {
  const generator = new PasswordGenerator(words, () => 0);
  const { password, bits } = generator.generate({
    ...defaultGeneratorOptions,
    mode: "characters",
    length: 8,
    symbols: false,
    minDigits: 2,
  });
  assert.equal([...password].sort().join(""), "00Aaaaaa");
  const pool = 26 + 26 + 10;
  const expected = 2 * Math.log2(26) + 2 * Math.log2(10) + 4 * Math.log2(pool);
  assert.ok(Math.abs(bits - expected) < 1e-9, `${bits} bits`);
});

test("minimums and length keep the guaranteed characters within the password", () => {
  const options = {
    ...defaultGeneratorOptions,
    mode: "characters" as const,
    length: 8,
  };
  assert.equal(
    fitGeneratorOptions({ ...options, minDigits: 9 }, "minDigits").length,
    12,
  );
  const shortened = fitGeneratorOptions(
    { ...options, minDigits: 4, minSymbols: 4, length: 8 },
    "length",
  );
  assert.deepEqual([shortened.minDigits, shortened.minSymbols], [4, 2]);
});

test("characterRuns splits a password into letters, digits and symbols", () => {
  assert.deepEqual(characterRuns("ab12#c"), [
    { text: "ab", kind: "letter" },
    { text: "12", kind: "digit" },
    { text: "#", kind: "symbol" },
    { text: "c", kind: "letter" },
  ]);
});

test("strength follows the entropy", () => {
  assert.equal(PasswordGenerator.strength(39.9), "weak");
  assert.equal(PasswordGenerator.strength(40), "fair");
  assert.equal(PasswordGenerator.strength(61), "strong");
  assert.equal(PasswordGenerator.strength(80), "very-strong");
});

test("stored options keep what is valid and fall back for the rest", () => {
  assert.deepEqual(readGeneratorOptions(null), defaultGeneratorOptions);
  assert.deepEqual(readGeneratorOptions("{"), defaultGeneratorOptions);
  assert.deepEqual(readGeneratorOptions("7"), defaultGeneratorOptions);
  assert.deepEqual(
    readGeneratorOptions(
      JSON.stringify({
        mode: "characters",
        words: 11,
        separator: "/",
        capitalize: false,
        length: 32,
        symbols: "yes",
      }),
    ),
    {
      ...defaultGeneratorOptions,
      mode: "characters",
      capitalize: false,
      length: 32,
    },
  );
});
