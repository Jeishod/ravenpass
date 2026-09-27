import assert from "node:assert/strict";
import test from "node:test";
import {
  alreadyRegistered,
  type BridgeMessage,
  type ChannelPort,
  native,
  notAllowed,
  type PageAnswer,
  type PageMessage,
} from "./page-channel.ts";
import {
  type PageCreateCall,
  PageCredentials,
  type PageGetCall,
} from "./page-credentials.ts";
import type { PageCreationOptions, PageRequestOptions } from "./requests.ts";
import type { CreatedPasskey, SignedPasskey } from "./responses.ts";

class PageCredential {}
class PageAttestation {}
class PageAssertion {}

const created: CreatedPasskey = {
  credentialId: "AQID",
  clientDataJSON: "e30",
  attestationObject: "oA",
  authenticatorData: "BAUG",
  publicKey: "MFkwEw",
  publicKeyAlgorithm: -7,
};

const signed: SignedPasskey = {
  credentialId: "AQID",
  clientDataJSON: "e30",
  authenticatorData: "BAUG",
  signature: "MEUCIQ",
  userHandle: "dXNlcg",
};

const publicKeyCreation: PageCreationOptions = {
  rp: { name: "Example" },
  user: { id: new Uint8Array([1]), name: "alex", displayName: "Alex" },
  challenge: new Uint8Array([1, 2]),
  pubKeyCredParams: [{ type: "public-key", alg: -7 }],
  extensions: { credProps: true },
};

const publicKeyRequest: PageRequestOptions = {
  challenge: new Uint8Array([1, 2]),
};

/** The isolated script's end of the port, recording what the page's script sends. */
class IsolatedEnd implements ChannelPort<PageMessage> {
  readonly sent: PageMessage[] = [];
  private listener: ((event: MessageEvent<unknown>) => void) | null = null;

  postMessage(message: PageMessage): void {
    this.sent.push(message);
  }

  addEventListener(
    _type: "message",
    listener: (event: MessageEvent<unknown>) => void,
  ): void {
    this.listener = listener;
  }

  start(): void {}

  deliver(message: BridgeMessage): void {
    this.listener?.({ data: structuredClone(message) } as MessageEvent);
  }

  answer(id: number, answer: PageAnswer): void {
    this.deliver({ kind: "answer", id, answer });
  }

  /** The last request sent, by its id. */
  lastRequest(): Extract<PageMessage, { kind: "request" }> {
    const request = [...this.sent]
      .reverse()
      .find((message) => message.kind === "request");
    assert.ok(request?.kind === "request");
    return request;
  }
}

/** Chrome's own WebAuthn: it records each call and settles as `settle` says, by default never. */
class ChromeWebAuthn {
  readonly calls: [string, unknown][] = [];
  conditional = true;
  settle: (
    options: PageGetCall | PageCreateCall | undefined,
  ) => Promise<Credential | null> = () => new Promise(() => {});

  create = (options?: PageCreateCall) => {
    this.calls.push(["create", options]);
    return this.settle(options);
  };

  get = (options?: PageGetCall) => {
    this.calls.push(["get", options]);
    return this.settle(options);
  };

  conditionalMediation = async () => this.conditional;
}

class PageException extends Error {
  constructor(message: string, name: string) {
    super(message);
    this.name = name;
  }
}

function page() {
  const chrome = new ChromeWebAuthn();
  const credentials = new PageCredentials({
    native: chrome,
    prototypes: {
      credential: PageCredential.prototype,
      attestation: PageAttestation.prototype,
      assertion: PageAssertion.prototype,
    },
    exception: (message, name) => new PageException(message, name),
  });
  const port = new IsolatedEnd();
  credentials.connect(port);
  return { chrome, credentials, port };
}

const tick = () => new Promise((resolve) => setTimeout(resolve, 0));

test("a creation goes to the isolated script as plain data and resolves with the page's credential", async () => {
  const { chrome, credentials, port } = page();

  const pending = credentials.create({ publicKey: publicKeyCreation });
  const { id, request } = port.lastRequest();
  assert.equal(request.mode, "create");
  assert.equal(request.mode === "create" && request.options.challenge, "AQI");
  port.answer(id, { kind: "created", passkey: created });

  const credential = (await pending) as unknown as {
    response: object;
    getClientExtensionResults(): object;
  };
  assert.ok(credential instanceof PageCredential);
  assert.ok(credential.response instanceof PageAttestation);
  assert.deepEqual(credential.getClientExtensionResults(), {
    credProps: { rk: true },
  });
  assert.deepEqual(chrome.calls, []);
});

test("a sign-in resolves with an assertion", async () => {
  const { credentials, port } = page();

  const pending = credentials.get({ publicKey: publicKeyRequest });
  const { id, request } = port.lastRequest();
  assert.deepEqual(request.mode === "get" && request.conditional, false);
  port.answer(id, { kind: "signed", passkey: signed });

  const credential = (await pending) as unknown as { response: object };
  assert.ok(credential.response instanceof PageAssertion);
});

test("an answer handing the request to Chrome calls Chrome with the page's own options", async () => {
  const { chrome, credentials, port } = page();
  const answer = { id: "chrome" } as unknown as Credential;
  chrome.settle = async () => answer;
  const signal = new AbortController().signal;
  const options = { publicKey: publicKeyRequest, signal };

  const pending = credentials.get(options);
  port.answer(port.lastRequest().id, native);

  assert.equal(await pending, answer);
  assert.deepEqual(chrome.calls, [["get", options]]);
  assert.equal(chrome.calls[0]?.[1], options);
});

test("a refusal rejects with its DOMException", async () => {
  const { credentials, port } = page();

  const pending = credentials.create({ publicKey: publicKeyCreation });
  port.answer(port.lastRequest().id, alreadyRegistered);

  await assert.rejects(pending, {
    name: "InvalidStateError",
    message: alreadyRegistered.message,
  });
});

test("calls without publicKey, a conditional creation, a silent sign-in and unreadable options go to Chrome at once", async () => {
  const { chrome, credentials, port } = page();
  chrome.settle = async () => null;
  const calls: [string, PageCreateCall | PageGetCall | undefined][] = [
    ["create", undefined],
    ["get", {}],
    ["create", { publicKey: publicKeyCreation, mediation: "conditional" }],
    ["get", { publicKey: publicKeyRequest, mediation: "silent" }],
    ["get", { publicKey: { challenge: "AQ" as unknown as BufferSource } }],
  ];

  for (const [kind, options] of calls) {
    await (kind === "create"
      ? credentials.create(options as PageCreateCall)
      : credentials.get(options as PageGetCall));
  }

  assert.deepEqual(chrome.calls, calls);
  assert.deepEqual(port.sent, []);
});

test("the page aborting rejects with its reason and withdraws the request", async () => {
  const { credentials, port } = page();
  const controller = new AbortController();

  const pending = credentials.get({
    publicKey: publicKeyRequest,
    signal: controller.signal,
  });
  const { id } = port.lastRequest();
  controller.abort();

  await assert.rejects(pending, { name: "AbortError" });
  assert.deepEqual(port.sent.at(-1), { kind: "withdraw", id });
  port.answer(id, { kind: "signed", passkey: signed });
});

test("a signal aborted before the call rejects without asking", async () => {
  const { credentials, port } = page();
  const reason = new Error("gone");

  await assert.rejects(
    credentials.create({
      publicKey: publicKeyCreation,
      signal: AbortSignal.abort(reason),
    }),
    reason,
  );
  assert.deepEqual(port.sent, []);
});

test("requests made before the port arrives wait for it", async () => {
  const chrome = new ChromeWebAuthn();
  const credentials = new PageCredentials({
    native: chrome,
    prototypes: {
      credential: PageCredential.prototype,
      attestation: PageAttestation.prototype,
      assertion: PageAssertion.prototype,
    },
    exception: (message, name) => new PageException(message, name),
  });
  const pending = credentials.get({ publicKey: publicKeyRequest });
  const port = new IsolatedEnd();

  assert.equal(credentials.connect(port), true);
  assert.equal(credentials.connect(new IsolatedEnd()), false);
  port.answer(port.lastRequest().id, { kind: "signed", passkey: signed });

  assert.ok(((await pending) as unknown as object) instanceof PageCredential);
});

test("a conditional sign-in runs in Chrome with the page's options and its own signal, and Ravenpass's passkey wins", async () => {
  const { chrome, credentials, port } = page();
  let chromeSignal: AbortSignal | undefined;
  chrome.settle = (options) => {
    chromeSignal = options?.signal;
    return new Promise((_resolve, reject) =>
      options?.signal?.addEventListener("abort", () =>
        reject(options.signal?.reason),
      ),
    );
  };
  const pageSignal = new AbortController().signal;

  const pending = credentials.get({
    publicKey: publicKeyRequest,
    mediation: "conditional",
    signal: pageSignal,
  });
  const { id, request } = port.lastRequest();
  assert.equal(request.mode === "get" && request.conditional, true);
  assert.equal(chrome.calls.length, 1);
  const [, options] = chrome.calls[0] as [string, PageGetCall];
  assert.equal(options.mediation, "conditional");
  assert.equal(options.publicKey, publicKeyRequest);
  assert.notEqual(options.signal, pageSignal);

  port.answer(id, native);
  await tick();
  assert.equal(chromeSignal?.aborted, false);

  port.answer(id, { kind: "signed", passkey: signed });
  assert.ok(((await pending) as unknown as object) instanceof PageCredential);
  assert.equal(chromeSignal?.aborted, true);
  assert.equal(port.sent.filter(({ kind }) => kind === "withdraw").length, 0);
});

test("Chrome answering a conditional sign-in first wins, and Ravenpass's request is withdrawn", async () => {
  const { chrome, credentials, port } = page();
  const answer = { id: "chrome" } as unknown as Credential;
  let settle: (credential: Credential) => void = () => {};
  chrome.settle = () => new Promise((resolve) => (settle = resolve));

  const pending = credentials.get({
    publicKey: publicKeyRequest,
    mediation: "conditional",
  });
  const { id } = port.lastRequest();
  settle(answer);

  assert.equal(await pending, answer);
  assert.deepEqual(port.sent.at(-1), { kind: "withdraw", id });
  port.answer(id, { kind: "signed", passkey: signed });
});

test("Chrome refusing a conditional sign-in rejects it and withdraws Ravenpass's request", async () => {
  const { chrome, credentials, port } = page();
  chrome.settle = async () => {
    throw new PageException(notAllowed.message, "NotAllowedError");
  };

  await assert.rejects(
    credentials.get({ publicKey: publicKeyRequest, mediation: "conditional" }),
    { name: "NotAllowedError" },
  );
  assert.equal(port.sent.at(-1)?.kind, "withdraw");
});

test("the page aborting a conditional sign-in aborts Chrome's and withdraws Ravenpass's", async () => {
  const { chrome, credentials, port } = page();
  let chromeSignal: AbortSignal | undefined;
  chrome.settle = (options) => {
    chromeSignal = options?.signal;
    return new Promise(() => {});
  };
  const controller = new AbortController();

  const pending = credentials.get({
    publicKey: publicKeyRequest,
    mediation: "conditional",
    signal: controller.signal,
  });
  const { id } = port.lastRequest();
  controller.abort();

  await assert.rejects(pending, { name: "AbortError" });
  assert.equal(chromeSignal?.aborted, true);
  assert.deepEqual(port.sent.at(-1), { kind: "withdraw", id });
});

test("conditional mediation is available when Chrome's is, or when Ravenpass is linked", async () => {
  const { chrome, credentials, port } = page();
  assert.equal(await credentials.isConditionalMediationAvailable(), true);
  assert.equal(port.sent.length, 0);

  chrome.conditional = false;
  for (const linked of [true, false]) {
    const available = credentials.isConditionalMediationAvailable();
    await tick();
    const asked = port.sent.at(-1);
    assert.equal(asked?.kind, "linked");
    port.deliver({ kind: "linked", id: asked.id, linked });
    assert.equal(await available, linked);
  }
});
