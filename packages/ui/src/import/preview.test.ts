import assert from "node:assert/strict";
import test from "node:test";
import {
  isEncryptedFormat,
  isImportFormat,
  isImportOrigin,
} from "./preview.ts";

test("password-protected formats are encrypted", () => {
  assert.ok(isEncryptedFormat("encrypted-json"));
  assert.ok(isEncryptedFormat("encrypted-zip"));
});

test("formats read without a password are not encrypted", () => {
  for (const format of ["json", "zip", "csv"] as const) {
    assert.ok(!isEncryptedFormat(format), format);
  }
});

test("a password-protected archive is a known format", () => {
  assert.ok(isImportFormat("encrypted-zip"));
  assert.ok(!isImportFormat("encrypted-csv"));
});

test("an alias is a known origin", () => {
  assert.ok(isImportOrigin("alias"));
  assert.ok(!isImportOrigin("tag"));
});
