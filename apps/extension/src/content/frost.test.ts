import assert from "node:assert/strict";
import test from "node:test";
import { Frost, type Styled } from "./frost.ts";

const filter = "blur(14px) saturate(1.6) brightness(0.131)";

class StyledElement implements Styled {
  readonly properties = new Map<string, string>();
  readonly style = {
    setProperty: (property: string, value: string | null) => {
      this.properties.set(property, value ?? "");
    },
    removeProperty: (property: string) => {
      const value = this.properties.get(property) ?? "";
      this.properties.delete(property);
      return value;
    },
  };
}

function frost() {
  const surface = new StyledElement();
  const layer = new StyledElement();
  return { surface, layer, frost: new Frost(surface, layer, filter) };
}

test("the surface carries the blur while it animates in", () => {
  const { surface, layer, frost: blur } = frost();

  blur.moving();

  assert.equal(surface.properties.get("backdrop-filter"), filter);
  assert.equal(layer.properties.has("backdrop-filter"), false);
});

test("once still, only the layer beneath the frame carries the blur", () => {
  const { surface, layer, frost: blur } = frost();
  blur.moving();

  blur.settled();

  assert.equal(layer.properties.get("backdrop-filter"), filter);
  assert.equal(surface.properties.has("backdrop-filter"), false);
});

test("the blur returns to the surface for the exit", () => {
  const { surface, layer, frost: blur } = frost();
  blur.moving();
  blur.settled();

  blur.moving();

  assert.equal(surface.properties.get("backdrop-filter"), filter);
  assert.equal(layer.properties.has("backdrop-filter"), false);
});
