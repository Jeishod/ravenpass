import assert from "node:assert/strict";
import test from "node:test";
import type { CredentialInput } from "../vault-api.ts";
import { accountOf, emptyCredential, readyToSave } from "./credential.ts";

test("websites left blank are dropped before saving", () => {
  const input: CredentialInput = {
    ...emptyCredential,
    label: "GitHub",
    websites: ["", "github.com", "  ", "gist.github.com"],
    password: "secret",
  };
  assert.deepEqual(readyToSave(input), {
    ...input,
    websites: ["github.com", "gist.github.com"],
  });
});

test("a website holding anything is kept as typed, in its place", () => {
  const input: CredentialInput = {
    ...emptyCredential,
    label: "Admin",
    websites: [" admin.example.com ", "example.com"],
  };
  assert.deepEqual(readyToSave(input).websites, input.websites);
});

test("the name is saved without surrounding spaces", () => {
  assert.equal(
    readyToSave({ ...emptyCredential, label: "  GitHub " }).label,
    "GitHub",
  );
});

test("a new credential starts with no websites", () => {
  assert.deepEqual(readyToSave(emptyCredential).websites, []);
});

test("a credential signs in with its login, else its email", () => {
  assert.equal(
    accountOf({ login: "octocat", email: "me@example.com" }),
    "octocat",
  );
  assert.equal(
    accountOf({ login: "", email: "me@example.com" }),
    "me@example.com",
  );
  assert.equal(accountOf({ login: "", email: "" }), "");
});
