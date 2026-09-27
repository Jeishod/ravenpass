import assert from "node:assert/strict";
import test from "node:test";
import { hintSeen, markHintSeen } from "./hints.ts";

function memoryStorage(): Storage {
  const values = new Map<string, string>();
  return {
    get length() {
      return values.size;
    },
    clear: () => values.clear(),
    getItem: (key) => values.get(key) ?? null,
    key: (index) => Array.from(values.keys())[index] ?? null,
    removeItem: (key) => values.delete(key),
    setItem: (key, value) => values.set(key, value),
  };
}

test("a hint is shown until it is marked seen", () => {
  const store = memoryStorage();
  assert.equal(hintSeen("card-bank-site", store), false);
  markHintSeen("card-bank-site", store);
  assert.equal(hintSeen("card-bank-site", store), true);
});

test("a storage that refuses access shows the hint and takes no harm", () => {
  const refusing = {
    ...memoryStorage(),
    getItem: () => {
      throw new Error("denied");
    },
    setItem: () => {
      throw new Error("denied");
    },
  } as Storage;
  assert.equal(hintSeen("card-bank-site", refusing), false);
  assert.doesNotThrow(() => markHintSeen("card-bank-site", refusing));
  assert.equal(hintSeen("card-bank-site", null), false);
});
