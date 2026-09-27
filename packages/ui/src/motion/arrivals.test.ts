import assert from "node:assert/strict";
import test from "node:test";
import { RowArrivals } from "./arrivals.ts";

/** A scheduler whose frames the test ends by hand. */
function manualFrames() {
  const tasks: (() => void)[] = [];
  return {
    schedule: (task: () => void) => {
      tasks.push(task);
    },
    end: () => {
      for (const task of tasks.splice(0)) task();
    },
    pending: () => tasks.length,
  };
}

test("rows arriving in one frame follow one another", () => {
  const frames = manualFrames();
  const arrivals = new RowArrivals(frames.schedule);
  assert.deepEqual(
    ["a", "b", "c"].map((id) => arrivals.arrive(id)),
    [0, 1, 2],
  );
  assert.equal(frames.pending(), 1);
});

test("a row arriving in a later frame arrives first", () => {
  const frames = manualFrames();
  const arrivals = new RowArrivals(frames.schedule);
  arrivals.arrive("a");
  arrivals.arrive("b");
  frames.end();
  assert.equal(arrivals.arrive("c"), 0);
});

test("a row is seen once it has arrived", () => {
  const arrivals = new RowArrivals(manualFrames().schedule);
  assert.equal(arrivals.seenBefore("a"), false);
  arrivals.arrive("a");
  assert.equal(arrivals.seenBefore("a"), true);
  assert.equal(arrivals.seenBefore("b"), false);
});
