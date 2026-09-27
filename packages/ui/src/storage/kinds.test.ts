import assert from "node:assert/strict";
import test from "node:test";
import { storageKindDetail, storageWarnings } from "./kinds.ts";

test("a local file is described by who picks where it lives", () => {
  assert.equal(
    storageKindDetail("local-file", ["local-file"]),
    "storage.type.local-file.detail",
  );
  assert.equal(
    storageKindDetail("local-file", ["document"]),
    "storage.type.local-file.private",
  );
  assert.equal(
    storageKindDetail("local-file", []),
    "storage.type.local-file.private",
  );
});

test("a document is always a file the owner picks", () => {
  assert.equal(
    storageKindDetail("document", ["document"]),
    "storage.type.document.detail",
  );
  assert.equal(
    storageKindDetail("document", []),
    "storage.type.document.detail",
  );
});

test("an unrestricted location warns only for a local file", () => {
  assert.deepEqual(storageWarnings({ kind: "local-file", restricted: false }), [
    "unrestricted",
  ]);
  assert.deepEqual(
    storageWarnings({ kind: "local-file", restricted: true }),
    [],
  );
  assert.deepEqual(storageWarnings({ kind: "", restricted: false }), []);
});

test("a document warns that concurrent writes are not merged", () => {
  assert.deepEqual(storageWarnings({ kind: "document", restricted: false }), [
    "concurrent",
  ]);
  assert.deepEqual(storageWarnings({ kind: "document", restricted: true }), [
    "concurrent",
  ]);
});
