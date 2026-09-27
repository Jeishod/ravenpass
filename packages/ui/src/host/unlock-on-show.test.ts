import assert from "node:assert/strict";
import test from "node:test";
import { type LockedScreen, UnlockOnShow } from "./unlock-on-show.ts";

const ready: LockedScreen = {
  visible: true,
  deviceUnlock: true,
  allowed: true,
  occupied: false,
};
const hidden: LockedScreen = { ...ready, visible: false };
const unlocking: LockedScreen = { ...ready, occupied: true };
const heldBack: LockedScreen = { ...ready, allowed: false };

test("a screen shown with the device's unlock asks once", () => {
  const prompts = new UnlockOnShow();
  assert.equal(prompts.next(ready), true);
  assert.equal(prompts.next(unlocking), false);
  assert.equal(
    prompts.next(ready),
    false,
    "a failed unlock does not ask again",
  );
});

test("a screen waits to know how the vault opens", () => {
  const prompts = new UnlockOnShow();
  assert.equal(prompts.next({ ...ready, deviceUnlock: false }), false);
  assert.equal(prompts.next(ready), true);
});

test("a screen waits for the host to allow asking", () => {
  const prompts = new UnlockOnShow();
  assert.equal(prompts.next(heldBack), false);
  assert.equal(prompts.next(ready), true);
});

test("a screen the owner locked into asks only after the app returns", () => {
  const prompts = new UnlockOnShow();
  assert.equal(prompts.next(heldBack), false);
  assert.equal(prompts.next(heldBack), false);
  assert.equal(prompts.next({ ...heldBack, visible: false }), false);
  assert.equal(prompts.next(ready), true);
});

test("a vault without the device's unlock never asks", () => {
  const prompts = new UnlockOnShow();
  const pinOnly = { ...ready, deviceUnlock: false };
  assert.equal(prompts.next(pinOnly), false);
  assert.equal(prompts.next(hidden), false);
  assert.equal(prompts.next(pinOnly), false);
});

test("a page loaded off screen asks once it shows", () => {
  const prompts = new UnlockOnShow();
  assert.equal(prompts.next(hidden), false);
  assert.equal(prompts.next(ready), true);
});

test("each return to the screen asks again", () => {
  const prompts = new UnlockOnShow();
  assert.equal(prompts.next(ready), true);
  assert.equal(prompts.next(hidden), false);
  assert.equal(prompts.next(ready), true);
});

test("a showing spent on something else asks nothing", () => {
  const prompts = new UnlockOnShow();
  assert.equal(prompts.next({ ...unlocking, deviceUnlock: false }), false);
  assert.equal(prompts.next(ready), false);
});

test("a page hidden while an unlock runs asks nothing on its return", () => {
  const prompts = new UnlockOnShow();
  assert.equal(prompts.next(ready), true);
  assert.equal(prompts.next(unlocking), false);
  assert.equal(prompts.next({ ...unlocking, visible: false }), false);
  assert.equal(prompts.next(hidden), false);
  assert.equal(prompts.next(ready), false);
  assert.equal(prompts.next(hidden), false);
  assert.equal(prompts.next(ready), true, "the next showing asks again");
});
