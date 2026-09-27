import assert from "node:assert/strict";
import test from "node:test";
import { cropSquare, photoSource } from "./photo.ts";

test("a photo is shown from its bytes, by the format they carry", () => {
  assert.equal(photoSource("/9j/4AAQ"), "data:image/jpeg;base64,/9j/4AAQ");
  assert.equal(
    photoSource("iVBORw0KGgoAAAA"),
    "data:image/png;base64,iVBORw0KGgoAAAA",
  );
});

test("no photo, or bytes of another format, show nothing", () => {
  assert.equal(photoSource(""), null);
  assert.equal(photoSource("R0lGODlh"), null);
});

test("the chosen area is rounded to a whole square", () => {
  assert.deepEqual(
    cropSquare(
      { x: 10.4, y: 20.6, width: 300.49, height: 300.51 },
      { width: 800, height: 600 },
    ),
    { x: 10, y: 21, size: 300 },
  );
});

test("the square stays inside the preview", () => {
  assert.deepEqual(
    cropSquare(
      { x: 500.7, y: -0.4, width: 300.6, height: 300.6 },
      { width: 800, height: 600 },
    ),
    { x: 499, y: 0, size: 301 },
  );
  assert.deepEqual(
    cropSquare(
      { x: 0, y: 0, width: 700.2, height: 700.2 },
      { width: 800, height: 600 },
    ),
    { x: 0, y: 0, size: 600 },
  );
});
