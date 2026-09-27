import assert from "node:assert/strict";
import { afterEach, beforeEach, mock, test } from "node:test";
import type { Answers, PasskeyPageRequest } from "../messages.ts";
import {
  alreadyPending,
  type BridgeMessage,
  type ChannelPort,
  native,
  notAllowed,
  type PageMessage,
} from "../passkeys/page-channel.ts";
import type { GetOptions, PageRequest } from "../passkeys/requests.ts";
import type { SignedPasskey } from "../passkeys/responses.ts";
import {
  type BridgeAsk,
  PasskeyBridge,
  type RuntimeMessages,
} from "./passkey-bridge.ts";

const extensionId = "cceiadaelnccfbakmhcleifjfilkakag";

const options: GetOptions = {
  rpId: null,
  challenge: "AQI",
  timeout: 30_000,
  allow: [],
  userVerification: "preferred",
  hints: [],
};

const modal: PageRequest = { mode: "get", options, conditional: false };
const conditional: PageRequest = { mode: "get", options, conditional: true };

const signed: SignedPasskey = {
  credentialId: "AQID",
  clientDataJSON: "e30",
  authenticatorData: "BAUG",
  signature: "MEUCIQ",
  userHandle: "dXNlcg",
};

beforeEach(() => {
  mock.timers.enable({ apis: ["setTimeout"] });
});

afterEach(() => {
  mock.timers.reset();
});

function settle(): Promise<void> {
  return new Promise((resolve) => setImmediate(resolve));
}

class MainEnd implements ChannelPort<BridgeMessage> {
  readonly received: BridgeMessage[] = [];
  private listener: ((event: MessageEvent<unknown>) => void) | null = null;

  postMessage(message: BridgeMessage): void {
    this.received.push(message);
  }

  addEventListener(
    _type: "message",
    listener: (event: MessageEvent<unknown>) => void,
  ): void {
    this.listener = listener;
  }

  start(): void {}

  send(message: PageMessage | unknown): void {
    this.listener?.({ data: structuredClone(message) } as MessageEvent);
  }
}

/** `chrome.runtime.onMessage` of the isolated world. */
class RuntimeEvents implements RuntimeMessages {
  private readonly listeners = new Set<
    Parameters<RuntimeMessages["addListener"]>[0]
  >();

  addListener(listener: Parameters<RuntimeMessages["addListener"]>[0]): void {
    this.listeners.add(listener);
  }

  removeListener(
    listener: Parameters<RuntimeMessages["addListener"]>[0],
  ): void {
    this.listeners.delete(listener);
  }

  deliver(message: unknown, sender = { id: extensionId }): void {
    for (const listener of this.listeners) listener(message, sender);
  }
}

/** A bridge whose service worker answers a request with `reply`, rejecting when it is null. */
function bridge(reply: Answers["passkey-request"] | null = { answer: null }) {
  const page = new MainEnd();
  const runtime = new RuntimeEvents();
  const asked: PasskeyPageRequest[] = [];
  const ask = (async (request: PasskeyPageRequest) => {
    asked.push(request);
    switch (request.kind) {
      case "passkey-request":
        if (!reply) throw new Error("The service worker is gone.");
        return reply;
      case "passkey-withdraw":
        return { ok: true };
      case "passkey-linked":
        return { linked: true };
    }
  }) as BridgeAsk;
  const passkeys = new PasskeyBridge({
    port: page,
    ask,
    messages: runtime,
    extensionId,
  });
  passkeys.start();
  return { page, runtime, asked, passkeys };
}

test("a request goes to the service worker, whose answer goes back to the page", async () => {
  const { page, asked } = bridge({ answer: native });

  page.send({ kind: "request", id: 4, request: modal });
  await settle();

  assert.deepEqual(asked, [{ kind: "passkey-request", id: 4, request: modal }]);
  assert.deepEqual(page.received, [{ kind: "answer", id: 4, answer: native }]);
});

test("a waiting request takes the answer the service worker sends its document, once", async () => {
  const { page, runtime } = bridge();
  page.send({ kind: "request", id: 4, request: modal });
  await settle();
  assert.deepEqual(page.received, []);

  const answer = { kind: "signed", passkey: signed };
  runtime.deliver({ kind: "passkey-answer", id: 4, answer }, { id: "other" });
  runtime.deliver({ kind: "passkey-answer", id: 5, answer });
  runtime.deliver({
    kind: "passkey-answer",
    id: 4,
    answer: { kind: "signed" },
  });
  assert.deepEqual(page.received, []);

  runtime.deliver({ kind: "passkey-answer", id: 4, answer });
  runtime.deliver({ kind: "passkey-answer", id: 4, answer: alreadyPending });
  assert.deepEqual(page.received, [{ kind: "answer", id: 4, answer }]);
});

test("a modal request the page's timeout ends is refused and withdrawn, and a late answer is dropped", async () => {
  const { page, runtime, asked } = bridge();
  page.send({ kind: "request", id: 4, request: modal });
  await settle();

  mock.timers.tick(29_999);
  assert.deepEqual(page.received, []);
  mock.timers.tick(1);
  await settle();

  assert.deepEqual(page.received, [
    { kind: "answer", id: 4, answer: notAllowed },
  ]);
  assert.deepEqual(asked.at(-1), { kind: "passkey-withdraw", id: 4 });
  runtime.deliver({
    kind: "passkey-answer",
    id: 4,
    answer: { kind: "signed", passkey: signed },
  });
  assert.equal(page.received.length, 1);
});

test("a conditional request waits as long as the document", async () => {
  const { page, asked } = bridge();
  page.send({ kind: "request", id: 4, request: conditional });
  await settle();

  mock.timers.tick(3_600_000);
  await settle();

  assert.deepEqual(page.received, []);
  assert.deepEqual(asked, [
    { kind: "passkey-request", id: 4, request: conditional },
  ]);
});

test("a request the page withdraws is withdrawn from the service worker and takes no answer", async () => {
  const { page, runtime, asked } = bridge();
  page.send({ kind: "request", id: 4, request: modal });
  await settle();

  page.send({ kind: "withdraw", id: 4 });
  page.send({ kind: "withdraw", id: 9 });
  await settle();
  mock.timers.tick(30_000);
  runtime.deliver({
    kind: "passkey-answer",
    id: 4,
    answer: { kind: "signed", passkey: signed },
  });

  assert.deepEqual(asked.at(-1), { kind: "passkey-withdraw", id: 4 });
  assert.equal(
    asked.filter(({ kind }) => kind === "passkey-withdraw").length,
    1,
  );
  assert.deepEqual(page.received, []);
});

test("a request the service worker cannot take goes to Chrome", async () => {
  const { page } = bridge(null);
  page.send({ kind: "request", id: 4, request: modal });
  await settle();
  assert.deepEqual(page.received, [{ kind: "answer", id: 4, answer: native }]);
});

test("a malformed message from the page asks nothing", async () => {
  const { page, asked } = bridge();
  for (const message of [
    null,
    { kind: "request", id: "4", request: modal },
    { kind: "request", id: 4, request: { ...modal, mode: "delete" } },
    { kind: "request", id: 4.5, request: modal },
    { kind: "answer", id: 4, answer: native },
  ]) {
    page.send(message);
  }
  await settle();
  assert.deepEqual(asked, []);
});

test("the page asking whether Ravenpass is linked is answered", async () => {
  const { page, asked } = bridge();
  page.send({ kind: "linked", id: 2 });
  await settle();
  assert.deepEqual(asked, [{ kind: "passkey-linked" }]);
  assert.deepEqual(page.received, [{ kind: "linked", id: 2, linked: true }]);
});
