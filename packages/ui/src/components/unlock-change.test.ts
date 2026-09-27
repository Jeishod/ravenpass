import assert from "node:assert/strict";
import test from "node:test";
import type { UnlockMethods } from "../vault-api.ts";
import { bindsBiometry, ownerCheck } from "./unlock-change.ts";

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

test("a vault with neither way in on the device asks nothing", () => {
  assert.equal(
    ownerCheck({ ...methods, biometryEnabled: false, pinSet: false }),
    "none",
  );
  assert.equal(
    ownerCheck({ ...methods, biometryAvailable: false, pinSet: false }),
    "none",
  );
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
