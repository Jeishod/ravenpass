import * as v from "valibot";
import {
  bridgeMessage,
  pageAnswer,
  pageMessage,
  portOfferMessage,
  read,
} from "./page-wire.ts";

// The isolated script posts one MessagePort per top frame at document_start with `portOffer`.
export const portOffer = {
  ravenpass: "webauthn-port",
} as const satisfies v.InferOutput<typeof portOfferMessage>;

export function isPortOffer(data: unknown): boolean {
  return v.is(portOfferMessage, data);
}

export type PageAnswer = v.InferOutput<typeof pageAnswer>;

/** A DOMException a page's request rejects with. */
export type Refusal = Extract<PageAnswer, { readonly kind: "refused" }>;

export const native: PageAnswer = { kind: "native" };

/** Chrome's refusal when the person dismisses its dialog or the request times out. */
export const notAllowed: Refusal = {
  kind: "refused",
  name: "NotAllowedError",
  message:
    "The operation either timed out or was not allowed. See: https://www.w3.org/TR/webauthn-2/#sctn-privacy-considerations-client.",
};

/** Chrome's refusal of a creation that an existing credential excludes. */
export const alreadyRegistered: Refusal = {
  kind: "refused",
  name: "InvalidStateError",
  message:
    "The user attempted to register an authenticator that contains one of the credentials already registered with the relying party.",
};

/** Chrome's refusal of a request while another of the document waits. */
export const alreadyPending: Refusal = {
  kind: "refused",
  name: "NotAllowedError",
  message: "A request is already pending.",
};

export type PageMessage = v.InferOutput<typeof pageMessage>;

export type BridgeMessage = v.InferOutput<typeof bridgeMessage>;

export interface ChannelPort<Outgoing> {
  postMessage(message: Outgoing): void;
  addEventListener(
    type: "message",
    listener: (event: MessageEvent<unknown>) => void,
  ): void;
  start(): void;
}

/** Reads a message from the main-world script, which runs among the page's own scripts. */
export function readPageMessage(data: unknown): PageMessage | null {
  return read(pageMessage, data);
}

export function readBridgeMessage(data: unknown): BridgeMessage | null {
  return read(bridgeMessage, data);
}

export function readPageAnswer(value: unknown): PageAnswer | null {
  return read(pageAnswer, value);
}
