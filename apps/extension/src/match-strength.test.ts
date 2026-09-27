import assert from "node:assert/strict";
import test from "node:test";
import { matchStrength } from "./match-strength.ts";

test("an exact match on an https page is strong", () => {
  assert.equal(
    matchStrength(true, "github.com", "https://github.com"),
    "strong",
  );
  assert.equal(
    matchStrength(true, "github.com", "https://github.com:8443"),
    "strong",
  );
});

test("an exact match on a plain http page is weak", () => {
  assert.equal(
    matchStrength(true, "example.com", "http://example.com"),
    "insecure-page",
  );
  assert.equal(
    matchStrength(true, "127.0.0.1", "http://127.0.0.1:8080"),
    "insecure-page",
  );
});

test("an exact match on http localhost is strong", () => {
  assert.equal(
    matchStrength(true, "localhost", "http://localhost:3000"),
    "strong",
  );
  assert.equal(
    matchStrength(true, "app.localhost", "http://app.localhost"),
    "strong",
  );
  assert.equal(
    matchStrength(
      true,
      "localhost.example.com",
      "http://localhost.example.com",
    ),
    "insecure-page",
  );
});

test("a domain-only match on an https page of the saved site's registrable domain is the same site", () => {
  assert.equal(
    matchStrength(false, "soundcloud.com", "https://secure.soundcloud.com"),
    "same-site",
  );
  assert.equal(
    matchStrength(false, "example.co.uk", "https://files.example.co.uk"),
    "same-site",
  );
});

test("any other domain-only match is weak", () => {
  assert.equal(
    matchStrength(false, "alice.github.io", "https://bob.github.io"),
    "other-site",
  );
  assert.equal(
    matchStrength(false, "localhost", "http://localhost:3000"),
    "other-site",
  );
  assert.equal(
    matchStrength(false, "example.com", "http://files.example.com"),
    "insecure-page",
  );
});
