import assert from "node:assert/strict";
import test from "node:test";
import {
  checksumLabel,
  codeLines,
  completeWord,
  completions,
  isSeedChecksum,
  isSeedFormat,
  keptCodes,
  nextUnusedCode,
  phraseColumns,
  splitWords,
  summaryLabel,
  wordAt,
} from "./phrase.ts";

const list = ["abandon", "ability", "able", "about", "above", "absent", "zoo"];

test("a phrase splits on any whitespace", () => {
  assert.deepEqual(splitWords("  abandon\tability\n\nable  "), [
    "abandon",
    "ability",
    "able",
  ]);
  assert.deepEqual(splitWords("   "), []);
  assert.deepEqual(splitWords(""), []);
});

test("codes are read one per line, trimmed, without blank lines", () => {
  assert.deepEqual(codeLines(" 1111-2222 \r\n\n3333 4444\n  "), [
    "1111-2222",
    "3333 4444",
  ]);
});

test("completions follow the list order within the limit", () => {
  assert.deepEqual(completions("ab", list, 3), ["abandon", "ability", "able"]);
  assert.deepEqual(completions("ABO", list), ["about", "above"]);
  assert.deepEqual(completions("", list), []);
  assert.deepEqual(completions("xyz", list), []);
});

test("a finished word offers nothing more, a prefix of longer words still does", () => {
  assert.deepEqual(completions("zoo", list), []);
  assert.deepEqual(completions("able", list), []);
  assert.deepEqual(completions("abl", list), ["able"]);
  assert.deepEqual(completions("abou", ["about", "abouts"]), [
    "about",
    "abouts",
  ]);
});

test("the word at the caret is found and replaced by a completion", () => {
  const text = "abandon abi zoo";
  assert.deepEqual(wordAt(text, 10), { word: "abi", start: 8, end: 11 });
  assert.deepEqual(wordAt(text, 8), { word: "abi", start: 8, end: 11 });
  assert.deepEqual(wordAt("abandon  zoo", 8), {
    word: "",
    start: 8,
    end: 8,
  });
  assert.deepEqual(completeWord(text, 10, "ability"), {
    text: "abandon ability zoo",
    caret: 16,
  });
  assert.deepEqual(completeWord("abandon ab", 10, "able"), {
    text: "abandon able ",
    caret: 13,
  });
});

test("a grid takes four columns only above twelve words", () => {
  assert.equal(phraseColumns(12), 3);
  assert.equal(phraseColumns(15), 4);
  assert.equal(phraseColumns(24), 4);
  assert.equal(phraseColumns(1), 3);
});

test("codes keep the used mark chosen for their value", () => {
  const codes = keptCodes(["a", "b", "c"], new Set(["b", "gone"]));
  assert.deepEqual(codes, [
    { value: "a", used: false },
    { value: "b", used: true },
    { value: "c", used: false },
  ]);
  assert.equal(nextUnusedCode(codes), 0);
  assert.equal(nextUnusedCode([{ value: "a", used: true }, ...codes]), 1);
  assert.equal(nextUnusedCode([{ value: "a", used: true }]), -1);
});

test("a seed summary names its words, its codes left or its key", () => {
  assert.deepEqual(summaryLabel({ format: "phrase", total: 24, used: 0 }), {
    key: "seed.summary.words",
    values: { count: 24 },
    warning: false,
  });
  assert.deepEqual(summaryLabel({ format: "codes", total: 10, used: 3 }), {
    key: "seed.summary.codes",
    values: { left: 7, total: 10 },
    warning: false,
  });
  assert.equal(
    summaryLabel({ format: "codes", total: 10, used: 7 }).warning,
    true,
  );
  assert.equal(
    summaryLabel({ format: "codes", total: 10, used: 6 }).warning,
    false,
  );
  assert.deepEqual(summaryLabel({ format: "key", total: 0, used: 0 }), {
    key: "seed.summary.key",
    warning: false,
  });
});

test("only a failed checksum warns", () => {
  assert.equal(checksumLabel("valid").warning, false);
  assert.equal(checksumLabel("invalid").warning, true);
  assert.equal(checksumLabel("unknown").warning, false);
});

test("formats and checksums are recognised by name", () => {
  assert.ok(
    isSeedFormat("phrase") && isSeedFormat("key") && isSeedFormat("codes"),
  );
  assert.ok(!isSeedFormat("words"));
  assert.ok(isSeedChecksum("") && isSeedChecksum("valid"));
  assert.ok(!isSeedChecksum("broken"));
});
