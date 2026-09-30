import assert from "node:assert/strict";
import test from "node:test";
import { appearanceAttribute, appearanceOf } from "./appearance.ts";

test("an unknown appearance follows the system", () => {
  assert.equal(appearanceOf("sepia"), "system");
  assert.equal(appearanceOf(undefined), "system");
  assert.equal(appearanceOf("dark"), "dark");
});

test("only a chosen light or dark marks the page", () => {
  assert.equal(appearanceAttribute("system"), null);
  assert.equal(appearanceAttribute("light"), "light");
  assert.equal(appearanceAttribute("dark"), "dark");
});
