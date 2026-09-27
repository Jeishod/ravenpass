import assert from "node:assert/strict";
import test from "node:test";
import { Shortcuts, shortcutDefinitions } from "./shortcuts.ts";

test("every action starts on its default", () => {
  for (const { action, hotkey } of shortcutDefinitions) {
    assert.equal(Shortcuts.defaults.hotkey(action), hotkey);
    assert.ok(Shortcuts.defaults.isDefault(action));
  }
});

test("recorded hotkeys replace the defaults of known actions only", () => {
  const shortcuts = Shortcuts.fromRecorded({
    palette: "Mod+P",
    search: "",
    unknown: "Mod+U",
  });
  assert.equal(shortcuts.hotkey("palette"), "Mod+P");
  assert.ok(!shortcuts.isDefault("palette"));
  assert.equal(shortcuts.hotkey("search"), "Mod+F");
  assert.ok(shortcuts.isDefault("search"));
});

test("an invalid recorded hotkey keeps the default", () => {
  const shortcuts = Shortcuts.fromRecorded({ palette: "Mod+" });
  assert.equal(shortcuts.hotkey("palette"), "Mod+K");
});

test("a hotkey another action answers to is refused, naming that action", () => {
  const shortcuts = Shortcuts.defaults;
  assert.deepEqual(shortcuts.refusal("Mod+F", "palette"), {
    kind: "taken",
    by: "search",
  });
  assert.equal(shortcuts.refusal("Mod+K", "palette"), null);
  assert.equal(shortcuts.refusal("Mod+J", "palette"), null);
});

test("editing and system hotkeys are refused", () => {
  for (const hotkey of ["Mod+C", "Mod+V", "Mod+Q", "Mod+Shift+Z"] as const) {
    assert.deepEqual(Shortcuts.defaults.refusal(hotkey, "palette"), {
      kind: "reserved",
    });
  }
});

test("a hotkey without a modifier is refused, since typing would set it off", () => {
  assert.deepEqual(Shortcuts.defaults.refusal("K", "palette"), {
    kind: "bare",
  });
  assert.deepEqual(Shortcuts.defaults.refusal("Shift+K", "palette"), {
    kind: "bare",
  });
  assert.equal(Shortcuts.defaults.refusal("Alt+K", "palette"), null);
});

test("changing one action leaves the others and can be undone", () => {
  const changed = Shortcuts.defaults.with("search", "Mod+Shift+F");
  assert.equal(changed.hotkey("search"), "Mod+Shift+F");
  assert.equal(changed.hotkey("palette"), "Mod+K");
  assert.equal(Shortcuts.defaults.hotkey("search"), "Mod+F");
  assert.ok(changed.with("search", null).isDefault("search"));
});
