import assert from "node:assert/strict";
import test from "node:test";
import {
  InterfaceScale,
  type PageViewport,
  scaledViewport,
} from "./interface-size.ts";

const natural = "width=device-width, initial-scale=1, viewport-fit=cover";

/** A viewport whose width the test sets, counting the writes to its tag. */
class FakeViewport implements PageViewport {
  width = 412;
  writes = 0;
  #content = natural;
  #listener: () => void = () => {};

  get content() {
    return this.#content;
  }

  set content(content: string) {
    this.writes++;
    this.#content = content;
  }

  onResize(listener: () => void) {
    this.#listener = listener;
  }

  resize(width: number) {
    this.width = width;
    this.#listener();
  }
}

test("a larger size lays the page out narrower at a fixed scale", () => {
  assert.equal(
    scaledViewport(115, 412),
    "width=358, initial-scale=1.15, minimum-scale=1.15, maximum-scale=1.15, user-scalable=no, viewport-fit=cover",
  );
  assert.equal(
    scaledViewport(150, 412),
    "width=275, initial-scale=1.5, minimum-scale=1.5, maximum-scale=1.5, user-scalable=no, viewport-fit=cover",
  );
});

test("a smaller size lays the page out wider", () => {
  assert.equal(
    scaledViewport(85, 412),
    "width=485, initial-scale=0.85, minimum-scale=0.85, maximum-scale=0.85, user-scalable=no, viewport-fit=cover",
  );
});

test("the natural size restores the tag's own content", () => {
  const viewport = new FakeViewport();
  const scale = new InterfaceScale(viewport);
  scale.show(130);
  assert.match(viewport.content, /^width=317, initial-scale=1.3,/);
  scale.show(100);
  assert.equal(viewport.content, natural);
});

test("the natural size leaves an untouched tag alone", () => {
  const viewport = new FakeViewport();
  new InterfaceScale(viewport).show(100);
  assert.equal(viewport.writes, 0);
});

test("a turned screen lays the page out again at the same size", () => {
  const viewport = new FakeViewport();
  const scale = new InterfaceScale(viewport);
  scale.show(115);
  viewport.resize(915);
  assert.equal(viewport.content, scaledViewport(115, 915));
  const writes = viewport.writes;
  viewport.resize(915);
  assert.equal(viewport.writes, writes, "an unchanged width rewrites nothing");
});

test("a page with no size waits for one", () => {
  const viewport = new FakeViewport();
  viewport.width = 0;
  const scale = new InterfaceScale(viewport);
  scale.show(150);
  assert.equal(viewport.content, natural);
  viewport.resize(412);
  assert.equal(viewport.content, scaledViewport(150, 412));
});

test("a resize at the natural size keeps the tag's own content", () => {
  const viewport = new FakeViewport();
  const scale = new InterfaceScale(viewport);
  scale.show(115);
  scale.show(100);
  viewport.resize(915);
  assert.equal(viewport.content, natural);
});
