import assert from "node:assert/strict";
import { test } from "node:test";
import {
  isCardShow,
  isFileMenuOpen,
  isFormMessage,
  isFrameMessage,
  isMenuMoved,
  isOfferShow,
  isPasskeyAnswer,
  isPlaced,
  isPresent,
  isShown,
  isSubmission,
  isVaultStateMessage,
} from "./messages.ts";

test("a message is told apart by its kind alone, whatever else it carries", () => {
  assert.equal(isCardShow({ kind: "card-show" }), true);
  assert.equal(isCardShow({ kind: "card-show", token: "t", extra: 1 }), true);
  assert.equal(isFileMenuOpen({ kind: "file-menu-open" }), true);
  assert.equal(isMenuMoved({ kind: "menu-moved" }), true);
  assert.equal(isOfferShow({ kind: "offer-show" }), true);
  assert.equal(isPasskeyAnswer({ kind: "passkey-answer" }), true);
  for (const message of [
    null,
    undefined,
    "card-show",
    {},
    { kind: "Card-show" },
  ]) {
    assert.equal(isCardShow(message), false);
  }
  assert.equal(isOfferShow({ kind: "card-show" }), false);
});

test("a form message is one of the kinds a sign-in card sends its frame", () => {
  for (const kind of ["form-sign-in", "form-code", "form-present"]) {
    assert.equal(isFormMessage({ kind }), true);
  }
  assert.equal(isFormMessage({ kind: "fill" }), false);
  assert.equal(isFormMessage({}), false);
});

test("a frame message names a frame kind and the menu's own token", () => {
  assert.equal(isFrameMessage({ kind: "fill", token: "t1" }, "t1"), true);
  assert.equal(isFrameMessage({ kind: "card-hide", token: "t1" }, "t1"), true);
  assert.equal(isFrameMessage({ kind: "fill", token: "t2" }, "t1"), false);
  assert.equal(isFrameMessage({ kind: "fill" }, "t1"), false);
  assert.equal(isFrameMessage({ kind: "form-code", token: "t1" }, "t1"), false);
});

test("a vault state message names a known state", () => {
  for (const state of ["unlocked", "locked", "not-open", "unlinked"]) {
    assert.equal(isVaultStateMessage({ kind: "vault-state", state }), true);
  }
  assert.equal(
    isVaultStateMessage({ kind: "vault-state", state: "open" }),
    false,
  );
  assert.equal(isVaultStateMessage({ kind: "vault-state" }), false);
});

test("a frame's answer counts only when it says so with true", () => {
  assert.equal(isShown({ shown: true }), true);
  assert.equal(isShown({ shown: 1 }), false);
  assert.equal(isShown(undefined), false);
  assert.equal(isPresent({ present: true }), true);
  assert.equal(isPresent({ present: false }), false);
  assert.equal(isPlaced({ placed: true }), true);
  assert.equal(isPlaced({}), false);
  assert.equal(isSubmission({ submitted: true, password: false }), true);
  assert.equal(isSubmission({ submitted: true }), false);
  assert.equal(isSubmission({ submitted: "yes", password: false }), false);
});
