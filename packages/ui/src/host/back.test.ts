import assert from "node:assert/strict";
import test from "node:test";
import { type BackHistory, BackStack } from "./back.ts";

/** A history whose back pops at once, as a browser does after the current task. */
class FakeHistory implements BackHistory {
  entries = 0;
  #listener: () => void = () => {};

  push() {
    this.entries++;
  }

  back() {
    this.entries--;
    queueMicrotask(() => this.#listener());
  }

  onPop(listener: () => void) {
    this.#listener = listener;
  }

  /** The system's back gesture. */
  gesture() {
    this.entries--;
    this.#listener();
  }
}

const settle = () => new Promise((resolve) => setTimeout(resolve));

test("the gesture closes the innermost screen", async () => {
  const history = new FakeHistory();
  const stack = new BackStack(history);
  const closed: string[] = [];
  stack.open(() => closed.push("settings"));
  stack.open(() => closed.push("section"));
  assert.equal(history.entries, 1);
  history.gesture();
  assert.deepEqual(closed, ["section"]);
  assert.equal(history.entries, 1, "the outer screen keeps an entry");
  await settle();
});

test("closing from the interface drops the entry once no screen is left", async () => {
  const history = new FakeHistory();
  const stack = new BackStack(history);
  const gone = stack.open(() => assert.fail("the gesture was not used"));
  gone();
  await settle();
  assert.equal(history.entries, 0);
});

test("a held screen stays open and keeps the gesture from the screen under it", async () => {
  const history = new FakeHistory();
  const stack = new BackStack(history);
  const closed: string[] = [];
  let held = true;
  stack.open(() => closed.push("settings"));
  stack.open(
    () => closed.push("change"),
    () => held,
  );
  history.gesture();
  assert.deepEqual(closed, []);
  assert.equal(history.entries, 1, "the held screen keeps an entry");
  held = false;
  history.gesture();
  assert.deepEqual(closed, ["change"]);
  await settle();
});

test("a screen that replaces another keeps the entry", async () => {
  const history = new FakeHistory();
  const stack = new BackStack(history);
  const closed: string[] = [];
  const list = stack.open(() => closed.push("list"));
  list();
  stack.open(() => closed.push("section"));
  await settle();
  assert.equal(history.entries, 1);
  history.gesture();
  assert.deepEqual(closed, ["section"]);
});
