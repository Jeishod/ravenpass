import { base64urlnopad } from "@scure/base";
import * as v from "valibot";

function decodes(text: string): boolean {
  try {
    base64urlnopad.decode(text);
    return true;
  } catch {
    return false;
  }
}

export const base64url = v.pipe(v.string(), v.check(decodes));

const optionalText = v.nullable(v.string());
const finiteNumber = v.pipe(v.number(), v.finite());
const safeInteger = v.pipe(v.number(), v.safeInteger());
const texts = v.pipe(v.array(v.string()), v.readonly());
const base64urls = v.pipe(v.array(base64url), v.readonly());

export const passkeyUser = v.pipe(
  v.object({ id: base64url, name: v.string(), displayName: v.string() }),
  v.readonly(),
);

export const createOptions = v.pipe(
  v.object({
    rpId: optionalText,
    rpName: v.string(),
    user: passkeyUser,
    challenge: base64url,
    // The desktop app decodes COSE algorithm numbers as integers and refuses a request carrying any other number.
    algorithms: v.pipe(v.array(safeInteger), v.readonly()),
    timeout: v.nullable(finiteNumber),
    exclude: base64urls,
    attachment: optionalText,
    userVerification: v.string(),
    hints: texts,
    credProps: v.boolean(),
  }),
  v.readonly(),
);

export const getOptions = v.pipe(
  v.object({
    rpId: optionalText,
    challenge: base64url,
    timeout: v.nullable(finiteNumber),
    allow: base64urls,
    userVerification: v.string(),
    hints: texts,
  }),
  v.readonly(),
);

export const pageRequest = v.pipe(
  v.variant("mode", [
    v.object({ mode: v.literal("create"), options: createOptions }),
    v.object({
      mode: v.literal("get"),
      options: getOptions,
      conditional: v.boolean(),
    }),
  ]),
  v.readonly(),
);

export const createdPasskey = v.pipe(
  v.object({
    credentialId: base64url,
    clientDataJSON: base64url,
    attestationObject: base64url,
    authenticatorData: base64url,
    // SubjectPublicKeyInfo DER, and its COSE algorithm number.
    publicKey: base64url,
    publicKeyAlgorithm: safeInteger,
  }),
  v.readonly(),
);

export const signedPasskey = v.pipe(
  v.object({
    credentialId: base64url,
    clientDataJSON: base64url,
    authenticatorData: base64url,
    signature: base64url,
    userHandle: base64url,
  }),
  v.readonly(),
);

export const pageAnswer = v.pipe(
  v.variant("kind", [
    v.object({ kind: v.literal("created"), passkey: createdPasskey }),
    v.object({ kind: v.literal("signed"), passkey: signedPasskey }),
    v.object({ kind: v.literal("native") }),
    v.object({
      kind: v.literal("refused"),
      name: v.picklist(["NotAllowedError", "InvalidStateError"]),
      message: v.string(),
    }),
  ]),
  v.readonly(),
);

export const pageMessage = v.pipe(
  v.variant("kind", [
    v.object({
      kind: v.literal("request"),
      id: safeInteger,
      request: pageRequest,
    }),
    v.object({ kind: v.literal("withdraw"), id: safeInteger }),
    v.object({ kind: v.literal("linked"), id: safeInteger }),
  ]),
  v.readonly(),
);

export const bridgeMessage = v.pipe(
  v.variant("kind", [
    v.object({
      kind: v.literal("answer"),
      id: safeInteger,
      answer: pageAnswer,
    }),
    v.object({
      kind: v.literal("linked"),
      id: safeInteger,
      linked: v.boolean(),
    }),
  ]),
  v.readonly(),
);

export const portOfferMessage = v.object({
  ravenpass: v.literal("webauthn-port"),
});

export function read<Schema extends v.GenericSchema>(
  schema: Schema,
  value: unknown,
): v.InferOutput<Schema> | null {
  const parsed = v.safeParse(schema, value);
  return parsed.success ? parsed.output : null;
}
