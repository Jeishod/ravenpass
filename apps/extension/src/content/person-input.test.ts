import assert from "node:assert/strict";
import test from "node:test";
import { PersonInput, type PressEvent, tabFocusMs } from "./person-input.ts";

function event(shape: Partial<PressEvent> & { type: string }): PressEvent {
  return { isTrusted: true, timeStamp: 1_000, ...shape };
}

test("a scripted focus with no input before it opens no field menu", () => {
  const person = new PersonInput();

  assert.equal(person.opensMenu(event({ type: "focusin" })), false);
});

test("the person's pointer press or key press on the field opens its menu", () => {
  const person = new PersonInput();

  assert.equal(person.opensMenu(event({ type: "pointerdown" })), true);
  assert.equal(
    person.opensMenu(event({ type: "keydown", key: "ArrowDown" })),
    true,
  );
  assert.equal(person.opensMenu(event({ type: "keydown", key: "a" })), true);
});

test("Tab and Escape on the field open no menu", () => {
  const person = new PersonInput();
  for (const key of ["Tab", "Escape"]) {
    assert.equal(person.opensMenu(event({ type: "keydown", key })), false);
  }
});

test("events a page dispatches open no field menu", () => {
  const person = new PersonInput();
  person.pressed(event({ type: "keydown", key: "Tab", isTrusted: false }));

  assert.equal(
    person.opensMenu(event({ type: "pointerdown", isTrusted: false })),
    false,
  );
  assert.equal(
    person.opensMenu(
      event({ type: "keydown", key: "ArrowDown", isTrusted: false }),
    ),
    false,
  );
  assert.equal(person.opensMenu(event({ type: "focusin" })), false);
});

test("the focus the person's Tab or Shift+Tab moves into a field opens its menu", () => {
  const person = new PersonInput();

  person.pressed(event({ type: "keydown", key: "Tab", timeStamp: 1_000 }));
  assert.equal(
    person.opensMenu(event({ type: "focusin", timeStamp: 1_002 })),
    true,
  );
});

test("a focus later than 100 ms after Tab opens nothing", () => {
  const person = new PersonInput();

  person.pressed(event({ type: "keydown", key: "Tab", timeStamp: 1_000 }));
  assert.equal(
    person.opensMenu(
      event({ type: "focusin", timeStamp: 1_000 + tabFocusMs + 1 }),
    ),
    false,
  );
});

test("one Tab press opens one menu: a scripted focus right after the first opens nothing", () => {
  const person = new PersonInput();
  person.pressed(event({ type: "keydown", key: "Tab", timeStamp: 1_000 }));
  person.opensMenu(event({ type: "focusin", timeStamp: 1_001 }));

  assert.equal(
    person.opensMenu(event({ type: "focusin", timeStamp: 1_010 })),
    false,
  );
});

test("keys other than Tab pressed elsewhere open nothing on a later focus", () => {
  const person = new PersonInput();
  person.pressed(event({ type: "keydown", key: "Enter", timeStamp: 1_000 }));

  assert.equal(
    person.opensMenu(event({ type: "focusin", timeStamp: 1_001 })),
    false,
  );
});
