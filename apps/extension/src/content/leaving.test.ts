import assert from "node:assert/strict";
import { afterEach, beforeEach, mock, test } from "node:test";
import { Arming, armMs, Leaving } from "./leaving.ts";
import type { PageChanges } from "./page-changes.ts";

beforeEach(() => {
  mock.timers.enable({ apis: ["setTimeout"] });
});

afterEach(() => {
  mock.timers.reset();
});

class Page {
  private readonly observers = new Set<() => void>();

  readonly changes: PageChanges = (onChange) => {
    this.observers.add(onChange);
    return () => this.observers.delete(onChange);
  };

  get observed(): number {
    return this.observers.size;
  }

  change(): void {
    for (const observer of [...this.observers]) observer();
  }
}

class Form {
  readonly name: string;
  left = false;

  constructor(name: string) {
    this.name = name;
  }
}

function arming(page = new Page()) {
  const captured: [string, boolean][] = [];
  const armed = new Arming<Form>({
    left: (form) => form.left,
    capture: (form, left) => captured.push([form.name, left]),
    changes: () => page.changes,
  });
  return { armed, captured, page };
}

test("a watched form is reported once, on the first page change after it left", () => {
  const page = new Page();
  const form = new Form("sign-in");
  let reports = 0;
  new Leaving(
    () => form.left,
    () => {
      reports += 1;
    },
    60_000,
    page.changes,
  );

  page.change();
  assert.equal(reports, 0);
  form.left = true;
  page.change();
  page.change();

  assert.equal(reports, 1);
  assert.equal(page.observed, 0);
});

test("a watch gives up after its time, and a stopped one reports nothing", () => {
  const page = new Page();
  const late = new Form("late");
  const stopped = new Form("stopped");
  let reports = 0;
  const report = () => {
    reports += 1;
  };
  const lateWatch = new Leaving(() => late.left, report, 1000, page.changes);
  const stoppedWatch = new Leaving(
    () => stopped.left,
    report,
    60_000,
    page.changes,
  );

  stoppedWatch.stop();
  mock.timers.tick(1000);
  late.left = true;
  stopped.left = true;
  page.change();

  assert.equal(reports, 0);
  assert.equal(lateWatch.watching, false);
  assert.equal(stoppedWatch.watching, false);
  assert.equal(page.observed, 0);
});

test("an armed form that leaves the page within ten seconds is captured, its sign-in done", () => {
  const { armed, captured, page } = arming();
  const form = new Form("sign-in");

  armed.arm(form);
  mock.timers.tick(armMs - 1000);
  form.left = true;
  page.change();

  assert.deepEqual(captured, [["sign-in", true]]);
  armed.pageHidden();
  assert.deepEqual(captured, [["sign-in", true]]);
});

test("an armed form that stays, as after a failed sign-in or a show-password toggle, is let go after ten seconds", () => {
  const { armed, captured, page } = arming();
  const form = new Form("sign-in");

  armed.arm(form);
  mock.timers.tick(armMs);
  form.left = true;
  page.change();
  armed.pageHidden();

  assert.deepEqual(captured, []);
});

test("the form armed when the page is hidden is captured then, once", () => {
  const { armed, captured, page } = arming();
  const form = new Form("sign-in");

  armed.arm(form);
  mock.timers.tick(armMs - 1);
  armed.pageHidden();
  armed.pageHidden();
  form.left = true;
  page.change();

  assert.deepEqual(captured, [["sign-in", false]]);
});

test("arming another form, or disarming, lets the armed one go", () => {
  const { armed, captured, page } = arming();
  const first = new Form("first");
  const second = new Form("second");

  armed.arm(first);
  armed.arm(second);
  armed.disarm((form) => form === first);
  first.left = true;
  page.change();
  assert.deepEqual(captured, []);

  armed.disarm((form) => form === second);
  second.left = true;
  page.change();
  armed.pageHidden();
  assert.deepEqual(captured, []);
  assert.equal(page.observed, 0);
});
