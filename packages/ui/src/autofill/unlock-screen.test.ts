import assert from "node:assert/strict";
import test from "node:test";
import type { UnlockOpening, UnlockOutcome } from "./autofill-api.ts";
import { UnlockScreen } from "./unlock-screen.ts";

const opening: UnlockOpening = {
  screen: "unlock",
  methods: {
    biometryAvailable: true,
    biometryEnabled: true,
    pinSet: true,
    pinAttemptsLeft: 5,
    pinMinLength: 6,
    pinMaxLength: 12,
  },
  verify: false,
};

function screenAnswering(...outcomes: UnlockOutcome[]) {
  const attempts: string[] = [];
  const screen = new UnlockScreen(opening, async (pin) => {
    attempts.push(pin);
    return outcomes.shift() ?? { kind: "failed" };
  });
  return { screen, attempts };
}

test("a PIN is tried once it is long enough", async () => {
  const { screen, attempts } = screenAnswering({ kind: "opened" });
  screen.enter("12345");
  await screen.submit();
  assert.deepEqual(attempts, []);
  screen.enter("123456");
  await screen.submit();
  assert.deepEqual(attempts, ["123456"]);
  assert.equal(screen.state.get().busy, true);
});

test("a wrong PIN clears the field and counts the attempts left", async () => {
  const { screen } = screenAnswering({ kind: "wrong-pin", attemptsLeft: 2 });
  screen.enter("000000");
  await screen.submit();
  const view = screen.state.get();
  assert.equal(view.pin, "");
  assert.equal(view.note, "wrong-pin");
  assert.equal(view.methods.pinAttemptsLeft, 2);
  assert.equal(view.busy, false);
});

test("a PIN tried too soon clears the field and keeps the attempts left", async () => {
  const { screen, attempts } = screenAnswering(
    { kind: "too-soon" },
    { kind: "opened" },
  );
  screen.enter("000000");
  await screen.submit();
  const view = screen.state.get();
  assert.equal(view.pin, "");
  assert.equal(view.note, "too-soon");
  assert.equal(view.methods.pinAttemptsLeft, 5);
  assert.equal(view.methods.pinSet, true);
  assert.equal(view.busy, false);
  screen.enter("123456");
  await screen.submit();
  assert.deepEqual(attempts, ["000000", "123456"]);
});

test("a removed PIN leaves the device's own unlock", async () => {
  const { screen, attempts } = screenAnswering(
    { kind: "pin-removed" },
    { kind: "opened" },
  );
  screen.enter("000000");
  await screen.submit();
  assert.equal(screen.state.get().methods.pinSet, false);
  assert.equal(screen.state.get().note, "pin-removed");
  assert.equal(screen.pinReady, false);
  await screen.unlockWithDevice();
  assert.deepEqual(attempts, ["000000", ""]);
});

test("a device unlock the owner turned down is no failure", async () => {
  const { screen } = screenAnswering({ kind: "canceled" });
  await screen.unlockWithDevice();
  assert.equal(screen.state.get().note, null);
  assert.equal(screen.state.get().busy, false);
});

test("the device's own unlock is not offered where the vault does not take it", async () => {
  const attempts: string[] = [];
  const screen = new UnlockScreen(
    { ...opening, methods: { ...opening.methods, biometryEnabled: false } },
    async (pin) => {
      attempts.push(pin);
      return { kind: "opened" };
    },
  );
  await screen.unlockWithDevice();
  assert.equal(screen.biometry, false);
  assert.deepEqual(attempts, []);
});
