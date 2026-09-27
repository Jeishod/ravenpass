import assert from "node:assert/strict";
import test from "node:test";
import { isSignInStyle, signInStyleOf } from "./sign-in-style.ts";

test("a known style reads as itself", () => {
  assert.equal(signInStyleOf("card"), "card");
  assert.equal(signInStyleOf("field"), "field");
  assert.ok(isSignInStyle("field"));
});

test("a style this build does not know reads as the card", () => {
  for (const value of ["popup", "Card", "", undefined, null, 1, {}]) {
    assert.equal(signInStyleOf(value), "card");
    assert.ok(!isSignInStyle(value));
  }
});
