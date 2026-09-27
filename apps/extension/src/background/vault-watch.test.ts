import assert from "node:assert/strict";
import { afterEach, beforeEach, mock, test } from "node:test";
import type { LinkStatus } from "../link/client.ts";
import type { VaultState, VaultStateMessage } from "../messages.ts";
import { VaultWatch, type WatchPort } from "./vault-watch.ts";

const everyMs = 1500;

beforeEach(() => {
  mock.timers.enable({ apis: ["setTimeout"] });
});

afterEach(() => {
  mock.timers.reset();
});

function settle(): Promise<void> {
  return new Promise((resolve) => setImmediate(resolve));
}

class FakePort implements WatchPort {
  readonly received: VaultState[] = [];
  private readonly disconnects: (() => void)[] = [];

  readonly onDisconnect = {
    addListener: (listener: () => void) => {
      this.disconnects.push(listener);
    },
  };

  postMessage(message: VaultStateMessage): void {
    this.received.push(message.state);
  }

  disconnect(): void {
    for (const listener of this.disconnects) listener();
  }
}

function linked(desktop: "unlocked" | "locked" | "not-open"): LinkStatus {
  return { linked: true, desktop, linkedAt: 0 };
}

/** A watch whose desktop app answers `state`, which the test changes. */
function watch() {
  const desktop = { status: linked("locked") as LinkStatus, asked: 0 };
  const vault = new VaultWatch({
    status: async () => {
      desktop.asked += 1;
      return desktop.status;
    },
    everyMs,
  });
  return { vault, desktop };
}

test("a port receives the state at once and then each change", async () => {
  const { vault, desktop } = watch();
  const port = new FakePort();
  vault.connect(port);
  await settle();
  assert.deepEqual(port.received, ["locked"]);

  mock.timers.tick(everyMs);
  await settle();
  assert.deepEqual(port.received, ["locked"]);

  desktop.status = linked("unlocked");
  mock.timers.tick(everyMs);
  await settle();
  desktop.status = { linked: false, unlinkedInRavenpass: true };
  mock.timers.tick(everyMs);
  await settle();
  assert.deepEqual(port.received, ["locked", "unlocked", "unlinked"]);
});

test("a port that connects later receives the state known", async () => {
  const { vault } = watch();
  const first = new FakePort();
  vault.connect(first);
  await settle();

  const second = new FakePort();
  vault.connect(second);
  assert.deepEqual(second.received, ["locked"]);
  await settle();
  assert.deepEqual(first.received, ["locked"]);
});

test("the watch asks nothing once every port disconnected", async () => {
  const { vault, desktop } = watch();
  const port = new FakePort();
  vault.connect(port);
  await settle();
  const asked = desktop.asked;

  port.disconnect();
  mock.timers.tick(everyMs * 4);
  await settle();
  assert.equal(desktop.asked, asked);
});

test("a second port does not start a second round of questions", async () => {
  const { vault, desktop } = watch();
  vault.connect(new FakePort());
  await settle();
  vault.connect(new FakePort());
  await settle();
  const asked = desktop.asked;

  mock.timers.tick(everyMs);
  await settle();
  assert.equal(desktop.asked, asked + 1);
});
