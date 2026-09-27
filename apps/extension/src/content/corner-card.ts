import {
  ask,
  type CardShown,
  isCardShow,
  isFrameMessage,
  isOfferShow,
  menuPage,
} from "../messages.ts";
import { sendIgnoringClosedPort } from "../messaging/send.ts";
import { CornerAnchor, MenuFrame } from "./menu-frame.ts";
import { showsSignInForm } from "./sign-in-forms.ts";

/** How long a new document renders before it tells whether it shows a sign-in form. */
const renderMs = 1000;

interface OpenCard {
  readonly token: string;
  readonly frame: MenuFrame;
  /** Whether the card is a save offer, which waits for an answer. */
  readonly offer: boolean;
}

/** A sign-in card never replaces a shown save offer. */
export class CornerCard {
  private readonly document: Document;
  private open: OpenCard | null = null;
  /** Whether the person closed a sign-in card since a sign-in field last took focus. */
  private dismissed = false;
  /** Counts the offers asked for; an answer to any but the latest is dropped. */
  private offersAsked = 0;
  private loaded: ReturnType<typeof setTimeout> | undefined;

  constructor(document: Document) {
    this.document = document;
  }

  start(): void {
    addEventListener("pagehide", this.onPageHide);
    chrome.runtime.onMessage.addListener(this.onMessage);
    this.loaded = setTimeout(() => void this.showOffer(), renderMs);
  }

  stop(): void {
    removeEventListener("pagehide", this.onPageHide);
    chrome.runtime.onMessage.removeListener(this.onMessage);
    clearTimeout(this.loaded);
    this.close({ tell: true });
  }

  private show(token: string, offer: boolean): void {
    this.close({ tell: true });
    this.open = {
      token,
      offer,
      frame: new MenuFrame(
        new CornerAnchor(this.document),
        `${chrome.runtime.getURL(menuPage)}#${token}`,
        () => this.close({ tell: true }),
      ),
    };
  }

  private async showOffer(): Promise<void> {
    this.offersAsked += 1;
    const asked = this.offersAsked;
    const answer = await sendIgnoringClosedPort(
      ask({ kind: "offer-open", signInForm: showsSignInForm(this.document) }),
    );
    const token = answer?.token;
    if (!token) return;
    if (asked !== this.offersAsked) {
      void sendIgnoringClosedPort(ask({ kind: "menu-close", token }));
      return;
    }
    this.show(token, true);
  }

  private readonly onPageHide = (): void => {
    this.close({ tell: true });
  };

  private readonly onMessage = (
    message: unknown,
    sender: chrome.runtime.MessageSender,
    respond: (answer: CardShown) => void,
  ): undefined => {
    if (sender.id !== chrome.runtime.id) return;
    if (isOfferShow(message)) {
      void this.showOffer();
      return;
    }
    if (isCardShow(message)) {
      const shown = !this.open?.offer && (message.reopen || !this.dismissed);
      if (shown) {
        this.dismissed = false;
        this.show(message.token, false);
      }
      respond({ shown });
      return;
    }
    const open = this.open;
    if (!open || !isFrameMessage(message, open.token)) return;
    switch (message.kind) {
      case "menu-size":
        open.frame.resize(message.height);
        break;
      case "menu-close":
        this.close({ tell: false });
        break;
      case "card-hide":
        if (message.dismissed) this.dismissed = true;
        this.close({ tell: false });
        break;
    }
  };

  /** `tell` is false when the service worker already ended the token. */
  private close({ tell }: { tell: boolean }): void {
    const open = this.open;
    if (!open) return;
    this.open = null;
    open.frame.remove();
    if (tell) {
      void sendIgnoringClosedPort(
        ask({ kind: "menu-close", token: open.token }),
      );
    }
  }
}
