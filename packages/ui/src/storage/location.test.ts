import assert from "node:assert/strict";
import test from "node:test";
import { vaultLocation, vaultName } from "./location.ts";

test("a vault is named by its file without the extension", () => {
  assert.equal(vaultName("vault.rpv"), "vault");
  assert.equal(vaultName("Work notes.rpv"), "Work notes");
  assert.equal(vaultName("archive.2024.rpv"), "archive.2024");
  assert.equal(vaultName("vault"), "vault");
  assert.equal(vaultName(".hidden"), ".hidden");
  assert.equal(vaultName(""), "");
});

test("a vault file is located by its place and its name", () => {
  assert.equal(
    vaultLocation({ name: "vault.rpv", place: "Google Drive › My Drive" }),
    "Google Drive › My Drive › vault.rpv",
  );
  assert.equal(
    vaultLocation({ name: "vault.rpv", place: "On this device" }),
    "On this device › vault.rpv",
  );
});

test("a file without a place is located by its name alone", () => {
  assert.equal(vaultLocation({ name: "vault.rpv", place: "" }), "vault.rpv");
  assert.equal(vaultLocation({ name: "", place: "" }), "");
});
