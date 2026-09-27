import assert from "node:assert/strict";
import test from "node:test";
import { type BooleanSetting, ChromeAutofill } from "./chrome-autofill.ts";
import { MemoryLocalArea } from "./test-doubles.test-support.ts";

type LevelOfControl = chrome.types.LevelOfControl;

/** A Chrome setting that records what the extension asked of it. */
class ScriptedSetting implements BooleanSetting {
  levelOfControl: LevelOfControl;
  value = true;
  failure: Error | null = null;
  readonly calls: string[] = [];

  constructor(
    levelOfControl: LevelOfControl = "controllable_by_this_extension",
  ) {
    this.levelOfControl = levelOfControl;
  }

  async get(): Promise<{ levelOfControl: LevelOfControl; value: boolean }> {
    if (this.failure) throw this.failure;
    return { levelOfControl: this.levelOfControl, value: this.value };
  }

  async set({ value }: { value: boolean }): Promise<void> {
    this.calls.push(`set ${value}`);
    this.value = value;
    this.levelOfControl = "controlled_by_this_extension";
  }

  async clear(): Promise<void> {
    this.calls.push("clear");
    this.value = true;
    this.levelOfControl = "controllable_by_this_extension";
  }
}

function autofill(settings: ScriptedSetting[], link = { linked: true }) {
  const area = new MemoryLocalArea();
  return {
    area,
    choice: new ChromeAutofill({
      area,
      settings: () => settings,
      linked: async () => link.linked,
    }),
  };
}

test("an unlinked extension leaves Chrome's autofill alone and gives it back on unlinking", async () => {
  const link = { linked: false };
  const settings = [new ScriptedSetting()];
  const { choice } = autofill(settings, link);

  await choice.apply();
  assert.deepEqual(settings[0]?.calls, []);

  link.linked = true;
  await choice.apply();
  link.linked = false;
  await choice.apply();
  assert.deepEqual(settings[0]?.calls, ["set false", "clear"]);
});

test("the link marker kept in storage counts as linked", async () => {
  const area = new MemoryLocalArea();
  const settings = [new ScriptedSetting()];
  const choice = new ChromeAutofill({ area, settings: () => settings });

  await choice.apply();
  assert.deepEqual(settings[0]?.calls, []);

  await area.set({ desktopSignIn: "card" });
  await choice.apply();
  assert.deepEqual(settings[0]?.calls, ["set false"]);
});

test("Chrome's autofill is off until the person turns it back on", async () => {
  const settings = [new ScriptedSetting(), new ScriptedSetting()];
  const { choice } = autofill(settings);

  assert.equal(await choice.turnedOff(), true);
  await choice.apply();

  assert.deepEqual(
    settings.map(({ calls }) => calls),
    [["set false"], ["set false"]],
  );
});

test("turning it back on clears only what the extension controls", async () => {
  const ours = new ScriptedSetting();
  const untouched = new ScriptedSetting();
  const { choice } = autofill([ours, untouched]);
  await choice.apply();
  untouched.levelOfControl = "controllable_by_this_extension";

  await choice.turnOff(false);
  await choice.apply();

  assert.equal(await choice.turnedOff(), false);
  assert.deepEqual(ours.calls, ["set false", "clear"]);
  assert.deepEqual(untouched.calls, ["set false"]);
  assert.equal(ours.value, true);
});

test("a setting another extension or a policy controls is left alone", async () => {
  const settings = [
    new ScriptedSetting("controlled_by_other_extensions"),
    new ScriptedSetting("not_controllable"),
    new ScriptedSetting(),
  ];
  const { choice } = autofill(settings);

  await choice.apply();
  await choice.turnOff(false);
  await choice.apply();

  assert.deepEqual(
    settings.map(({ calls }) => calls),
    [[], [], ["set false", "clear"]],
  );
});

test("a setting that fails leaves the others applied and reports the failure", async () => {
  const failing = new ScriptedSetting();
  failing.failure = new Error("unavailable");
  const working = new ScriptedSetting();
  const { choice } = autofill([failing, working]);

  await assert.rejects(choice.apply(), /unavailable/);
  assert.deepEqual(working.calls, ["set false"]);

  failing.failure = null;
  await choice.apply();
  assert.deepEqual(failing.calls, ["set false"]);
});

test("the watch tells of a change to the choice and of no other key", async () => {
  const { area, choice } = autofill([]);
  let changes = 0;
  const stop = choice.watch(() => {
    changes += 1;
  });

  await area.set({ language: "ru" });
  await choice.turnOff(false);
  stop();
  await choice.turnOff(true);

  assert.equal(changes, 1);
});
