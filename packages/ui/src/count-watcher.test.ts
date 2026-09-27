import assert from "node:assert/strict";
import test from "node:test";
import { CountWatcher } from "./count-watcher.ts";

interface Wait {
  seen: number;
  signal: AbortSignal;
  answer: (count: number) => void;
  fail: (cause: unknown) => void;
}

class ScriptedCount {
  readonly waits: Wait[] = [];

  readonly awaitCount = (seen: number, signal: AbortSignal): Promise<number> =>
    new Promise((answer, fail) => {
      signal.addEventListener("abort", () =>
        fail(new Error("The call was canceled.")),
      );
      this.waits.push({ seen, signal, answer, fail });
    });

  wait(index: number): Wait {
    const wait = this.waits[index];
    assert.ok(wait, `no wait ${index} started`);
    return wait;
  }
}

function settle() {
  return new Promise((resolve) => setTimeout(resolve, 0));
}

test("each change is reported and the next wait starts from its count", async () => {
  const host = new ScriptedCount();
  let changes = 0;
  const stop = new CountWatcher(host.awaitCount).watch(() => {
    changes += 1;
  });
  await settle();
  assert.deepEqual(
    host.waits.map((wait) => wait.seen),
    [0],
  );
  assert.equal(changes, 0);

  host.wait(0).answer(1);
  await settle();
  assert.equal(changes, 1);
  host.wait(1).answer(3);
  await settle();
  assert.equal(changes, 2);
  assert.deepEqual(
    host.waits.map((wait) => wait.seen),
    [0, 1, 3],
  );
  stop();
});

test("stopping cancels the wait and reports nothing more", async () => {
  const host = new ScriptedCount();
  let changes = 0;
  const stop = new CountWatcher(host.awaitCount).watch(() => {
    changes += 1;
  });
  await settle();
  stop();
  assert.equal(host.wait(0).signal.aborted, true);
  host.wait(0).answer(1);
  await settle();
  assert.equal(changes, 0);
  assert.equal(host.waits.length, 1);
});

test("a failed wait ends the watch", async () => {
  const host = new ScriptedCount();
  let changes = 0;
  new CountWatcher(host.awaitCount).watch(() => {
    changes += 1;
  });
  await settle();
  host.wait(0).fail(new Error("Ravenpass closed."));
  await settle();
  assert.equal(changes, 0);
  assert.equal(host.waits.length, 1);
});
