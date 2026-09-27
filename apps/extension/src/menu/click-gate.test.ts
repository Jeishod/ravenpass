import assert from "node:assert/strict";
import test from "node:test";
import { ClickGate, type GatedEvent, stillMs } from "./click-gate.ts";

function clickGate() {
  const clock = { time: 1_000 };
  const gate = new ClickGate({ now: () => clock.time });
  return { clock, gate };
}

/** A click that records whether the gate stopped it. */
function click() {
  const stopped = { defaultPrevented: false, propagation: true };
  const event: GatedEvent = {
    preventDefault: () => {
      stopped.defaultPrevented = true;
    },
    stopImmediatePropagation: () => {
      stopped.propagation = false;
    },
  };
  return { event, stopped };
}

test("a click is refused while Chrome has not reported the menu visible", () => {
  const { clock, gate } = clickGate();
  clock.time += stillMs * 4;

  const { event, stopped } = click();
  gate.screen(event);

  assert.equal(gate.accepts(), false);
  assert.deepEqual(stopped, { defaultPrevented: true, propagation: false });
});

test("a click is refused within 500 ms of the menu becoming visible, and accepted after", () => {
  const { clock, gate } = clickGate();
  gate.visibility(true);

  clock.time += stillMs - 1;
  assert.equal(gate.accepts(), false);

  clock.time += 1;
  const { event, stopped } = click();
  gate.screen(event);
  assert.equal(gate.accepts(), true);
  assert.deepEqual(stopped, { defaultPrevented: false, propagation: true });
});

test("a click is refused once the page hides or covers the menu", () => {
  const { clock, gate } = clickGate();
  gate.visibility(true);
  clock.time += stillMs;

  gate.visibility(false);

  assert.equal(gate.accepts(), false);
});

test("a menu shown again waits 500 ms again", () => {
  const { clock, gate } = clickGate();
  gate.visibility(true);
  clock.time += stillMs;
  gate.visibility(false);
  clock.time += stillMs;

  gate.visibility(true);
  assert.equal(gate.accepts(), false);
  clock.time += stillMs;
  assert.equal(gate.accepts(), true);
});

test("a repeated visible report does not restart the wait", () => {
  const { clock, gate } = clickGate();
  gate.visibility(true);
  clock.time += stillMs;

  gate.visibility(true);

  assert.equal(gate.accepts(), true);
});

test("the still period counts from the end of the entrance, which the frame reports", () => {
  const { clock, gate } = clickGate();
  gate.visibility(true);
  clock.time += 200;

  gate.moved();
  clock.time += stillMs - 1;
  assert.equal(gate.accepts(), false);

  clock.time += 1;
  assert.equal(gate.accepts(), true);
});

test("a click is refused within 500 ms of the menu moving", () => {
  const { clock, gate } = clickGate();
  gate.visibility(true);
  clock.time += stillMs * 2;

  gate.moved();
  clock.time += stillMs - 1;
  assert.equal(gate.accepts(), false);

  clock.time += 1;
  assert.equal(gate.accepts(), true);
});
