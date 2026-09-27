import type { Answers, PasskeyPageRequest, Request } from "../messages.ts";
import { isPasskeyAnswer } from "../messages.ts";
import { sendIgnoringClosedPort } from "../messaging/send.ts";
import {
  type BridgeMessage,
  type ChannelPort,
  native,
  notAllowed,
  type PageAnswer,
  readPageAnswer,
  readPageMessage,
} from "../passkeys/page-channel.ts";
import { type PageRequest, timeoutMs } from "../passkeys/requests.ts";

/** Asks the service worker one of the requests the bridge relays. */
export type BridgeAsk = <Kind extends PasskeyPageRequest["kind"]>(
  request: Extract<Request, { kind: Kind }>,
) => Promise<Answers[Kind]>;

type RuntimeListener = (
  message: unknown,
  sender: { readonly id?: string },
) => undefined;

/** `chrome.runtime.onMessage` as far as the bridge listens to it. */
export interface RuntimeMessages {
  addListener(listener: RuntimeListener): void;
  removeListener(listener: RuntimeListener): void;
}

export interface PasskeyBridgeDependencies {
  readonly port: ChannelPort<BridgeMessage>;
  readonly ask: BridgeAsk;
  readonly messages: RuntimeMessages;
  /** The extension's own ID, which Chrome reports as the sender of the service worker. */
  readonly extensionId: string;
}

/** Every message from the main-world script is checked: the page can tamper with it.
 * The page receives nothing but the answers to its own requests. */
export class PasskeyBridge {
  private readonly port: ChannelPort<BridgeMessage>;
  private readonly ask: BridgeAsk;
  private readonly messages: RuntimeMessages;
  private readonly extensionId: string;
  /** The waiting requests by id, with the timer of each modal one. */
  private readonly waiting = new Map<
    number,
    ReturnType<typeof setTimeout> | undefined
  >();

  constructor({ port, ask, messages, extensionId }: PasskeyBridgeDependencies) {
    this.port = port;
    this.ask = ask;
    this.messages = messages;
    this.extensionId = extensionId;
  }

  start(): void {
    this.port.addEventListener("message", this.onPageMessage);
    this.port.start();
    this.messages.addListener(this.onRuntimeMessage);
  }

  stop(): void {
    this.messages.removeListener(this.onRuntimeMessage);
    for (const timer of this.waiting.values()) clearTimeout(timer);
    this.waiting.clear();
  }

  private readonly onPageMessage = (event: MessageEvent<unknown>): void => {
    const message = readPageMessage(event.data);
    if (!message) return;
    switch (message.kind) {
      case "request":
        void this.relay(message.id, message.request);
        break;
      case "withdraw":
        this.withdraw(message.id);
        break;
      case "linked":
        void this.linked(message.id);
        break;
    }
  };

  private readonly onRuntimeMessage: RuntimeListener = (message, sender) => {
    if (sender.id !== this.extensionId || !isPasskeyAnswer(message)) return;
    const answer = readPageAnswer(message.answer);
    if (typeof message.id === "number" && answer) {
      this.settle(message.id, answer);
    }
  };

  private async relay(id: number, request: PageRequest): Promise<void> {
    if (this.waiting.has(id)) return;
    const conditional = request.mode === "get" && request.conditional;
    this.waiting.set(
      id,
      conditional
        ? undefined
        : setTimeout(() => this.expire(id), timeoutMs(request.options)),
    );
    const { answer } = await this.ask({
      kind: "passkey-request",
      id,
      request,
    }).catch(() => ({ answer: native }));
    if (answer) this.settle(id, answer);
  }

  /** Refuses a timed-out modal request as Chrome does. */
  private expire(id: number): void {
    if (!this.waiting.delete(id)) return;
    void sendIgnoringClosedPort(this.ask({ kind: "passkey-withdraw", id }));
    this.port.postMessage({ kind: "answer", id, answer: notAllowed });
  }

  private withdraw(id: number): void {
    if (!this.waiting.has(id)) return;
    clearTimeout(this.waiting.get(id));
    this.waiting.delete(id);
    void sendIgnoringClosedPort(this.ask({ kind: "passkey-withdraw", id }));
  }

  private settle(id: number, answer: PageAnswer): void {
    if (!this.waiting.has(id)) return;
    clearTimeout(this.waiting.get(id));
    this.waiting.delete(id);
    this.port.postMessage({ kind: "answer", id, answer });
  }

  private async linked(id: number): Promise<void> {
    const { linked } = await this.ask({ kind: "passkey-linked" }).catch(() => ({
      linked: false,
    }));
    this.port.postMessage({ kind: "linked", id, linked });
  }
}
