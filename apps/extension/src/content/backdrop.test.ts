import assert from "node:assert/strict";
import { describe, it } from "node:test";
import {
  backdropFilter,
  composite,
  frostings,
  luminance,
  type Rgba,
} from "./backdrop.ts";

const white: Rgba = { red: 1, green: 1, blue: 1, alpha: 1 };
const black: Rgba = { red: 0, green: 0, blue: 0, alpha: 1 };
const { dark, light: lightFrost } = frostings;

function brightnessOf(filter: string): number {
  const match = /brightness\(([\d.]+)\)/.exec(filter);
  assert.ok(match, filter);
  return Number(match[1]);
}

/** What the blurred page adds to the dark menu's colour for a spot of `spot` luminance. */
function light(filter: string, spot: number, tint = dark.tint): number {
  return brightnessOf(filter) * spot * (1 - tint);
}

describe("backdropFilter", () => {
  it("adds the same light from a page's own background on light and mid pages", () => {
    assert.ok(
      Math.abs(
        light(backdropFilter(1, dark), 1) -
          light(backdropFilter(0.4, dark), 0.4),
      ) < 0.002,
    );
  });

  it("lets a dark page show through more than a light one", () => {
    assert.ok(
      brightnessOf(backdropFilter(0.1, dark)) >
        brightnessOf(backdropFilter(1, dark)) * 3.5,
    );
  });

  it("keeps a white spot behind the menu dim on a dark page", () => {
    assert.ok(light(backdropFilter(0.02, dark), 1) <= 0.321);
  });

  it("survives a black page", () => {
    assert.ok(Number.isFinite(brightnessOf(backdropFilter(0, dark))));
  });

  it("keeps the light menu light over a black page and at most white over a white spot", () => {
    const tint = lightFrost.tint;
    assert.ok(tint >= 0.78);
    assert.ok(tint + light(backdropFilter(0.02, lightFrost), 1, tint) <= 1.001);
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
