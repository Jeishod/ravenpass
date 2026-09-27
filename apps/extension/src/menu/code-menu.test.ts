import assert from "node:assert/strict";
import { afterEach, beforeEach, mock, test } from "node:test";
import { periodEnd } from "@ravenpass/ui/credentials/one-time-code.ts";
import { CodeMenu } from "./code-menu.ts";

// The thirty- and sixty-second periods holding this moment both end at 1_700_000_040_000.
const periodStart = 1_700_000_010_000;

beforeEach(() => {
  mock.timers.enable({ apis: ["setTimeout", "Date"], now: periodStart });
});

afterEach(() => {
  mock.timers.reset();
});

/** Lets the answers already given reach the menu. */
function settle(): Promise<void> {
  return new Promise((resolve) => setImmediate(resolve));
}

function codeMenu() {
  const asked: string[] = [];
  const filled: string[] = [];
  const menu = new CodeMenu({
    rows: [
      { id: "a1", period: 30 },
      { id: "b2", period: 60 },
    ],
    reveal: async (id) => {
      asked.push(id);
      return {
        code: `${id}:${asked.length}`,
        digits: 6,
        period: 30,
        expiresAt: periodEnd(30, Date.now()),
      };
    },
    fill: (id) => {
      filled.push(id);
    },
  });
  return { menu, asked, filled };
}

test("no code is asked for to show the rows, highlight one, or hold Option over none", async () => {
  const { menu, asked } = codeMenu();

  menu.highlight("a1");
  menu.highlight("b2");
  menu.highlight(null);
  menu.hold(true);
  mock.timers.tick(90_000);
  await settle();

  assert.deepEqual(asked, []);
  assert.deepEqual(menu.view(), { revealed: null, waiting: null });
});

test("holding Option reveals the highlighted row's code alone, and releasing it masks the code", async () => {
  const { menu, asked } = codeMenu();

  menu.highlight("a1");
  menu.hold(true);
  await settle();
  assert.deepEqual(menu.view().revealed, { id: "a1", code: "a1:1" });

  menu.hold(false);
  assert.equal(menu.view().revealed, null);
  assert.deepEqual(asked, ["a1"]);
});

test("moving the highlight masks the code, and reveals the next row's while Option is held", async () => {
  const { menu, asked } = codeMenu();
  menu.hold(true);
  menu.highlight("a1");
  await settle();

  menu.highlight("b2");
  assert.equal(menu.view().revealed, null);
  await settle();
  assert.deepEqual(menu.view().revealed, { id: "b2", code: "b2:2" });

  menu.highlight(null);
  assert.equal(menu.view().revealed, null);
  assert.deepEqual(asked, ["a1", "b2"]);
});

test("the end of the period masks the code and asks for the next one while Option is held", async () => {
  const { menu, asked } = codeMenu();
  menu.highlight("a1");
  menu.hold(true);
  await settle();

  mock.timers.tick(29_999);
  assert.deepEqual(menu.view().revealed, { id: "a1", code: "a1:1" });
  mock.timers.tick(1);
  assert.equal(menu.view().revealed, null);
  await settle();
  assert.deepEqual(menu.view().revealed, { id: "a1", code: "a1:2" });

  menu.hold(false);
  mock.timers.tick(90_000);
  await settle();
  assert.deepEqual(asked, ["a1", "a1"]);
  assert.equal(menu.view().revealed, null);
});

test("a code asked for and arriving after Option is released is shown, as after a PIN typed on the desktop", async () => {
  const { menu, asked } = codeMenu();
  menu.highlight("a1");

  menu.hold(true);
  menu.hold(false);
  menu.hold(true);
  await settle();

  assert.deepEqual(menu.view().revealed, { id: "a1", code: "a1:1" });
  assert.deepEqual(asked, ["a1"]);
});

test("a code on its way is dropped when another row is highlighted", async () => {
  const { menu, asked } = codeMenu();
  menu.highlight("a1");

  menu.hold(true);
  menu.highlight("a2");
  await settle();

  assert.deepEqual(asked, ["a1", "a2"]);
  assert.equal(menu.view().revealed?.id, "a2");
});

test("a row chosen with more than five seconds left fills at once", () => {
  const { menu, asked, filled } = codeMenu();
  mock.timers.tick(24_999);

  menu.choose("a1");

  assert.deepEqual(filled, ["a1"]);
  assert.equal(menu.view().waiting, null);
  assert.deepEqual(asked, []);
});

test("a row chosen in the last five seconds waits for the next period, then fills", () => {
  const { menu, asked, filled } = codeMenu();
  mock.timers.tick(25_000);

  menu.choose("a1");
  assert.deepEqual(filled, []);
  assert.equal(menu.view().waiting, "a1");

  mock.timers.tick(4_999);
  assert.deepEqual(filled, []);
  mock.timers.tick(1);
  assert.deepEqual(filled, ["a1"]);
  assert.equal(menu.view().waiting, null);
  assert.deepEqual(asked, []);
});

test("each row's closing seconds follow its own period", () => {
  const { menu, filled } = codeMenu();
  // The thirty-second period ends in 4 s, the sixty-second one in 34 s.
  mock.timers.tick(56_000);

  menu.choose("b2");
  assert.deepEqual(filled, ["b2"]);
  menu.choose("a1");
  assert.deepEqual(filled, ["b2"]);
  assert.equal(menu.view().waiting, "a1");
});

test("cancelling the wait fills nothing, and there is then nothing to cancel", () => {
  const { menu, filled } = codeMenu();
  mock.timers.tick(27_000);
  menu.choose("a1");

  assert.equal(menu.cancel(), true);
  assert.equal(menu.view().waiting, null);
  mock.timers.tick(60_000);
  assert.deepEqual(filled, []);
  assert.equal(menu.cancel(), false);
});

test("choosing another row replaces the wait", () => {
  const { menu, filled } = codeMenu();
  mock.timers.tick(27_000);
  menu.choose("a1");

  menu.choose("b2");
  assert.equal(menu.view().waiting, "b2");
  mock.timers.tick(3_000);
  assert.deepEqual(filled, ["b2"]);
});

test("a row the menu does not list is not filled", () => {
  const { menu, filled } = codeMenu();

  menu.choose("c3");

  assert.deepEqual(filled, []);
});

test("stopping drops a waiting fill and a code on its way", async () => {
  const { menu, filled } = codeMenu();
  mock.timers.tick(27_000);
  menu.choose("a1");
  menu.highlight("b2");
  menu.hold(true);

  menu.stop();
  mock.timers.tick(60_000);
  await settle();

  assert.deepEqual(filled, []);
  assert.equal(menu.view().revealed, null);
});
