import { base64urlnopad } from "@scure/base";
import type * as v from "valibot";
import {
  type createOptions,
  type getOptions,
  pageRequest,
  type passkeyUser,
  read,
} from "./page-wire.ts";

// COSE algorithm number of ES256, the one algorithm Ravenpass's passkeys sign with.
const es256 = -7;

const defaultTimeoutMs = 5 * 60_000;
// Chrome clamps a page's timeout to these bounds.
const minimumTimeoutMs = 10_000;
const maximumTimeoutMs = 10 * 60_000;

/** Unpadded base64url. */
export type Base64URL = string;

export type PasskeyUser = v.InferOutput<typeof passkeyUser>;

export type CreateOptions = v.InferOutput<typeof createOptions>;

export type GetOptions = v.InferOutput<typeof getOptions>;

export type PageRequest = v.InferOutput<typeof pageRequest>;

// TypeScript's DOM library omits the WebAuthn Level 3 `hints`.
export type PageCreationOptions = PublicKeyCredentialCreationOptions & {
  readonly hints?: Iterable<string>;
};

export type PageRequestOptions = PublicKeyCredentialRequestOptions & {
  readonly hints?: Iterable<string>;
};

/** Converts as WebIDL does; throws a TypeError for options WebAuthn refuses. */
export function createOptionsOf(options: PageCreationOptions): CreateOptions {
  const selection = options.authenticatorSelection;
  return {
    rpId: optionalText(options.rp.id),
    rpName: text(options.rp.name),
    user: {
      id: encoded(options.user.id),
      name: text(options.user.name),
      displayName: text(options.user.displayName),
    },
    challenge: encoded(options.challenge),
    algorithms: Array.from(options.pubKeyCredParams)
      .filter(({ type }) => type === "public-key")
      .map(({ alg }) => Number(alg)),
    timeout: timeoutOf(options.timeout),
    exclude: descriptorIds(options.excludeCredentials),
    attachment: optionalText(selection?.authenticatorAttachment),
    userVerification: optionalText(selection?.userVerification) ?? "preferred",
    hints: Array.from(options.hints ?? [], String),
    credProps: options.extensions?.credProps === true,
  };
}

/** Converts as WebIDL does; throws a TypeError for options WebAuthn refuses. */
export function getOptionsOf(options: PageRequestOptions): GetOptions {
  return {
    rpId: optionalText(options.rpId),
    challenge: encoded(options.challenge),
    timeout: timeoutOf(options.timeout),
    allow: descriptorIds(options.allowCredentials),
    userVerification: optionalText(options.userVerification) ?? "preferred",
    hints: Array.from(options.hints ?? [], String),
  };
}

export function readPageRequest(value: unknown): PageRequest | null {
  return read(pageRequest, value);
}

// An empty `pubKeyCredParams` means ES256 and RS256, WebAuthn §5.4.
export function servable(request: PageRequest): boolean {
  const { hints } = request.options;
  if (hints.length > 0 && hints.every((hint) => hint === "security-key")) {
    return false;
  }
  if (request.mode === "get") return true;
  const { algorithms, attachment } = request.options;
  return (
    attachment !== "cross-platform" &&
    (algorithms.length === 0 || algorithms.includes(es256))
  );
}

export function verifies(options: CreateOptions | GetOptions): boolean {
  return options.userVerification !== "discouraged";
}

export function timeoutMs(options: CreateOptions | GetOptions): number {
  if (options.timeout === null) return defaultTimeoutMs;
  return Math.min(
    Math.max(options.timeout, minimumTimeoutMs),
    maximumTimeoutMs,
  );
}

/** The IDs of a descriptor list's public-key credentials, WebAuthn §5.8.3. */
function descriptorIds(
  descriptors: Iterable<PublicKeyCredentialDescriptor> | undefined,
): Base64URL[] {
  return Array.from(descriptors ?? [])
    .filter(({ type }) => type === "public-key")
    .map(({ id }) => encoded(id));
}

function encoded(source: BufferSource): Base64URL {
  if (ArrayBuffer.isView(source)) {
    return base64urlnopad.encode(
      new Uint8Array(source.buffer, source.byteOffset, source.byteLength),
    );
  }
  if (source instanceof ArrayBuffer) {
    return base64urlnopad.encode(new Uint8Array(source));
  }
  throw new TypeError("A WebAuthn byte field is not a BufferSource.");
}

/** A required DOMString member, converted as WebIDL does. */
function text(value: unknown): string {
  if (value === undefined) {
    throw new TypeError("A required WebAuthn member is missing.");
  }
  return String(value);
}

function optionalText(value: unknown): string | null {
  return value === undefined ? null : String(value);
}

function timeoutOf(value: unknown): number | null {
  if (value === undefined) return null;
  const timeout = Number(value);
  return Number.isFinite(timeout) ? timeout : null;
}
