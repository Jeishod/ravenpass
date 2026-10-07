import assert from "node:assert/strict";
import { test } from "node:test";
import type { GeneratorOptions } from "../../vault-api.ts";
import { clampOption, styledCharacters } from "./options.ts";

const defaults: GeneratorOptions = {
  kind: "password",
  length: 14,
  uppercase: true,
  lowercase: true,
  numbers: true,
  symbols: true,
  minNumbers: 1,
  minSymbols: 1,
  avoidAmbiguous: false,
  words: 6,
  separator: "-",
  capitalize: false,
  includeNumber: false,
};

test("a number past its limits is clamped", () => {
  assert.equal(clampOption(defaults, "length", 500).length, 128);
  assert.equal(clampOption(defaults, "length", 1).length, 5);
  assert.equal(clampOption(defaults, "words", 2).words, 3);
  assert.equal(clampOption(defaults, "minNumbers", 4.6).minNumbers, 5);
});

test("raising a minimum lengthens a password too short to hold it", () => {
  const short = { ...defaults, length: 5 };
  const next = clampOption(short, "minSymbols", 9);
  assert.equal(next.minSymbols, 9);
  assert.equal(next.length, 1 + 1 + 1 + 9);
});

test("shortening a password lowers the minimums it can no longer hold", () => {
  const demanding = { ...defaults, length: 20, minNumbers: 9, minSymbols: 9 };
  const next = clampOption(demanding, "length", 5);
  assert.equal(next.length, 5);
  assert.ok(2 + next.minNumbers + next.minSymbols <= 5);
});

test("a value splits into runs of letters, numbers and symbols", () => {
  assert.deepEqual(styledCharacters("ab12#c"), [
    { text: "ab", style: "letter" },
    { text: "12", style: "number" },
    { text: "#", style: "symbol" },
    { text: "c", style: "letter" },
  ]);
});
