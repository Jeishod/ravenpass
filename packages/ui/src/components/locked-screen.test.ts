import assert from "node:assert/strict";
import test from "node:test";
import { lockedActions, lockedScreen } from "./locked-screen.ts";

const none = { pinSet: false, biometryEnabled: false, biometryAvailable: true };
const pin = { ...none, pinSet: true };
const biometry = { ...none, biometryEnabled: true };
const both = { ...pin, biometryEnabled: true };

test("a vault file that went asks for the file before the recovery key", () => {
  assert.equal(lockedScreen({ missing: true }, none), "missing");
  assert.equal(lockedScreen({ missing: true }, null), "missing");
  assert.equal(lockedScreen({ missing: true }, pin), "missing");
});

test("a device with no way in asks for the recovery key", () => {
  assert.equal(lockedScreen({ missing: false }, none), "restore");
  assert.equal(lockedScreen(null, none), "restore");
  assert.equal(
    lockedScreen(null, { ...biometry, biometryAvailable: false }),
    "restore",
  );
});

test("a device with a way in asks for it", () => {
  assert.equal(lockedScreen({ missing: false }, pin), "unlock");
  assert.equal(lockedScreen({ missing: false }, biometry), "unlock");
  assert.equal(lockedScreen({ missing: false }, null), "unlock");
});

test("a missing vault file offers only its reopening, whatever the device's ways in", () => {
  for (const methods of [none, pin, biometry, both, null]) {
    assert.deepEqual(lockedActions("missing", methods), {
      pin: false,
      deviceUnlock: false,
      openFile: true,
      recovery: null,
    });
  }
});

test("a device with no way in offers the recovery key first", () => {
  assert.deepEqual(lockedActions("restore", none), {
    pin: false,
    deviceUnlock: false,
    openFile: false,
    recovery: "primary",
  });
});

test("a device with a way in offers it before the recovery key", () => {
  assert.deepEqual(lockedActions("unlock", pin), {
    pin: true,
    deviceUnlock: false,
    openFile: false,
    recovery: "secondary",
  });
  assert.deepEqual(lockedActions("unlock", biometry), {
    pin: false,
    deviceUnlock: true,
    openFile: false,
    recovery: "secondary",
  });
  assert.deepEqual(lockedActions("unlock", both), {
    pin: true,
    deviceUnlock: true,
    openFile: false,
    recovery: "secondary",
  });
  assert.deepEqual(
    lockedActions("unlock", { ...biometry, biometryAvailable: false }),
    { pin: false, deviceUnlock: false, openFile: false, recovery: "secondary" },
  );
  assert.deepEqual(lockedActions("unlock", null), {
    pin: false,
    deviceUnlock: false,
    openFile: false,
    recovery: "secondary",
  });
});
