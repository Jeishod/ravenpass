import assert from "node:assert/strict";
import test from "node:test";
import { SelectionQueue } from "./selection.ts";

test("closes run one after another", async () => {
  const order: string[] = [];
  const pending: (() => void)[] = [];
  let calls = 0;
  const queue = new SelectionQueue(async () => {
    calls += 1;
    const call = calls;
    order.push(`start ${call}`);
    await new Promise<void>((resolve) => pending.push(resolve));
    order.push(`end ${call}`);
  });
  function release(index: number) {
    const resolve = pending[index];
    assert.ok(resolve, `close ${index + 1} has not started`);
    resolve();
  }
  const first = queue.close();
  const second = queue.close();
  await new Promise((resolve) => setTimeout(resolve, 0));
  assert.deepEqual(order, ["start 1"]);
  release(0);
  await first;
  await new Promise((resolve) => setTimeout(resolve, 0));
  release(1);
  await second;
  assert.deepEqual(order, ["start 1", "end 1", "start 2", "end 2"]);
});

test("a failed close is reported and does not block the next", async () => {
  let fail = true;
  const queue = new SelectionQueue(async () => {
    if (fail) throw new Error("closed badly");
  });
  await assert.rejects(queue.close());
  await assert.rejects(queue.settled());
  fail = false;
  await queue.close();
  await queue.settled();
});
