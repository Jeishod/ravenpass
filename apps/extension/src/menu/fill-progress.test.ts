import assert from "node:assert/strict";
import test from "node:test";
import { FillProgress } from "./fill-progress.ts";

function deferred<Value>() {
  let resolve: (value: Value) => void = () => {};
  let reject: (reason: unknown) => void = () => {};
  const promise = new Promise<Value>((onResolve, onReject) => {
    resolve = onResolve;
    reject = onReject;
  });
  return { promise, resolve, reject };
}

test("progress shows while a fill waits and clears once it is answered", async () => {
  const progress = new FillProgress();
  const answer = deferred<string>();

  const tracked = progress.track(answer.promise);
  progress.report("confirm-on-device");
  assert.equal(progress.view(), "confirm-on-device");

  answer.resolve("filled");
  assert.equal(await tracked, "filled");
  assert.equal(progress.view(), null);
});

test("progress clears when a fill is refused", async () => {
  const progress = new FillProgress();
  const answer = deferred<string>();

  const tracked = progress.track(answer.promise);
  progress.report("confirm-in-ravenpass");
  answer.reject(new Error("declined"));

  await assert.rejects(tracked);
  assert.equal(progress.view(), null);
});

test("progress with no fill waiting, or arriving after the answer, is dropped", async () => {
  const progress = new FillProgress();
  let heard = 0;
  progress.subscribe(() => {
    heard += 1;
  });

  progress.report("confirm-on-device");
  await progress.track(Promise.resolve("filled"));
  progress.report("confirm-on-device");

  assert.equal(progress.view(), null);
  assert.equal(heard, 0);
});

test("progress stays until the last of overlapping requests is answered", async () => {
  const progress = new FillProgress();
  const first = deferred<string>();
  const second = deferred<string>();

  const tracked = [
    progress.track(first.promise),
    progress.track(second.promise),
  ];
  progress.report("confirm-on-device");
  first.resolve("first");
  await tracked[0];
  assert.equal(progress.view(), "confirm-on-device");

  second.resolve("second");
  await tracked[1];
  assert.equal(progress.view(), null);
});
