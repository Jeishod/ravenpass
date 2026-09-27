import assert from "node:assert/strict";
import test from "node:test";
import type {
  ExtensionLinking,
  ExtensionLinkOffer,
  ExtensionLinks,
  LinkedExtension,
  SignInStyleSetting,
} from "../vault-api.ts";
import { ExtensionLinker, type LinkEvent } from "./linker.ts";

interface Deferred<Value> {
  promise: Promise<Value>;
  resolve: (value: Value) => void;
  reject: (reason: unknown) => void;
}

function deferred<Value>(): Deferred<Value> {
  let resolve: (value: Value) => void = () => {};
  let reject: (reason: unknown) => void = () => {};
  const promise = new Promise<Value>((onResolve, onReject) => {
    resolve = onResolve;
    reject = onReject;
  });
  return { promise, resolve, reject };
}

interface Wait {
  answer: Deferred<LinkedExtension>;
  signal: AbortSignal;
}

class ScriptedLinking implements ExtensionLinking {
  readonly calls: string[] = [];
  private readonly begins: Deferred<ExtensionLinkOffer>[] = [];
  private readonly waits: Wait[] = [];

  begin(index: number): Deferred<ExtensionLinkOffer> {
    const begin = this.begins[index];
    assert.ok(begin, `no key ${index} was asked for`);
    return begin;
  }

  wait(index: number): Wait {
    const wait = this.waits[index];
    assert.ok(wait, `no wait ${index} started`);
    return wait;
  }

  extensionLinks(): Promise<ExtensionLinks> {
    return Promise.reject(new Error("The linker never lists extensions."));
  }

  beginExtensionLink(): Promise<ExtensionLinkOffer> {
    this.calls.push("begin");
    const answer = deferred<ExtensionLinkOffer>();
    this.begins.push(answer);
    return answer.promise;
  }

  awaitExtensionLink(signal: AbortSignal): Promise<LinkedExtension> {
    this.calls.push("await");
    const answer = deferred<LinkedExtension>();
    signal.addEventListener("abort", () =>
      answer.reject(new Error("The call was canceled.")),
    );
    this.waits.push({ answer, signal });
    return answer.promise;
  }

  cancelExtensionLink(): Promise<void> {
    this.calls.push("cancel");
    return Promise.resolve();
  }

  copyExtensionLinkKey(): Promise<void> {
    return Promise.reject(new Error("The linker never copies a key."));
  }

  unlinkExtension(): Promise<void> {
    return Promise.reject(new Error("The linker never unlinks."));
  }

  renameExtension(): Promise<void> {
    return Promise.reject(new Error("The linker never renames."));
  }

  confirmExtensionFills(): Promise<boolean> {
    return Promise.reject(
      new Error("The linker never reads the fill confirmation."),
    );
  }

  setConfirmExtensionFills(): Promise<void> {
    return Promise.reject(
      new Error("The linker never sets the fill confirmation."),
    );
  }

  signInStyle(): Promise<SignInStyleSetting> {
    return Promise.reject(
      new Error("The linker never reads the sign-in style."),
    );
  }

  setSignInStyle(): Promise<void> {
    return Promise.reject(
      new Error("The linker never sets the sign-in style."),
    );
  }
}

const offerA: ExtensionLinkOffer = { key: "key-a", expiresAt: 1_000 };
const offerB: ExtensionLinkOffer = { key: "key-b", expiresAt: 2_000 };
const chrome: LinkedExtension = {
  id: "ext-1",
  name: "Chrome · macOS",
  linkedAt: 500,
};

function settle() {
  return new Promise((resolve) => setTimeout(resolve, 0));
}

function start(host: ScriptedLinking) {
  const events: LinkEvent[] = [];
  const stop = new ExtensionLinker(host).offer((event) => events.push(event));
  return { events, stop };
}

test("an offer reports its key and the extension that links with it", async () => {
  const host = new ScriptedLinking();
  const { events } = start(host);
  await settle();
  host.begin(0).resolve(offerA);
  await settle();
  assert.deepEqual(host.calls, ["begin", "await"]);
  assert.deepEqual(events, [{ kind: "offered", offer: offerA }]);
  host.wait(0).answer.resolve(chrome);
  await settle();
  assert.deepEqual(events, [
    { kind: "offered", offer: offerA },
    { kind: "linked", extension: chrome },
  ]);
});

test("an expired key is reported as expired rather than as a failure", async () => {
  const host = new ScriptedLinking();
  const { events } = start(host);
  await settle();
  host.begin(0).resolve(offerA);
  await settle();
  host.wait(0).answer.reject(new Error("ravenpass:link-expired"));
  await settle();
  assert.deepEqual(events.at(-1), { kind: "expired" });
});

test("a key that cannot be created is reported with its cause", async () => {
  const host = new ScriptedLinking();
  const { events } = start(host);
  await settle();
  const cause = new Error("ravenpass:link-unavailable");
  host.begin(0).reject(cause);
  await settle();
  assert.deepEqual(events, [{ kind: "failed", cause }]);
  assert.deepEqual(host.calls, ["begin"]);
});

test("an offer stopped at once asks for no key", async () => {
  const host = new ScriptedLinking();
  const { events, stop } = start(host);
  stop();
  await settle();
  assert.deepEqual(host.calls, []);
  assert.deepEqual(events, []);
});

test("an offer stopped while its key is created cancels that key", async () => {
  const host = new ScriptedLinking();
  const { events, stop } = start(host);
  await settle();
  stop();
  host.begin(0).resolve(offerA);
  await settle();
  assert.deepEqual(host.calls, ["begin", "cancel"]);
  assert.deepEqual(events, []);
});

test("stopping a waiting offer aborts the wait and reports nothing more", async () => {
  const host = new ScriptedLinking();
  const { events, stop } = start(host);
  await settle();
  host.begin(0).resolve(offerA);
  await settle();
  stop();
  await settle();
  assert.ok(host.wait(0).signal.aborted);
  assert.deepEqual(events, [{ kind: "offered", offer: offerA }]);
});

test("a new offer creates its key only once the key before it exists", async () => {
  const host = new ScriptedLinking();
  const linker = new ExtensionLinker(host);
  const first: LinkEvent[] = [];
  const second: LinkEvent[] = [];
  const stopFirst = linker.offer((event) => first.push(event));
  await settle();
  stopFirst();
  linker.offer((event) => second.push(event));
  await settle();
  assert.deepEqual(host.calls, ["begin"]);
  host.begin(0).resolve(offerA);
  await settle();
  assert.deepEqual(host.calls, ["begin", "cancel", "begin"]);
  host.begin(1).resolve(offerB);
  await settle();
  assert.deepEqual(host.calls, ["begin", "cancel", "begin", "await"]);
  assert.deepEqual(first, []);
  assert.deepEqual(second, [{ kind: "offered", offer: offerB }]);
});
