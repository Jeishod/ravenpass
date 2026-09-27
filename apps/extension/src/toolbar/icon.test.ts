import assert from "node:assert/strict";
import { existsSync } from "node:fs";
import test from "node:test";
import manifest from "../manifest.json" with { type: "json" };
import { type ColorScheme, toolbarIcon } from "./icon.ts";

const schemes: readonly ColorScheme[] = ["light", "dark"];

test("a light toolbar shows the icon Chrome shows before the service worker runs", () => {
  const named = Object.fromEntries(
    Object.entries(manifest.action.default_icon).map(([size, path]) => [
      size,
      `/${path}`,
    ]),
  );
  assert.deepEqual(toolbarIcon("light"), named);
});

test("each colour scheme has its own icon", () => {
  assert.notDeepEqual(toolbarIcon("light"), toolbarIcon("dark"));
});

test("every toolbar icon ships with the extension", () => {
  for (const scheme of schemes) {
    for (const path of Object.values(toolbarIcon(scheme))) {
      const sources = [
        new URL(`..${path}`, import.meta.url),
        new URL(`../../public${path}`, import.meta.url),
      ];
      assert.ok(
        sources.some((source) => existsSync(source)),
        `${path} is in neither src/ nor public/.`,
      );
    }
  }
});
