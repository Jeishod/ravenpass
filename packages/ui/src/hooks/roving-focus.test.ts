import assert from "node:assert/strict";
import test from "node:test";
import { listTarget } from "./roving-focus.ts";

test("arrows step through the list and stop at its ends", () => {
  assert.equal(listTarget("ArrowDown", -1, 3, false), 0);
  assert.equal(listTarget("ArrowDown", 0, 3, false), 1);
  assert.equal(listTarget("ArrowDown", 2, 3, false), 2);
  assert.equal(listTarget("ArrowUp", 2, 3, false), 1);
  assert.equal(listTarget("ArrowUp", 0, 3, false), "before");
  assert.equal(listTarget("ArrowUp", -1, 3, false), "before");
});

test("a looping list wraps at both ends", () => {
  assert.equal(listTarget("ArrowDown", 2, 3, true), 0);
  assert.equal(listTarget("ArrowUp", 0, 3, true), 2);
});

test("Home and End reach the first and last items", () => {
  assert.equal(listTarget("Home", 2, 3, false), 0);
  assert.equal(listTarget("End", 0, 3, false), 2);
});

test("other keys and an empty list move nothing", () => {
  assert.equal(listTarget("Enter", 0, 3, false), null);
  assert.equal(listTarget("ArrowDown", -1, 0, false), null);
});
