import assert from "node:assert/strict";
import test from "node:test";
import {
  closing,
  groupedCode,
  maskedCode,
  periodEnd,
  periodLeft,
  timeLeft,
} from "./one-time-code.ts";

test("a code is split in two, the longer half first", () => {
  assert.equal(groupedCode("123456"), "123 456");
  assert.equal(groupedCode("12345678"), "1234 5678");
  assert.equal(groupedCode("1234567"), "1234 567");
});

test("a masked code has a dot for each digit, grouped as the digits are", () => {
  assert.equal(groupedCode(maskedCode(6)), "••• •••");
  assert.equal(groupedCode(maskedCode(8)), "•••• ••••");
  assert.equal(groupedCode(maskedCode(7)), "•••• •••");
});

test("a period ends on a multiple of its length since the Unix epoch", () => {
  assert.equal(periodEnd(30, 1_699_999_980_000), 1_700_000_010_000);
  assert.equal(periodEnd(30, 1_699_999_980_001), 1_700_000_010_000);
  assert.equal(periodEnd(30, 1_700_000_009_999), 1_700_000_010_000);
  assert.equal(periodEnd(30, 1_700_000_010_000), 1_700_000_040_000);
  assert.equal(periodEnd(60, 1_700_000_010_000), 1_700_000_040_000);
});

test("the time a period has left comes from the clock alone", () => {
  assert.deepEqual(periodLeft(30, 1_699_999_980_000), {
    seconds: 30,
    share: 1,
  });
  assert.deepEqual(periodLeft(30, 1_699_999_995_000), {
    seconds: 15,
    share: 0.5,
  });
  assert.deepEqual(periodLeft(60, 1_700_000_039_500), {
    seconds: 1,
    share: 1 / 60,
  });
});

test("a code closes in the last five seconds its timer shows", () => {
  const end = 1_700_000_010_000;
  assert.equal(closing(periodLeft(30, end - 5_001)), false);
  assert.equal(closing(periodLeft(30, end - 5_000)), true);
  assert.equal(closing(periodLeft(30, end - 1)), true);
  assert.equal(closing(periodLeft(30, end)), false);
});

test("a code's time left counts whole seconds, rounded up", () => {
  const code = { period: 30, expiresAt: 1_750_000_030_000 };
  assert.deepEqual(timeLeft(code, 1_750_000_000_000), {
    seconds: 30,
    share: 1,
  });
  assert.deepEqual(timeLeft(code, 1_750_000_015_001), {
    seconds: 15,
    share: 0.5,
  });
  assert.deepEqual(timeLeft(code, 1_750_000_029_999), {
    seconds: 1,
    share: 1 / 30,
  });
});

test("an expired code has nothing left", () => {
  assert.deepEqual(
    timeLeft({ period: 30, expiresAt: 1_750_000_000_000 }, 1_750_000_001_000),
    { seconds: 0, share: 0 },
  );
});

test("the share never passes a full ring, and a code without a period has none", () => {
  assert.equal(
    timeLeft({ period: 30, expiresAt: 1_750_000_060_000 }, 1_750_000_000_000)
      .share,
    1,
  );
  assert.equal(
    timeLeft({ period: 0, expiresAt: 1_750_000_030_000 }, 1_750_000_000_000)
      .share,
    0,
  );
});
