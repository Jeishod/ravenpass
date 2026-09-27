import assert from "node:assert/strict";
import test from "node:test";
import {
  type BridgeMessage,
  isPortOffer,
  native,
  notAllowed,
  type PageAnswer,
  type PageMessage,
  portOffer,
  readBridgeMessage,
  readPageAnswer,
  readPageMessage,
} from "./page-channel.ts";
import type { GetOptions } from "./requests.ts";
import type { CreatedPasskey, SignedPasskey } from "./responses.ts";

const options: GetOptions = {
  rpId: "example.com",
  challenge: "AQI",
  timeout: null,
  allow: [],
  userVerification: "preferred",
  hints: [],
};

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
  userHandle: "",
};

test("the port offer is recognized by its tag alone", () => {
  assert.equal(isPortOffer(structuredClone(portOffer)), true);
  assert.equal(isPortOffer({ ...portOffer, extra: 1 }), true);
  for (const data of [
    null,
    "webauthn-port",
    {},
    { ravenpass: "other" },
    [portOffer],
  ]) {
    assert.equal(isPortOffer(data), false);
  }
});

test("each page message survives the channel whole, with only its own fields", () => {
  const messages: PageMessage[] = [
    {
      kind: "request",
      id: 1,
      request: { mode: "get", options, conditional: true },
    },
    { kind: "withdraw", id: 2 },
    { kind: "linked", id: 3 },
  ];
  for (const message of messages) {
    assert.deepEqual(readPageMessage(structuredClone(message)), message);
  }
  assert.deepEqual(readPageMessage({ kind: "withdraw", id: 2, linked: true }), {
    kind: "withdraw",
    id: 2,
  });
});

test("a page message with a malformed id, kind or request reads as nothing", () => {
  for (const data of [
    null,
    { kind: "linked" },
    { kind: "linked", id: undefined },
    { kind: "linked", id: "1" },
    { kind: "linked", id: 1.5 },
    { kind: "linked", id: Number.NaN },
    { kind: "linked", id: Number.POSITIVE_INFINITY },
    { kind: "linked", id: 2 ** 53 },
    { kind: "answer", id: 1 },
    { kind: "request", id: 1 },
    {
      kind: "request",
      id: 1,
      request: { mode: "get", options: { ...options, timeout: Number.NaN } },
    },
  ]) {
    assert.equal(readPageMessage(structuredClone(data)), null);
  }
});

test("each bridge message survives the channel whole, with only its own fields", () => {
  const messages: BridgeMessage[] = [
    { kind: "linked", id: 1, linked: false },
    { kind: "answer", id: 2, answer: native },
    { kind: "answer", id: 3, answer: notAllowed },
    { kind: "answer", id: 4, answer: { kind: "created", passkey: created } },
    { kind: "answer", id: 5, answer: { kind: "signed", passkey: signed } },
  ];
  for (const message of messages) {
    assert.deepEqual(readBridgeMessage(structuredClone(message)), message);
  }
  assert.deepEqual(
    readBridgeMessage({
      kind: "answer",
      id: 6,
      answer: { kind: "native", passkey: created },
      linked: true,
    }),
    { kind: "answer", id: 6, answer: native },
  );
});

test("a bridge message with a malformed field reads as nothing", () => {
  for (const data of [
    { kind: "linked", id: 1 },
    { kind: "linked", id: 1, linked: "true" },
    { kind: "linked", id: Number.NaN, linked: true },
    { kind: "request", id: 1 },
    { kind: "answer", id: 1, answer: { kind: "created" } },
    {
      kind: "answer",
      id: 1,
      answer: { kind: "signed", passkey: { ...signed, signature: undefined } },
    },
  ]) {
    assert.equal(readBridgeMessage(structuredClone(data)), null);
  }
});

test("an answer is read with only its own fields, and a refusal only with a known name", () => {
  const answers: PageAnswer[] = [
    native,
    notAllowed,
    { kind: "created", passkey: created },
  ];
  for (const answer of answers) {
    assert.deepEqual(readPageAnswer(structuredClone(answer)), answer);
  }
  assert.deepEqual(
    readPageAnswer({ ...notAllowed, stack: "Error: at page.js" }),
    notAllowed,
  );
  for (const value of [
    null,
    { kind: "Native" },
    { ...notAllowed, name: "AbortError" },
    { ...notAllowed, message: undefined },
    {
      kind: "created",
      passkey: { ...created, publicKeyAlgorithm: Number.NaN },
    },
  ]) {
    assert.equal(readPageAnswer(structuredClone(value)), null);
  }
});
