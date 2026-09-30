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

/** The window the main-world script takes the isolated script's port from. */
export interface OfferWindow {
  addEventListener(
    type: "message",
    listener: (event: MessageEvent) => void,
    capture: boolean,
  ): void;
}

/**
 * Hands `connect` the first port the window offers itself: the isolated script posts it at document_start, before any
 * page script runs, so it is queued ahead of any offer a page makes. Every offer stops in the capture phase, so no
 * offered port reaches a page script, and each offer after the first is closed unused.
 */
export function takePortOffer(
  target: OfferWindow,
  connect: (port: MessagePort) => void,
): void {
  let taken = false;
  target.addEventListener(
    "message",
    (event) => {
      if (event.source !== target || !isPortOffer(event.data)) return;
      event.stopImmediatePropagation();
      const [port, ...others] = event.ports;
      if (taken || !port || others.length > 0) {
        for (const offered of event.ports) offered.close();
        return;
      }
      taken = true;
      connect(port);
    },
    true,
  );
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
