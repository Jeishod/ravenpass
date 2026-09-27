import assert from "node:assert/strict";
import { describe, it } from "node:test";
import {
  backdropFilter,
  composite,
  luminance,
  type Rgba,
  tintAlpha,
} from "./backdrop.ts";

const white: Rgba = { red: 1, green: 1, blue: 1, alpha: 1 };
const black: Rgba = { red: 0, green: 0, blue: 0, alpha: 1 };

function brightnessOf(filter: string): number {
  const match = /brightness\(([\d.]+)\)/.exec(filter);
  assert.ok(match, filter);
  return Number(match[1]);
}

/** What the blurred page adds to the menu's colour for a spot of `spot` luminance. */
function light(filter: string, spot: number): number {
  return brightnessOf(filter) * spot * (1 - tintAlpha);
}

describe("backdropFilter", () => {
  it("adds the same light from a page's own background on light and mid pages", () => {
    assert.ok(
      Math.abs(light(backdropFilter(1), 1) - light(backdropFilter(0.4), 0.4)) <
        0.002,
    );
  });

  it("lets a dark page show through more than a light one", () => {
    assert.ok(
      brightnessOf(backdropFilter(0.1)) > brightnessOf(backdropFilter(1)) * 3.5,
    );
  });

  it("keeps a white spot behind the menu dim on a dark page", () => {
    assert.ok(light(backdropFilter(0.02), 1) <= 0.321);
  });

  it("survives a black page", () => {
    assert.ok(Number.isFinite(brightnessOf(backdropFilter(0))));
  });
});

describe("composite", () => {
  it("lays translucent colours over the base in order", () => {
    const half = { ...white, alpha: 0.5 };
    assert.ok(
      Math.abs(luminance(composite(black, [half, half])) - 0.75) < 1e-9,
    );
  });

  it("covers the base with an opaque layer", () => {
    assert.equal(luminance(composite(white, [black])), 0);
  });
});
