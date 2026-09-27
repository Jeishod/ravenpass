import assert from "node:assert/strict";
import test from "node:test";
import sharedCredentials from "../../../../packages/authenticator/testdata/credentials.json" with {
  type: "json",
};
import {
  type CreatedPasskey,
  createdCredential,
  readCreatedPasskey,
  readSignedPasskey,
  type SignedPasskey,
  signedCredential,
  type WebAuthnPrototypes,
} from "./responses.ts";

class PageCredential {}
class PageAttestation {}
class PageAssertion {}

const prototypes: WebAuthnPrototypes = {
  credential: PageCredential.prototype,
  attestation: PageAttestation.prototype,
  assertion: PageAssertion.prototype,
};

const created: CreatedPasskey = {
  credentialId: "AQID",
  clientDataJSON: "eyJ0eXBlIjoid2ViYXV0aG4uY3JlYXRlIn0",
  attestationObject: "o2NmbXRkbm9uZQ",
  authenticatorData: "BAUG",
  publicKey: "MFkwEw",
  publicKeyAlgorithm: -7,
};

const signed: SignedPasskey = {
  credentialId: "AQID",
  clientDataJSON: "eyJ0eXBlIjoid2ViYXV0aG4uZ2V0In0",
  authenticatorData: "BAUG",
  signature: "MEUCIQ",
  userHandle: "dXNlcg",
};

interface Credential {
  readonly id: string;
  readonly rawId: ArrayBuffer;
  readonly type: string;
  readonly authenticatorAttachment: string;
  readonly response: Record<string, unknown> & {
    readonly clientDataJSON: ArrayBuffer;
    getAuthenticatorData(): ArrayBuffer;
    getPublicKey(): ArrayBuffer;
    getPublicKeyAlgorithm(): number;
    getTransports(): string[];
  };
  getClientExtensionResults(): object;
  toJSON(): object;
}

const bytesOf = (buffer: unknown) => [...new Uint8Array(buffer as ArrayBuffer)];

test("a created passkey is a PublicKeyCredential of the page with an attestation response", () => {
  const credential = createdCredential(
    created,
    false,
    prototypes,
  ) as Credential;

  assert.ok(credential instanceof PageCredential);
  assert.ok(credential.response instanceof PageAttestation);
  assert.equal(credential.id, "AQID");
  assert.deepEqual(bytesOf(credential.rawId), [1, 2, 3]);
  assert.equal(credential.type, "public-key");
  assert.equal(credential.authenticatorAttachment, "platform");
  assert.equal(
    new TextDecoder().decode(credential.response.clientDataJSON),
    '{"type":"webauthn.create"}',
  );
  assert.deepEqual(
    bytesOf(credential.response.attestationObject),
    [0xa3, 0x63, 0x66, 0x6d, 0x74, 0x64, 0x6e, 0x6f, 0x6e, 0x65],
  );
  assert.deepEqual(
    bytesOf(credential.response.getAuthenticatorData()),
    [4, 5, 6],
  );
  assert.deepEqual(
    bytesOf(credential.response.getPublicKey()),
    [0x30, 0x59, 0x30, 0x13],
  );
  assert.equal(credential.response.getPublicKeyAlgorithm(), -7);
  assert.deepEqual(credential.response.getTransports(), ["hybrid", "internal"]);
  assert.deepEqual(credential.getClientExtensionResults(), {});
});

test("a created passkey answers credProps when the page asked for it", () => {
  const credential = createdCredential(created, true, prototypes) as Credential;
  assert.deepEqual(credential.getClientExtensionResults(), {
    credProps: { rk: true },
  });
});

test("a created passkey's JSON is WebAuthn's registration response JSON", () => {
  const credential = createdCredential(created, true, prototypes) as Credential;
  const json = {
    id: "AQID",
    rawId: "AQID",
    response: {
      clientDataJSON: created.clientDataJSON,
      authenticatorData: "BAUG",
      transports: ["hybrid", "internal"],
      publicKey: "MFkwEw",
      publicKeyAlgorithm: -7,
      attestationObject: created.attestationObject,
    },
    authenticatorAttachment: "platform",
    clientExtensionResults: { credProps: { rk: true } },
    type: "public-key",
  };
  assert.deepEqual(credential.toJSON(), json);
  assert.deepEqual(JSON.parse(JSON.stringify(credential)), json);
});

test("a credential's JSON is what the table shared with Android says", () => {
  assert.ok(sharedCredentials.registrations.length > 0);
  assert.ok(sharedCredentials.authentications.length > 0);
  for (const {
    name,
    passkey,
    credProps,
    json,
  } of sharedCredentials.registrations) {
    const credential = createdCredential(
      passkey,
      credProps,
      prototypes,
    ) as Credential;
    assert.deepEqual(credential.toJSON(), json, name);
  }
  for (const { name, passkey, json } of sharedCredentials.authentications) {
    const credential = signedCredential(passkey, prototypes) as Credential;
    assert.deepEqual(credential.toJSON(), json, name);
  }
});

test("a signed sign-in is a PublicKeyCredential of the page with an assertion response and its JSON", () => {
  const credential = signedCredential(signed, prototypes) as Credential;

  assert.ok(credential instanceof PageCredential);
  assert.ok(credential.response instanceof PageAssertion);
  assert.deepEqual(bytesOf(credential.response.authenticatorData), [4, 5, 6]);
  assert.deepEqual(
    bytesOf(credential.response.signature),
    [0x30, 0x45, 0x02, 0x21],
  );
  assert.equal(
    new TextDecoder().decode(credential.response.userHandle as ArrayBuffer),
    "user",
  );
  assert.deepEqual(credential.getClientExtensionResults(), {});
  assert.deepEqual(credential.toJSON(), {
    id: "AQID",
    rawId: "AQID",
    response: {
      clientDataJSON: signed.clientDataJSON,
      authenticatorData: "BAUG",
      signature: "MEUCIQ",
      userHandle: "dXNlcg",
    },
    authenticatorAttachment: "platform",
    clientExtensionResults: {},
    type: "public-key",
  });
});

test("a passkey without a user handle answers null and leaves it out of its JSON", () => {
  const credential = signedCredential(
    { ...signed, userHandle: "" },
    prototypes,
  ) as Credential;
  assert.equal(credential.response.userHandle, null);
  assert.equal(
    "userHandle" in (credential.toJSON() as { response: object }).response,
    false,
  );
});

test("every buffer is the credential's own, and every method answers a fresh copy", () => {
  const credential = createdCredential(
    created,
    false,
    prototypes,
  ) as Credential;

  new Uint8Array(credential.response.getPublicKey()).fill(0);
  new Uint8Array(credential.response.getAuthenticatorData()).fill(0);
  new Uint8Array(credential.rawId).fill(0);
  credential.response.getTransports().pop();

  assert.deepEqual(
    bytesOf(credential.response.getPublicKey()),
    [0x30, 0x59, 0x30, 0x13],
  );
  assert.deepEqual(
    bytesOf(credential.response.getAuthenticatorData()),
    [4, 5, 6],
  );
  assert.deepEqual(credential.response.getTransports(), ["hybrid", "internal"]);
  assert.equal(credential.id, "AQID");
  assert.equal((credential.toJSON() as { rawId: string }).rawId, "AQID");
  assert.notEqual(credential.rawId, credential.response.clientDataJSON);
});

test("a credential's fields are read-only and, like native attributes, do not enumerate", () => {
  const credential = createdCredential(
    created,
    false,
    prototypes,
  ) as Credential;
  assert.throws(() => {
    (credential as { id: string }).id = "forged";
  }, TypeError);
  assert.deepEqual(Object.keys(credential), []);
});

test("only a well-formed passkey is read from a message", () => {
  assert.deepEqual(readCreatedPasskey(structuredClone(created)), created);
  assert.deepEqual(readSignedPasskey(structuredClone(signed)), signed);
  for (const value of [
    null,
    { ...created, publicKeyAlgorithm: "-7" },
    { ...created, publicKey: "not base64url!" },
    { ...created, credentialId: undefined },
  ]) {
    assert.equal(readCreatedPasskey(value), null);
  }
  for (const value of [
    { ...signed, signature: 1 },
    { ...signed, userHandle: undefined },
  ]) {
    assert.equal(readSignedPasskey(value), null);
  }
});

test("a passkey read from a message keeps only its own fields", () => {
  assert.deepEqual(
    readCreatedPasskey({ ...created, credential: "c1" }),
    created,
  );
  assert.deepEqual(readSignedPasskey({ ...signed, extra: 1 }), signed);
});

test("a created passkey's algorithm must be a safe integer", () => {
  for (const publicKeyAlgorithm of [
    Number.NaN,
    Number.POSITIVE_INFINITY,
    -7.5,
    2 ** 53,
  ]) {
    assert.equal(
      readCreatedPasskey(structuredClone({ ...created, publicKeyAlgorithm })),
      null,
    );
  }
  const { publicKeyAlgorithm: _algorithm, ...withoutAlgorithm } = created;
  assert.equal(readCreatedPasskey(withoutAlgorithm), null);
});
