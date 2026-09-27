import assert from "node:assert/strict";
import test from "node:test";
import { linkName } from "./name.ts";

test("the name is the browser's brand and the platform", () => {
  assert.equal(
    linkName({
      brands: [
        { brand: "Not A(Brand" },
        { brand: "Chromium" },
        { brand: "Google Chrome" },
      ],
      platform: "macOS",
    }),
    "Chrome · macOS",
  );
  assert.equal(
    linkName({
      brands: [{ brand: "Microsoft Edge" }, { brand: "Chromium" }],
      platform: "Windows",
    }),
    "Edge · Windows",
  );
});

test("a browser that names only its engine is named after it", () => {
  assert.equal(
    linkName({
      brands: [{ brand: "Chromium" }, { brand: "Not)A;Brand" }],
      platform: "Linux",
    }),
    "Chromium · Linux",
  );
});

test("without client hints the name is Chrome", () => {
  assert.equal(linkName(undefined), "Chrome");
});

test("the name fits the 64 characters the desktop app accepts", () => {
  const name = linkName({
    brands: [{ brand: "Ω".repeat(80) }],
    platform: "macOS",
  });
  assert.equal(Array.from(name).length, 64);
});
