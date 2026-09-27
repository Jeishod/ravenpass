import assert from "node:assert/strict";
import { mock, test } from "node:test";
import type { OneTimeCode } from "../vault-api.ts";
import { CodeRenewal } from "./code-renewal.ts";

function codeExpiringAt(expiresAt: number, code = "123456"): OneTimeCode {
  return { code, digits: 6, period: 30, expiresAt };
}

async function flush() {
  for (let turn = 0; turn < 5; turn += 1) await Promise.resolve();
}

test("asks again when the current code expires", async (context) => {
  context.mock.timers.enable({ apis: ["setTimeout", "Date"], now: 0 });
  const answers = [
    codeExpiringAt(5000, "111111"),
    codeExpiringAt(35000, "222222"),
  ];
  const generate = mock.fn(async () => {
    const answer = answers.shift();
    assert.ok(answer);
    return answer;
  });
  const received: (string | null)[] = [];
  const renewal = new CodeRenewal(generate, (code) => {
    received.push(code?.code ?? null);
  });

  renewal.start();
  await flush();
  assert.deepEqual(received, ["111111"]);

  context.mock.timers.tick(4999);
  await flush();
  assert.equal(generate.mock.callCount(), 1);

  context.mock.timers.tick(1);
  await flush();
  assert.deepEqual(received, ["111111", "222222"]);
  renewal.stop();
});

test("waits at least a second for a code that has already expired", async (context) => {
  context.mock.timers.enable({ apis: ["setTimeout", "Date"], now: 10000 });
  const generate = mock.fn(async () => codeExpiringAt(0));
  const renewal = new CodeRenewal(generate, () => {});

  renewal.start();
  await flush();
  context.mock.timers.tick(999);
  await flush();
  assert.equal(generate.mock.callCount(), 1);

  context.mock.timers.tick(1);
  await flush();
  assert.equal(generate.mock.callCount(), 2);
  renewal.stop();
});

test("reports a failed request as null and stops asking", async (context) => {
  context.mock.timers.enable({ apis: ["setTimeout", "Date"], now: 0 });
  const generate = mock.fn(async (): Promise<OneTimeCode> => {
    throw new Error("no code");
  });
  const received: (OneTimeCode | null)[] = [];
  const renewal = new CodeRenewal(generate, (code) => received.push(code));

  renewal.start();
  await flush();
  context.mock.timers.tick(60000);
  await flush();
  assert.deepEqual(received, [null]);
  assert.equal(generate.mock.callCount(), 1);
});

test("delivers nothing and asks no more once stopped", async (context) => {
  context.mock.timers.enable({ apis: ["setTimeout", "Date"], now: 0 });
  const generate = mock.fn(async () => codeExpiringAt(5000));
  const received: (OneTimeCode | null)[] = [];
  const renewal = new CodeRenewal(generate, (code) => received.push(code));

  renewal.start();
  renewal.stop();
  await flush();
  context.mock.timers.tick(60000);
  await flush();
  assert.deepEqual(received, []);
  assert.equal(generate.mock.callCount(), 1);
});
