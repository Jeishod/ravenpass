import assert from "node:assert/strict";
import test from "node:test";
import { Countdown } from "./countdown.ts";

const start = Date.UTC(2026, 8, 22, 12, 0, 0);
const fiveMinutes = new Countdown(start + 5 * 60 * 1000);

test("a key reads its time left in minutes and two-digit seconds", () => {
  assert.equal(fiveMinutes.display(start), "5:00");
  assert.equal(fiveMinutes.display(start + 1000), "4:59");
  assert.equal(fiveMinutes.display(start + 239_000), "1:01");
  assert.equal(fiveMinutes.display(start + 291_000), "0:09");
});

test("a second counts until it has passed", () => {
  assert.equal(fiveMinutes.secondsLeft(start + 1), 300);
  assert.equal(fiveMinutes.secondsLeft(start + 299_500), 1);
  assert.equal(fiveMinutes.expired(start + 299_500), false);
});

test("a key past its moment has no time left", () => {
  assert.equal(fiveMinutes.secondsLeft(start + 300_000), 0);
  assert.equal(fiveMinutes.secondsLeft(start + 400_000), 0);
  assert.equal(fiveMinutes.display(start + 400_000), "0:00");
  assert.ok(fiveMinutes.expired(start + 300_000));
});
