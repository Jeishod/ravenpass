import assert from "node:assert/strict";
import test from "node:test";
import { appKey } from "./apps.ts";

const signer = `ab01${"00".repeat(28)}ef7c`;

test("each certificate of one app is its own link", () => {
  const other = `cd${signer.slice(2)}`;
  assert.notEqual(
    appKey({ package: "com.example.mail", signer }),
    appKey({ package: "com.example.mail", signer: other }),
  );
});
