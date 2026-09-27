import { base64urlnopad } from "@scure/base";
import type * as v from "valibot";
import { createdPasskey, read, signedPasskey } from "./page-wire.ts";
import type { Base64URL } from "./requests.ts";

export type CreatedPasskey = v.InferOutput<typeof createdPasskey>;

export type SignedPasskey = v.InferOutput<typeof signedPasskey>;

// The page's own WebAuthn prototypes, so `instanceof` holds in the page.
export interface WebAuthnPrototypes {
  readonly credential: object;
  readonly attestation: object;
  readonly assertion: object;
}

// Sorted as WebAuthn §5.2.1.1 requires.
const transports = ["hybrid", "internal"] as const;

const attachment = "platform";

const type = "public-key";

export function readCreatedPasskey(value: unknown): CreatedPasskey | null {
  return read(createdPasskey, value);
}

export function readSignedPasskey(value: unknown): SignedPasskey | null {
  return read(signedPasskey, value);
}

// Every field is an own property: native prototype getters throw on objects Chrome did not create.
export function createdCredential(
  created: CreatedPasskey,
  credProps: boolean,
  prototypes: WebAuthnPrototypes,
): object {
  const rawId = bytes(created.credentialId);
  const clientDataJSON = bytes(created.clientDataJSON);
  const attestationObject = bytes(created.attestationObject);
  const authenticatorData = bytes(created.authenticatorData);
  const publicKey = bytes(created.publicKey);
  const algorithm = created.publicKeyAlgorithm;
  const extensions = () => (credProps ? { credProps: { rk: true } } : {});
  const response = Object.create(prototypes.attestation, {
    clientDataJSON: constant(buffer(clientDataJSON)),
    attestationObject: constant(buffer(attestationObject)),
    getAuthenticatorData: constant(() => buffer(authenticatorData)),
    getPublicKey: constant(() => buffer(publicKey)),
    getPublicKeyAlgorithm: constant(() => algorithm),
    getTransports: constant(() => [...transports]),
  });
  const id = encode(rawId);
  return credential(prototypes, rawId, response, extensions, () => ({
    id,
    rawId: id,
    response: {
      clientDataJSON: encode(clientDataJSON),
      authenticatorData: encode(authenticatorData),
      transports: [...transports],
      publicKey: encode(publicKey),
      publicKeyAlgorithm: algorithm,
      attestationObject: encode(attestationObject),
    },
    authenticatorAttachment: attachment,
    clientExtensionResults: extensions(),
    type,
  }));
}

export function signedCredential(
  signed: SignedPasskey,
  prototypes: WebAuthnPrototypes,
): object {
  const rawId = bytes(signed.credentialId);
  const clientDataJSON = bytes(signed.clientDataJSON);
  const authenticatorData = bytes(signed.authenticatorData);
  const signature = bytes(signed.signature);
  const userHandle = bytes(signed.userHandle);
  const extensions = () => ({});
  const response = Object.create(prototypes.assertion, {
    clientDataJSON: constant(buffer(clientDataJSON)),
    authenticatorData: constant(buffer(authenticatorData)),
    signature: constant(buffer(signature)),
    userHandle: constant(userHandle.length ? buffer(userHandle) : null),
  });
  const id = encode(rawId);
  return credential(prototypes, rawId, response, extensions, () => ({
    id,
    rawId: id,
    response: {
      clientDataJSON: encode(clientDataJSON),
      authenticatorData: encode(authenticatorData),
      signature: encode(signature),
      ...(userHandle.length ? { userHandle: encode(userHandle) } : {}),
    },
    authenticatorAttachment: attachment,
    clientExtensionResults: extensions(),
    type,
  }));
}

function credential(
  prototypes: WebAuthnPrototypes,
  rawId: Uint8Array,
  response: object,
  extensions: () => object,
  json: () => object,
): object {
  return Object.create(prototypes.credential, {
    id: constant(encode(rawId)),
    rawId: constant(buffer(rawId)),
    type: constant(type),
    authenticatorAttachment: constant(attachment),
    response: constant(response),
    getClientExtensionResults: constant(extensions),
    toJSON: constant(json),
  });
}

// Non-enumerable, like native attributes.
function constant(value: unknown): PropertyDescriptor {
  return { value, writable: false, enumerable: false, configurable: false };
}

function bytes(text: Base64URL): Uint8Array {
  return base64urlnopad.decode(text);
}

function encode(data: Uint8Array): Base64URL {
  return base64urlnopad.encode(data);
}

function buffer(data: Uint8Array): ArrayBuffer {
  return data.slice().buffer;
}
