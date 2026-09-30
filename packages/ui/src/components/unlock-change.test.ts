import assert from "node:assert/strict";
import test from "node:test";
import type { UnlockMethods } from "../vault-api.ts";
import { bindsBiometry, heldWays, ownerCheck } from "./unlock-change.ts";

const methods: UnlockMethods = {
  biometryAvailable: true,
  biometryEnabled: true,
  pinSet: true,
  pinAttemptsLeft: 10,
  pinMinLength: 6,
  pinMaxLength: 12,
};

test("device authentication verifies the owner wherever the vault opens with it", () => {
  assert.equal(ownerCheck(methods), "device");
  assert.equal(ownerCheck({ ...methods, pinSet: false }), "device");
});

test("the current PIN verifies the owner where device authentication is off or unavailable", () => {
  assert.equal(ownerCheck({ ...methods, biometryEnabled: false }), "pin");
  assert.equal(ownerCheck({ ...methods, biometryAvailable: false }), "pin");
});

test("a vault with no way in the device can use asks for its recovery key", () => {
  assert.equal(
    ownerCheck({ ...methods, biometryEnabled: false, pinSet: false }),
    "recovery-key",
  );
  assert.equal(
    ownerCheck({ ...methods, biometryAvailable: false, pinSet: false }),
    "recovery-key",
  );
});

test("a way in stays on only while no other works on the device", () => {
  assert.deepEqual(heldWays(methods), { biometry: false, pin: false });
  assert.deepEqual(heldWays({ ...methods, pinSet: false }), {
    biometry: true,
    pin: false,
  });
  assert.deepEqual(heldWays({ ...methods, biometryEnabled: false }), {
    biometry: false,
    pin: true,
  });
  assert.deepEqual(heldWays(null), { biometry: false, pin: false });
});

test("device authentication the device cannot give holds the PIN on and may itself be turned off", () => {
  assert.deepEqual(heldWays({ ...methods, biometryAvailable: false }), {
    biometry: false,
    pin: true,
  });
});

test("choosing device authentication creates its key", () => {
  assert.equal(bindsBiometry({ biometry: true, pin: "" }, null), true);
  assert.equal(bindsBiometry({ biometry: true, pin: "135790" }, methods), true);
});

test("keeping the ways in binds device authentication again only where it was on", () => {
  assert.equal(bindsBiometry({ biometry: false, pin: "" }, methods), true);
  assert.equal(
    bindsBiometry(
      { biometry: false, pin: "" },
      { ...methods, biometryEnabled: false },
    ),
    false,
  );
  assert.equal(bindsBiometry({ biometry: false, pin: "" }, null), false);
});

test("choosing a PIN alone creates no device authentication key", () => {
  assert.equal(
    bindsBiometry({ biometry: false, pin: "135790" }, methods),
    false,
  );
});
