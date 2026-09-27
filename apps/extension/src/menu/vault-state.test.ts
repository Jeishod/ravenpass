import assert from "node:assert/strict";
import { test } from "node:test";
import type { VaultState } from "../messages.ts";
import { FakeVault } from "./vault-state.test-support.ts";
import { LockedWatch, type StatePort, watchVaultState } from "./vault-state.ts";

class FakePort implements StatePort {
  disconnected = false;
  private readonly messages: ((message: unknown) => void)[] = [];
  private readonly disconnects: (() => void)[] = [];

  readonly onMessage = {
    addListener: (listener: (message: unknown) => void) => {
      this.messages.push(listener);
    },
  };

  readonly onDisconnect = {
    addListener: (listener: () => void) => {
      this.disconnects.push(listener);
    },
  };

  send(message: unknown): void {
    for (const listener of this.messages) listener(message);
  }

  drop(): void {
    for (const listener of this.disconnects) listener();
  }

  disconnect(): void {
    this.disconnected = true;
  }
}

function watching() {
  const ports: FakePort[] = [];
  const states: VaultState[] = [];
  const stop = watchVaultState(
    (state) => states.push(state),
    () => {
      const port = new FakePort();
      ports.push(port);
      return port;
    },
  );
  const port = (index: number): FakePort => {
    const opened = ports[index];
    assert.ok(opened, `port ${index} was opened`);
    return opened;
  };
  return { ports, port, states, stop };
}

test("the listener hears each vault state and nothing else", () => {
  const { port, states } = watching();
  port(0).send({ kind: "vault-state", state: "locked" });
  port(0).send({ kind: "vault-state", state: "open" });
  port(0).send({ kind: "menu-close", token: "t" });
  port(0).send({ kind: "vault-state", state: "unlocked" });
  assert.deepEqual(states, ["locked", "unlocked"]);
});

test("a port the service worker dropped is reopened", () => {
  const { ports, port, states } = watching();
  port(0).drop();
  assert.equal(ports.length, 2);
  port(1).send({ kind: "vault-state", state: "not-open" });
  assert.deepEqual(states, ["not-open"]);
});

test("a locked watch holds one watch while locked and hears every state but locked", () => {
  const vault = new FakeVault();
  let changes = 0;
  const locked = new LockedWatch(vault.watch, () => {
    changes += 1;
  });
  locked.sync(true);
  locked.sync(true);
  vault.push("locked");
  vault.push("unlocked");
  vault.push("not-open");
  assert.equal(changes, 2);
  locked.sync(false);
  assert.equal(vault.watching(), false);
  vault.push("unlocked");
  assert.equal(changes, 2);
});

test("stopping disconnects the port and hears nothing more", () => {
  const { ports, port, states, stop } = watching();
  stop();
  assert.equal(port(0).disconnected, true);
  port(0).send({ kind: "vault-state", state: "unlocked" });
  port(0).drop();
  assert.deepEqual(states, []);
  assert.equal(ports.length, 1);
});
