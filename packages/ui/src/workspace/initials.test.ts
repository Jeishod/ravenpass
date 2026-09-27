import assert from "node:assert/strict";
import test from "node:test";
import { initial } from "./initials.ts";

test("the first letter or digit of a title becomes the tile letter", () => {
  assert.equal(initial("GitHub"), "G");
  assert.equal(initial("  figma  "), "F");
  assert.equal(initial("нотion"), "Н");
  assert.equal(initial("1Password"), "1");
});

test("a title with nothing to show leaves the letter empty", () => {
  assert.equal(initial(""), "");
  assert.equal(initial("   "), "");
  assert.equal(initial("— …"), "");
});

test("punctuation and symbols are skipped rather than shown", () => {
  assert.equal(initial("@work mail"), "W");
  assert.equal(initial("«Хранилище»"), "Х");
});
