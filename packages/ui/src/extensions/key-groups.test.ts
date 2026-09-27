import assert from "node:assert/strict";
import test from "node:test";
import { keyGroups } from "./key-groups.ts";

test("a key reads in groups of four that join back into the key", () => {
  const key = "mlwT9ZqA-x_0abcdEFGHij";
  const groups = keyGroups(key);
  assert.deepEqual(
    groups.map((group) => group.text),
    ["mlwT", "9ZqA", "-x_0", "abcd", "EFGH", "ij"],
  );
  assert.deepEqual(
    groups.map((group) => group.start),
    [0, 4, 8, 12, 16, 20],
  );
  assert.equal(groups.map((group) => group.text).join(""), key);
});

test("an empty key has no groups", () => {
  assert.deepEqual(keyGroups(""), []);
});
