import assert from "node:assert/strict";
import test from "node:test";
import { failureCode, refusedBeforeWriting } from "./failures.ts";

test("a failure carries the code the desktop service reported", () => {
  assert.equal(
    failureCode(new Error("ravenpass:invalid-code")),
    "invalid-code",
  );
  assert.equal(failureCode("  ravenpass:vault-locked  "), "vault-locked");
  assert.equal(failureCode(new Error("something else")), "");
  assert.equal(failureCode(null), "");
  assert.equal(failureCode({ message: "ravenpass:invalid-code" }), "");
});

test("a failure the Android bridge wraps in its own words keeps its code", () => {
  assert.equal(
    failureCode(
      new Error(
        "Binding call failed: Bound method returned an error: ravenpass:malformed-vault",
      ),
    ),
    "malformed-vault",
  );
  assert.equal(failureCode(new Error("see ravenpass:vault-locked later")), "");
  assert.equal(failureCode(new Error("notravenpass:vault-locked")), "");
});

test("a refused input leaves the vault file untouched", () => {
  assert.ok(refusedBeforeWriting(new Error("ravenpass:invalid-code")));
  assert.ok(refusedBeforeWriting(new Error("ravenpass:invalid-item")));
  assert.ok(refusedBeforeWriting(new Error("ravenpass:group-unknown")));
  assert.ok(refusedBeforeWriting(new Error("ravenpass:code-setup-expired")));
});

test("an import with no staged file or nothing to add leaves the vault file untouched", () => {
  assert.ok(refusedBeforeWriting(new Error("ravenpass:import-not-active")));
  assert.ok(refusedBeforeWriting(new Error("ravenpass:import-empty")));
  assert.ok(refusedBeforeWriting(new Error("ravenpass:import-limit")));
});

test("anything else counts as a write that may have happened", () => {
  assert.ok(!refusedBeforeWriting(new Error("ravenpass:save-unconfirmed")));
  assert.ok(!refusedBeforeWriting(new Error("ravenpass:storage-unavailable")));
  assert.ok(!refusedBeforeWriting(new Error("ravenpass:general")));
  assert.ok(!refusedBeforeWriting(new Error("network died")));
});
