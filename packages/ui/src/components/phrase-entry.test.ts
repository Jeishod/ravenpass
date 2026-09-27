import assert from "node:assert/strict";
import test from "node:test";
import { PhraseEntry } from "./phrase-entry.ts";

const fields = (words: string[]) =>
  words.reduce(
    (entry, word, index) => entry.enter(index, word).entry,
    PhraseEntry.empty(words.length),
  );

test("a typed word stays in its field, in lower case", () => {
  const edit = PhraseEntry.empty(3).enter(0, "Aban");
  assert.deepEqual(edit.entry.words, ["aban", "", ""]);
  assert.equal(edit.focus, 0);
});

test("a space after a word goes on to the next field", () => {
  const edit = PhraseEntry.empty(3).enter(1, " abandon ");
  assert.deepEqual(edit.entry.words, ["", "abandon", ""]);
  assert.equal(edit.focus, 2);
});

test("spaces alone leave the field empty where it is", () => {
  const edit = PhraseEntry.empty(3).enter(1, "  ");
  assert.deepEqual(edit.entry.words, ["", "", ""]);
  assert.equal(edit.focus, 1);
});

test("the last field keeps the caret after a space", () => {
  const edit = PhraseEntry.empty(2).enter(1, "zoo ");
  assert.deepEqual(edit.entry.words, ["", "zoo"]);
  assert.equal(edit.focus, 1);
});

test("several words fill the fields from the one they went into", () => {
  const edit = fields(["one", "two", "", ""]).enter(1, "Able\nabout  above");
  assert.deepEqual(edit.entry.words, ["one", "able", "about", "above"]);
  assert.equal(edit.focus, 3);
});

test("words past the last field are left out", () => {
  const edit = PhraseEntry.empty(3).enter(1, "a b c d");
  assert.deepEqual(edit.entry.words, ["", "a", "b"]);
  assert.equal(edit.focus, 2);
});

test("a whole phrase pasted into any field replaces what it held", () => {
  const entry = fields(["one", "par", ""]);
  const edit = entry.paste(1, "two three");
  assert.deepEqual(edit?.entry.words, ["one", "two", "three"]);
  assert.equal(edit?.focus, 2);
  assert.equal(entry.paste(1, " word "), null, "one word pastes as typed");
});

test("the entry reads as one phrase once every field holds a word", () => {
  const partial = fields(["one", "", "three"]);
  assert.equal(partial.complete, false);
  assert.equal(partial.firstEmpty, 1);
  const whole = partial.enter(1, "two").entry;
  assert.equal(whole.complete, true);
  assert.equal(whole.firstEmpty, -1);
  assert.equal(whole.phrase, "one two three");
});

test("Space and Enter go on, and Enter in the last field submits", () => {
  const entry = PhraseEntry.empty(3);
  assert.equal(entry.keyMove(0, " "), 1);
  assert.equal(entry.keyMove(1, "Enter"), 2);
  assert.equal(entry.keyMove(2, " "), 2);
  assert.equal(entry.keyMove(2, "Enter"), null);
  assert.equal(entry.keyMove(1, "a"), null);
});

test("Backspace goes back only from an empty field", () => {
  const entry = fields(["one", "", "three"]);
  assert.equal(entry.keyMove(1, "Backspace"), 0);
  assert.equal(entry.keyMove(2, "Backspace"), null);
  assert.equal(entry.keyMove(0, "Backspace"), null);
});
