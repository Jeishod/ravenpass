import assert from "node:assert/strict";
import test from "node:test";
import { systemAutofillRoles } from "./system-autofill.ts";

test("a device that takes passkey providers reports both roles", () => {
  assert.deepEqual(
    systemAutofillRoles({
      autofill: true,
      passkeys: false,
      passkeyProviders: true,
    }),
    [
      { role: "autofill", on: true },
      { role: "passkeys", on: false },
    ],
  );
});

test("a device without passkey providers reports only the autofill service", () => {
  assert.deepEqual(
    systemAutofillRoles({
      autofill: false,
      passkeys: false,
      passkeyProviders: false,
    }),
    [{ role: "autofill", on: false }],
  );
});
