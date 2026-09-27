import assert from "node:assert/strict";
import test from "node:test";
import { type GuardedHost, HostGuard } from "./host-guard.ts";

class Host implements GuardedHost {
  readonly attributes = new Map<string, string>([
    ["style", "position: fixed !important; top: 10px !important;"],
    ["popover", "manual"],
  ]);

  getAttribute(name: string): string | null {
    return this.attributes.get(name) ?? null;
  }
}

function guarded() {
  const host = new Host();
  let tampered = 0;
  const guard = new HostGuard(host, () => {
    tampered += 1;
  });
  return { host, guard, tampered: () => tampered };
}

test("Ravenpass's own restyling of the host keeps the menu", () => {
  const { host, guard, tampered } = guarded();

  host.attributes.set(
    "style",
    "position: fixed !important; top: 42px !important;",
  );
  guard.settle();
  guard.review();

  assert.equal(tampered(), 0);
});

test("the page restyling the host, or changing its class or popover, closes the menu", () => {
  const changes: [string, string | null][] = [
    ["style", "opacity: 0 !important;"],
    ["class", "decoy"],
    ["popover", null],
  ];
  for (const [name, value] of changes) {
    const { host, guard, tampered } = guarded();

    if (value === null) host.attributes.delete(name);
    else host.attributes.set(name, value);
    guard.review();

    assert.equal(tampered(), 1, name);
  }
});
