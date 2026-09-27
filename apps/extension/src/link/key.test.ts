import assert from "node:assert/strict";
import test from "node:test";
import { ConnectionKey, KeyFormatError } from "./key.ts";
import { bytes, vectors } from "./vectors.test-support.ts";

test("every valid key reads as its port and secret", async () => {
  for (const vector of vectors.keys.valid) {
    const key = await ConnectionKey.parse(vector.key);
    assert.equal(key.port, vector.port, vector.key);
    assert.deepEqual(key.secret, bytes(vector.secret), vector.key);
  }
});

test("a key copied with surrounding whitespace reads the same", async () => {
  const [vector] = vectors.keys.valid;
  assert.ok(vector);
  const key = await ConnectionKey.parse(`  ${vector.key}\n`);
  assert.equal(key.port, vector.port);
});

test("every invalid key is rejected for the reason the desktop app rejects it", async () => {
  for (const vector of vectors.keys.invalid) {
    await assert.rejects(
      ConnectionKey.parse(vector.key),
      (error) =>
        error instanceof KeyFormatError && error.reason === vector.reason,
      `${vector.key} should be rejected for its ${vector.reason}`,
    );
  }
});
