import assert from "node:assert/strict";
import test from "node:test";
import { PendingSignIns } from "./pending-sign-ins.ts";
import { Clock, MemoryArea } from "./test-doubles.test-support.ts";

const tabId = 7;
const origin = "https://github.com";

function pendingSignIns() {
  const area = new MemoryArea();
  const clock = new Clock();
  return { area, clock, pending: new PendingSignIns({ area, now: clock.now }) };
}

test("the next step of the same origin in the tab takes the chosen credential once", async () => {
  const { area, pending } = pendingSignIns();
  await pending.record(tabId, "a1", origin);

  assert.equal(await pending.take(tabId, origin), "a1");
  assert.equal(await pending.take(tabId, origin), null);
  assert.equal(area.items.size, 0);
});

test("a sign-in pending for one site is not taken by another, which leaves it", async () => {
  const { pending } = pendingSignIns();
  await pending.record(tabId, "a1", origin);

  assert.equal(await pending.take(tabId, "https://example.com"), null);
  assert.equal(await pending.take(tabId, origin), "a1");
});

test("the same host over plain http or on another port takes nothing", async () => {
  const { pending } = pendingSignIns();
  await pending.record(tabId, "a1", origin);

  assert.equal(await pending.take(tabId, "http://github.com"), null);
  assert.equal(await pending.take(tabId, "https://github.com:8443"), null);
  assert.equal(await pending.take(tabId, origin), "a1");
});

test("a sign-in pends only in its own tab", async () => {
  const { pending } = pendingSignIns();
  await pending.record(tabId, "a1", origin);

  assert.equal(await pending.take(tabId + 1, origin), null);
  assert.equal(await pending.take(tabId, origin), "a1");
});

test("a sign-in pends for 60 seconds", async () => {
  const { area, clock, pending } = pendingSignIns();
  await pending.record(tabId, "a1", origin);
  clock.time += 60_000 - 1;
  assert.equal(await pending.take(tabId, origin), "a1");

  await pending.record(tabId, "a1", origin);
  clock.time += 60_000;
  assert.equal(await pending.take(tabId, origin), null);
  assert.equal(area.items.size, 0);
});

test("a newer choice in the tab replaces the one before", async () => {
  const { pending } = pendingSignIns();
  await pending.record(tabId, "a1", origin);
  await pending.record(tabId, "b2", origin);

  assert.equal(await pending.take(tabId, origin), "b2");
});

test("closing a tab forgets its pending sign-in and only its own", async () => {
  const { pending } = pendingSignIns();
  await pending.record(tabId, "a1", origin);
  await pending.record(tabId + 1, "b2", origin);

  await pending.forget(tabId);

  assert.equal(await pending.take(tabId, origin), null);
  assert.equal(await pending.take(tabId + 1, origin), "b2");
});

test("a pending sign-in holds the credential's id and the origin, never a value", async () => {
  const { area, clock, pending } = pendingSignIns();
  await pending.record(tabId, "a1", origin);

  assert.deepEqual(Object.fromEntries(area.items), {
    [`sign-in:${tabId}`]: {
      credential: "a1",
      origin,
      expiresAt: clock.time + 60_000,
    },
  });
});
