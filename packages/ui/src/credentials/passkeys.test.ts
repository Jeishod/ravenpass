import assert from "node:assert/strict";
import test from "node:test";
import { passkeyAccount, passkeyCreatedOn } from "./passkeys.ts";

test("a passkey signs in as its account name, else its display name", () => {
  assert.equal(
    passkeyAccount({ account: "octocat", displayName: "Mona Lisa" }),
    "octocat",
  );
  assert.equal(
    passkeyAccount({ account: "", displayName: "Mona Lisa" }),
    "Mona Lisa",
  );
  assert.equal(passkeyAccount({ account: "", displayName: "" }), "");
});

test("a passkey's creation reads as the local day it fell on", () => {
  const createdAt = new Date(2026, 8, 23, 23, 59).getTime();
  assert.equal(passkeyCreatedOn({ createdAt }, "en"), "Sep 23, 2026");
});
